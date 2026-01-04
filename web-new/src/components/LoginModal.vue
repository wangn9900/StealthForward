<script setup>
import { ref, computed } from 'vue'

const emit = defineEmits(['login'])

// 登录模式切换
const loginMode = ref('license') // 'license' 或 'admin'

// 表单字段
const licenseKey = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

// 切换登录模式
function toggleMode() {
  loginMode.value = loginMode.value === 'license' ? 'admin' : 'license'
  error.value = ''
}

// 处理登录
async function handleLogin() {
  loading.value = true
  error.value = ''
  
  try {
    let body = {}
    
    if (loginMode.value === 'admin') {
      // 管理员密码登录
      body = { username: 'admin', password: password.value }
    } else {
      // License Key 登录
      body = { license_key: licenseKey.value }
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
        <p class="text-[var(--text-muted)]">超隐蔽中转控制系统</p>
      </div>
      
      <!-- Login Form -->
      <div class="glass p-8 rounded-3xl space-y-6">
        <!-- 模式切换标签 -->
        <div class="flex rounded-xl bg-black/20 p-1">
          <button 
            @click="loginMode = 'license'"
            :class="[
              'flex-1 py-2 rounded-lg text-sm font-bold transition',
              loginMode === 'license' 
                ? 'bg-primary-600 text-white' 
                : 'text-[var(--text-muted)] hover:text-white'
            ]"
          >
            🔑 授权Key
          </button>
          <button 
            @click="loginMode = 'admin'"
            :class="[
              'flex-1 py-2 rounded-lg text-sm font-bold transition',
              loginMode === 'admin' 
                ? 'bg-amber-600 text-white' 
                : 'text-[var(--text-muted)] hover:text-white'
            ]"
          >
            👑 管理员
          </button>
        </div>

        <!-- License Key 输入 -->
        <div v-if="loginMode === 'license'">
          <label class="text-xs font-bold text-[var(--text-muted)] uppercase tracking-widest pl-1">
            授权Key
          </label>
          <input
            type="text"
            v-model="licenseKey"
            @keyup.enter="handleLogin"
            class="w-full mt-2 p-4 rounded-xl text-lg tracking-wide font-mono"
            placeholder="SF-B-XXXX-XXXX-XXXX-XXXX"
            autofocus
          />
          <p class="text-xs text-[var(--text-muted)] mt-2 pl-1">
            输入您购买的授权Key进行验证
          </p>
        </div>
        
        <!-- 管理员密码输入 -->
        <div v-else>
          <label class="text-xs font-bold text-amber-400 uppercase tracking-widest pl-1">
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
          <p class="text-xs text-amber-500/60 mt-2 pl-1">
            仅限系统管理员使用
          </p>
        </div>
        
        <button
          @click="handleLogin"
          :disabled="loading"
          :class="[
            'w-full p-4 rounded-xl font-bold transition shadow-lg active:scale-95 disabled:opacity-50',
            loginMode === 'license'
              ? 'bg-primary-600 hover:bg-primary-500 shadow-primary-500/20'
              : 'bg-amber-600 hover:bg-amber-500 shadow-amber-500/20'
          ]"
        >
          {{ loading ? '验证中...' : (loginMode === 'license' ? '验证授权' : '管理员登录') }}
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
