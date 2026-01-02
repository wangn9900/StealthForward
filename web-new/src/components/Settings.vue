<script setup>
import { inject, ref, onMounted } from 'vue'
import { useApi } from '../composables/useApi'

const settings = inject('settings')
const { apiPost, apiGet } = useApi()

const saving = ref(false)
const keys = ref([])

onMounted(async () => {
  await fetchKeys()
})

async function fetchKeys() {
  try {
    const res = await apiGet('/api/v1/cloud/keys')
    keys.value = res || []
  } catch (e) {
    console.error('获取密钥列表失败', e)
    keys.value = []
  }
}

async function saveSettings() {
  saving.value = true
  try {
    await apiPost('/api/v1/system/config', settings.value)
    alert('配置已保存')
  } catch (e) {
    alert('保存失败: ' + e.message)
  } finally {
    saving.value = false
  }
}

function downloadKey(name) {
  // 直接通过浏览器的下载功能
  const token = localStorage.getItem('stealth_token') // useApi.js 中使用的 key
  window.open(`/api/v1/cloud/keys/${name}?token=${token}`, '_blank')
}

function formatSize(bytes) {
  if (!bytes) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}
</script>

<template>
  <div class="max-w-4xl mx-auto space-y-6 animate-fade-in pb-12">
    <!-- Infrastructure Config -->
    <div class="glass p-8 rounded-3xl" v-if="settings">
      <h2 class="text-2xl font-bold mb-6 flex items-center gap-2 text-[var(--text-primary)]">
        <svg class="w-6 h-6 text-primary-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"></path>
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
        </svg>
        全局资源配置 (Infrastructure)
      </h2>
      
      <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
        <!-- AWS Section -->
        <div class="space-y-4">
          <h3 class="text-lg font-bold text-[var(--text-secondary)] border-b border-[var(--border-color)] pb-2 flex items-center gap-2">
            <span class="w-1.5 h-4 bg-orange-500 rounded-full"></span>
            AWS Credentials
          </h3>
          <label class="block text-sm text-[var(--text-muted)]">
            Access Key ID
            <input
              v-model="settings['aws.access_key_id']"
              class="w-full mt-1"
              placeholder="AKIA..."
            />
          </label>
          <label class="block text-sm text-[var(--text-muted)]">
            Secret Access Key
            <input
              v-model="settings['aws.secret_access_key']"
              type="password"
              class="w-full mt-1"
              placeholder="wJalrX..."
            />
          </label>
          <label class="block text-sm text-[var(--text-muted)]">
            Default Region
            <select v-model="settings['aws.default_region']" class="w-full mt-1">
              <option value="ap-northeast-1">Tokyo (ap-northeast-1)</option>
              <option value="ap-east-1">Hong Kong (ap-east-1)</option>
              <option value="ap-southeast-1">Singapore (ap-southeast-1)</option>
              <option value="us-west-1">California (us-west-1)</option>
            </select>
          </label>
        </div>
        
        <!-- Cloudflare Section -->
        <div class="space-y-4">
          <h3 class="text-lg font-bold text-[var(--text-secondary)] border-b border-[var(--border-color)] pb-2 flex items-center gap-2">
             <span class="w-1.5 h-4 bg-blue-500 rounded-full"></span>
            Cloudflare DNS
          </h3>
          <label class="block text-sm text-[var(--text-muted)]">
            API Token (Edit Zone DNS)
            <input
              v-model="settings['cloudflare.api_token']"
              type="password"
              class="w-full mt-1"
              placeholder="X-Auth-Key..."
            />
          </label>
          <label class="block text-sm text-[var(--text-muted)]">
            Default Zone (e.g. 2233006.xyz)
            <input
              v-model="settings['cloudflare.default_zone']"
              class="w-full mt-1"
              placeholder="example.com"
            />
          </label>
        </div>
      </div>
      
      <div class="mt-8 flex justify-end">
        <button
          @click="saveSettings"
          :disabled="saving"
          class="bg-primary-600 hover:bg-primary-500 disabled:opacity-50 text-white px-8 py-3 rounded-xl font-bold transition shadow-lg shadow-primary-500/20 active:scale-95"
        >
          {{ saving ? '保存中...' : '保存系统配置' }}
        </button>
      </div>
    </div>

    <!-- SSH Keys List -->
    <div class="glass p-8 rounded-3xl mt-6">
      <h2 class="text-xl font-bold mb-6 flex items-center gap-2 text-[var(--text-primary)]">
        <svg class="w-5 h-5 text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
        </svg>
        已保存的 SSH 密钥 (.pem)
      </h2>

      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <div 
          v-for="key in keys" 
          :key="key.name"
          class="p-4 bg-[var(--bg-secondary)] rounded-2xl border border-[var(--border-color)] flex justify-between items-center group hover:border-amber-500/30 transition"
        >
          <div class="overflow-hidden">
            <div class="text-sm font-bold truncate pr-2 text-amber-500">{{ key.name }}</div>
            <div class="text-[10px] text-[var(--text-muted)] mt-1">
              {{ formatSize(key.size) }} | {{ key.updated_at }}
            </div>
          </div>
          <button 
            @click="downloadKey(key.name)"
            class="p-2 bg-amber-500/10 text-amber-400 rounded-lg hover:bg-amber-500 hover:text-white transition shadow-sm"
            title="下载到本地"
          >
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a2 2 0 002 2h12a2 2 0 002-2v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
          </button>
        </div>
        <div v-if="keys.length === 0" class="md:col-span-2 py-8 text-center text-[var(--text-muted)] italic text-sm">
          暂无本地保存的密钥文件。新建实例时会自动生成并保存在此。
        </div>
      </div>
      <p class="mt-6 text-xs text-[var(--text-muted)] leading-relaxed">
        * 这些密钥文件保存在主机的 <code class="bg-black/20 p-0.5 rounded px-1">store/keys/</code> 目录下。为了安全，下载后请妥善保管。
      </p>
    </div>
  </div>
</template>
