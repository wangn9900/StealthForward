<script setup>
import { inject, ref } from 'vue'
import { useApi } from '../composables/useApi'

const settings = inject('settings')
const { apiPost } = useApi()

const saving = ref(false)

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
</script>

<template>
  <div class="max-w-4xl mx-auto space-y-6 animate-fade-in">
    <div class="glass p-8 rounded-3xl">
      <h2 class="text-2xl font-bold mb-6 flex items-center gap-2">
        <svg class="w-6 h-6 text-primary-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"></path>
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
        </svg>
        全局资源配置 (Infrastructure)
      </h2>
      
      <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
        <!-- AWS Section -->
        <div class="space-y-4">
          <h3 class="text-lg font-bold text-[var(--text-secondary)] border-b border-[var(--border-color)] pb-2">
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
          <h3 class="text-lg font-bold text-[var(--text-secondary)] border-b border-[var(--border-color)] pb-2">
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
  </div>
</template>
