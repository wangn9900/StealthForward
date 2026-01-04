<script setup>
defineProps({
  activeTab: String,
  isDark: Boolean
})

const emit = defineEmits(['update:activeTab', 'toggle-theme', 'refresh', 'logout'])

const tabs = [
  { key: 'dashboard', label: '概览' },
  { key: 'mappings', label: '配置' },
  { key: 'settings', label: '系统' }
]

const appVersion = import.meta.env.VITE_APP_VERSION || 'Dev'

import { ref, onMounted, computed, watch } from 'vue'
import { useLicense } from '../composables/useLicense'

const { licenseInfo, fetchLicenseInfo } = useLicense()
const expiresAt = ref('')
const activKey = ref('')
const loadingActiv = ref(false)

const renewVisible = computed(() => {
  if (!expiresAt.value) return false
  const exp = new Date(expiresAt.value)
  const now = new Date()
  const diffTime = exp - now
  const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24))
  return diffDays <= 7 // 这里设置剩下7天内显示续费
})

onMounted(async () => {
  expiresAt.value = localStorage.getItem('stealth_expires') || ''
  // 强制与后端同步授权状态
  await fetchLicenseInfo()
})

// 监听后端授权状态变化，自动同步 UI
watch(licenseInfo, (newVal) => {
  if (newVal && (newVal.expires_at === '-' || !newVal.expires_at)) {
    // 授权无效或过期
    expiresAt.value = ''
    localStorage.removeItem('stealth_expires')
    localStorage.removeItem('stealth_level')
  } else if (newVal && newVal.expires_at) {
    // 授权有效，更新状态
    expiresAt.value = newVal.expires_at
    localStorage.setItem('stealth_expires', newVal.expires_at)
  }
})

async function activate() {
  if (!activKey.value) return
  loadingActiv.value = true
  try {
    const res = await fetch('/api/v1/system/activate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': localStorage.getItem('stealth_token') },
      body: JSON.stringify({ license_key: activKey.value })
    })
    const data = await res.json()
    if (res.ok) {
      localStorage.setItem('stealth_expires', data.expires_at)
      localStorage.setItem('stealth_level', data.level)
      expiresAt.value = data.expires_at
      alert('激活成功！')
    } else {
      alert(data.error || '激活失败')
    }
  } catch(e) {
    alert('请求失败')
  } finally {
    loadingActiv.value = false
  }
}

function resetLicense() {
  if (confirm('确定要重置当前授权吗？\n\n重置后您可以用新的 License Key 重新激活，以升级 Pro 版或切换账号。')) {
    localStorage.removeItem('stealth_token')
    localStorage.removeItem('stealth_expires')
    localStorage.removeItem('stealth_level')
    expiresAt.value = ''
    setTimeout(() => location.reload(), 500)
  }
}
</script>

<template>
  <div class="flex flex-col md:flex-row justify-between items-start md:items-center mb-8 gap-4">
    <!-- Logo & Title -->
    <div>
      <h1 class="text-4xl font-extrabold tracking-tighter text-[var(--text-primary)] flex items-baseline gap-3">
        StealthForward
        <span class="text-lg font-mono text-[var(--text-muted)] opacity-60">
          {{ appVersion }}
        </span>
      </h1>
      <p class="text-[var(--text-muted)] text-sm mt-1">First-Principles 架构 | 隐形中转分流中心</p>
    </div>

    <!-- Controls -->
    <div class="flex gap-3 items-center">
      
      <!-- Validity Display or Activation Input -->
      <template v-if="expiresAt">
        <div @click="resetLicense" class="glass px-4 py-2 rounded-xl text-sm font-mono text-emerald-400 border border-emerald-500/20 flex items-center gap-2 animate-fade-in cursor-pointer hover:bg-emerald-500/10 transition" title="点击重置或升级授权">
          <span class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></span>
          有效期至 {{ expiresAt }}
        </div>
        <a 
          v-if="renewVisible"
          href="https://t.me/Milkyone_y" 
          target="_blank"
          class="glass px-4 py-2 rounded-xl text-sm font-bold text-amber-500 hover:text-amber-400 border border-amber-500/30 flex items-center gap-1 transition hover:bg-amber-500/10 animate-fade-in"
        >
          ⏱️ 立即续费
        </a>
      </template>
      
      <div v-else class="flex gap-2 animate-fade-in">
        <input 
          v-model="activKey" 
          type="text" 
          placeholder="输入 License/Smart Key 激活" 
          class="glass px-3 py-2 rounded-xl text-sm text-white placeholder-gray-400 border border-white/10 focus:border-primary-500 outline-none w-96"
        />
        <button 
          @click="activate" 
          :disabled="loadingActiv"
          class="bg-primary-600 px-4 py-2 rounded-xl text-sm font-bold text-white hover:bg-primary-500 transition disabled:opacity-50"
        >
          {{ loadingActiv ? '...' : '激活' }}
        </button>
        
        <!-- Buy License Placeholder -->
        <a 
          href="https://t.me/Milkyone_y" 
          target="_blank"
          class="glass px-4 py-2 rounded-xl text-sm font-bold text-amber-500 hover:text-amber-400 border border-amber-500/30 flex items-center gap-1 transition hover:bg-amber-500/10"
        >
          🛒 购买授权
        </a>
      </div>

      <!-- Tab Switcher -->
      <div class="glass flex p-1 rounded-2xl items-center">
        <div
          v-for="tab in tabs"
          :key="tab.key"
          @click="$emit('update:activeTab', tab.key)"
          :class="['tab-btn', activeTab === tab.key ? 'active' : '']"
        >
          {{ tab.label }}
        </div>
      </div>

      <!-- Theme Toggle -->
      <button
        @click="$emit('toggle-theme')"
        class="p-3 glass rounded-2xl hover:bg-[var(--bg-secondary)] transition"
        :title="isDark ? '切换到浅色模式' : '切换到深色模式'"
      >
        <!-- Sun icon (shown in dark mode) -->
        <svg v-if="isDark" class="w-5 h-5 text-amber-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
        </svg>
        <!-- Moon icon (shown in light mode) -->
        <svg v-else class="w-5 h-5 text-primary-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
        </svg>
      </button>

      <!-- Refresh -->
      <button
        @click="$emit('refresh')"
        class="p-3 px-5 glass rounded-2xl hover:bg-[var(--bg-secondary)] transition flex items-center gap-2"
      >
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
        刷新
      </button>

      <!-- Logout -->
      <button
        @click="$emit('logout')"
        class="p-3 px-5 glass rounded-2xl hover:bg-rose-500/10 transition flex items-center gap-2 text-rose-500"
      >
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
        </svg>
        退出
      </button>
    </div>
  </div>
</template>
