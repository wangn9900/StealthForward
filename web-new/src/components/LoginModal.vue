<script setup>
import { ref, computed } from 'vue'

const emit = defineEmits(['login'])

// 表单字段
const inputKey = ref('')
const error = ref('')
const loading = ref(false)

// 处理登录
async function handleLogin() {
  loading.value = true
  error.value = ''
  
  try {
    let body = {}
    
    const val = inputKey.value.trim()
    
    // 智能识别：以 SF- 开头视为 License Key，否则视为管理员密码
    if (val.startsWith('SF-')) {
      body = { license_key: val }
    } else {
      body = { username: 'admin', password: val }
    }

    const res = await fetch('/api/v1/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    })
    
    const data = await res.json()
    
    if (!res.ok) {
      throw new Error(data.error || '验证失败')
    }
    
    // 保存登录信息
    localStorage.setItem('stealth_token', data.token)
    localStorage.setItem('stealth_role', data.role || 'user')
    localStorage.setItem('stealth_level', data.level || 'basic')
    if (data.expires_at) {
      localStorage.setItem('stealth_expires', data.expires_at)
    }
    
    emit('login')
  } catch (e) {
    error.value = e.message || '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="fixed inset-0 bg-[var(--bg-primary)] z-50 flex items-center justify-center p-4 animate-fade-in">
    <div class="w-full max-w-md space-y-8">
        <!-- Logo -->
        <div class="text-center">
          <h1 class="text-5xl font-extrabold tracking-tighter gradient-text mb-2">StealthForward</h1>
          <p class="text-[var(--text-muted)]">商业化智能中转控制系统</p>
        </div>
        
        <!-- Login Form -->
        <div class="glass p-8 rounded-3xl space-y-6">
          
          <!-- Unified Input -->
          <div>
            <label class="text-xs font-bold text-[var(--text-muted)] uppercase tracking-widest pl-1">
              授权验证
            </label>
            <input
              type="text"
              v-model="inputKey"
              @keyup.enter="handleLogin"
              class="w-full mt-2 p-4 rounded-xl text-lg tracking-wide bg-black/20 border border-white/10 focus:border-primary-500 focus:outline-none transition-colors text-white"
              placeholder="粘贴 License Key 或输入管理员密码"
              autofocus
            />
            <p class="text-xs text-[var(--text-muted)] mt-2 pl-1">
              请直接输入 License Key 进行激活登录
            </p>
          </div>
        
        <button
          @click="handleLogin"
          :disabled="loading"
          :class="[
            'w-full p-4 rounded-xl font-bold transition shadow-lg active:scale-95 disabled:opacity-50',
            'w-full p-4 rounded-xl font-bold transition shadow-lg active:scale-95 disabled:opacity-50 bg-primary-600 hover:bg-primary-500 shadow-primary-500/20'
          ]"
        >
          {{ loading ? '正在验证...' : '立即激活 / 登录' }}
        </button>
        
        <p v-if="error" class="text-center text-rose-500 text-sm animate-pulse">
          {{ error }}
        </p>
      </div>
      
      <!-- Footer -->
      <p class="text-center text-xs text-[var(--text-muted)]/50">
        StealthForward v3.6.0 · Commercial License System
      </p>
    </div>
  </div>
</template>
