<script setup>
import { inject, computed, ref } from 'vue'
import { useApi } from '../composables/useApi'

const props = defineProps({
  entry: Object
})

const emit = defineEmits(['edit', 'refresh'])

const exits = inject('exits')
const trafficStats = inject('trafficStats')
const { apiDelete, apiPost } = useApi()

const rotating = ref(false)

const targetExitName = computed(() => {
  const ex = exits.value.find(e => e.id === props.entry.target_exit_id)
  return ex ? ex.name : '不绑定'
})

// Calculate total traffic for this entry
const entryTraffic = computed(() => {
  if (!trafficStats.value || !trafficStats.value.entry_stats) return 0
  const stats = trafficStats.value.entry_stats[props.entry.id]
  if (!stats) return 0
  return (stats.upload || 0) + (stats.download || 0)
})

async function handleDelete() {
  if (!confirm('确定删除此入口节点?')) return
  try {
    await apiDelete(`/api/v1/entries/${props.entry.id}`)
    emit('refresh')
  } catch (e) {
    alert('删除失败: ' + e.message)
  }
}

async function rotateIP() {
  if (!confirm('确定要更换此入口节点的 IP?')) return
  rotating.value = true
  try {
    await apiPost(`/api/v1/entries/${props.entry.id}/rotate-ip`, {})
    alert('IP 更换指令已下发，请等待生效')
    emit('refresh')
  } catch (e) {
    alert('IP 更换失败: ' + e.message)
  } finally {
    rotating.value = false
  }
}

async function toggleAutoRotate() {
  try {
    const newVal = !props.entry.auto_rotate_ip
    await apiPost(`/api/v1/entries/${props.entry.id}`, {
      ...props.entry,
      auto_rotate_ip: newVal
    })
    emit('refresh')
  } catch (e) {
    alert('操作失败: ' + e.message)
  }
}

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}
</script>

<template>
  <div class="glass p-6 rounded-3xl relative overflow-hidden group hover:shadow-lg transition-shadow">
    <div class="flex justify-between items-start">
      <div class="flex-1">
        <!-- Title & ID -->
        <div class="flex items-center gap-3 mb-2">
          <h3 class="text-xl font-bold">{{ entry.name }}</h3>
          <span class="px-2 py-0.5 rounded-full bg-primary-500/10 text-primary-400 text-xs font-bold">
            ID #{{ entry.id }}
          </span>
          <!-- Cert status -->
          <span v-if="entry.cert_task" class="flex items-center gap-1 text-xs text-blue-400 bg-blue-500/10 px-2 py-0.5 rounded-full animate-pulse">
            ⏳ 证书申请中...
          </span>
          <span v-else-if="entry.cert_body" class="flex items-center gap-1 text-xs text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded-full">
            ✓ 证书已生效
          </span>
        </div>
        
        <!-- Domain & Port -->
        <div class="font-mono text-[var(--text-muted)] text-sm">
          {{ entry.domain }} <span class="mx-2 opacity-50">|</span> {{ entry.port }}
        </div>
        
        <!-- Tags -->
        <div class="flex gap-2 mt-4 flex-wrap">
          <div class="p-2 px-3 bg-[var(--bg-secondary)] rounded-xl border border-[var(--border-color)] text-xs">
            <div class="text-[var(--text-muted)] mb-0.5 uppercase tracking-tighter">默认落地</div>
            {{ targetExitName }}
          </div>
          <div class="p-2 px-3 bg-[var(--bg-secondary)] rounded-xl border border-[var(--border-color)] text-xs">
            <div class="text-[var(--text-muted)] mb-0.5 uppercase tracking-tighter">API 同步</div>
            {{ entry.v2board_url ? '已开启' : '未开启' }}
          </div>
          <div class="p-2 px-3 bg-primary-500/10 rounded-xl border border-primary-500/20 text-xs min-w-[80px]">
            <div class="text-primary-400 mb-0.5 uppercase tracking-tighter font-bold">已用流量</div>
            <span class="font-mono">{{ formatBytes(entryTraffic) }}</span>
          </div>
        </div>

        <!-- IP Controls -->
        <div class="flex gap-3 mt-4 items-center">
          <!-- Auto Rotate Toggle -->
          <label class="flex items-center gap-2 cursor-pointer text-xs">
            <div class="relative">
              <input
                type="checkbox"
                :checked="entry.auto_rotate_ip"
                @change="toggleAutoRotate"
                class="sr-only peer"
              />
              <div class="w-9 h-5 bg-gray-600 rounded-full peer peer-checked:bg-primary-500 transition"></div>
              <div class="absolute left-0.5 top-0.5 w-4 h-4 bg-white rounded-full peer-checked:translate-x-4 transition"></div>
            </div>
            <span class="text-[var(--text-muted)]">自动换IP</span>
          </label>
          
          <!-- Manual Rotate Button -->
          <button
            @click="rotateIP"
            :disabled="rotating"
            class="text-xs px-3 py-1.5 rounded-lg bg-amber-500/10 text-amber-400 border border-amber-500/20 hover:bg-amber-500/20 transition disabled:opacity-50"
          >
            {{ rotating ? '⏳ 更换中...' : '🔄 手动换IP' }}
          </button>
        </div>
      </div>
      
      <!-- Actions -->
      <div class="flex flex-col gap-2 relative z-10">
        <button
          @click="$emit('edit', entry)"
          class="p-3 glass rounded-2xl text-primary-400 hover:scale-110 active:scale-95 transition cursor-pointer"
          title="编辑"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z"></path>
          </svg>
        </button>
        <button
          @click="handleDelete"
          class="p-3 glass rounded-2xl text-rose-500 hover:scale-110 active:scale-95 transition cursor-pointer"
          title="删除"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path>
          </svg>
        </button>
      </div>
    </div>
  </div>
</template>
