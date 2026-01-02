<script setup>
import { ref } from 'vue'

const emit = defineEmits(['login'])

const password = ref('')
const error = ref('')
const loading = ref(false)

async function handleLogin() {
  loading.value = true
  error.value = ''
  
  try {
    const res = await fetch('/api/v1/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: 'admin', password: password.value })
    })
    
    if (!res.ok) {
      throw new Error('密码错误')
    }
    
    const data = await res.json()
    localStorage.setItem('stealth_token', data.token)
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
        <p class="text-[var(--text-muted)]">受限访问控制系统</p>
      </div>
      
      <!-- Login Form -->
      <div class="glass p-8 rounded-3xl space-y-6">
        <div>
          <label class="text-xs font-bold text-[var(--text-muted)] uppercase tracking-widest pl-1">
            管理员密码
          </label>
          <input
            type="password"
            v-model="password"
            @keyup.enter="handleLogin"
            class="w-full mt-2 p-4 rounded-xl text-lg text-center tracking-widest"
            placeholder="••••••••••••"
            autofocus
          />
        </div>
        
        <button
          @click="handleLogin"
          :disabled="loading"
          class="w-full bg-primary-600 hover:bg-primary-500 disabled:opacity-50 text-white p-4 rounded-xl font-bold transition shadow-lg shadow-primary-500/20 active:scale-95"
        >
          {{ loading ? '验证中...' : '解锁控制台' }}
        </button>
        
        <p v-if="error" class="text-center text-rose-500 text-sm animate-pulse">
          {{ error }}
        </p>
      </div>
    </div>
  </div>
</template>
