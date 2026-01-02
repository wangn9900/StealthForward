<script setup>
import { inject, computed, ref } from 'vue'
import { useApi } from '../composables/useApi'

const props = defineProps({
  entry: Object
})

const emit = defineEmits(['edit', 'refresh'])

const exits = inject('exits')
const trafficStats = inject('trafficStats')
const settings = inject('settings')
const { apiDelete, apiPost, apiGet } = useApi()

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
  // 如果没绑定云平台，尝试自动识别
  if (!props.entry.cloud_provider || props.entry.cloud_provider === 'none') {
    if (confirm('此入口尚未绑定云实例，是否尝试根据当前 IP 自动识别并绑定？')) {
      rotating.value = true
      try {
        const res = await apiGet(`/api/v1/cloud/auto-detect?ip=${props.entry.ip}`)
        // 自动保存绑定关系
        await apiPost('/api/v1/entries', {
          ...props.entry,
          cloud_provider: res.provider,
          cloud_region: res.region,
          cloud_instance_id: res.instance_id,
          cloud_record_name: res.record_name || (props.entry.domain.split('.')[0])
        })
        alert(`识别成功: ${res.provider} (${res.region})。已自动绑定并保存。`)
        emit('refresh')
        // 绑定成功后继续执行换 IP
      } catch (e) {
        alert('自动识别失败: ' + e.message + '。请检查 IP 是否属于已配置的 AWS 账号。')
        rotating.value = false
        return
      }
    } else {
      return
    }
  }

  if (!confirm('确定要更换此入口节点的 IP? 这将申请新的弹性 IP 并更新 DNS。')) return
  rotating.value = true
  try {
    const res = await apiPost(`/api/v1/node/${props.entry.id}/rotate-ip`, {
      region: props.entry.cloud_region,
      instance_id: props.entry.cloud_instance_id,
      zone_name: settings?.value?.['cloudflare.default_zone'] || '',
      record_name: props.entry.cloud_record_name
    })
    alert('IP 更换成功！新 IP: ' + res.new_ip)
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
    await apiPost('/api/v1/entries/${props.entry.id}', {
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
  <div class="glass p-6 rounded-3xl relative overflow-hidden group hover:shadow-lg transition-shadow border border-white/5">
    <div class="flex flex-col md:flex-row justify-between items-start gap-6">
      <div class="flex-1">
        <!-- Title & ID -->
        <div class="flex items-center gap-3 mb-2">
          <div class="w-10 h-10 bg-primary-500/10 rounded-2xl flex items-center justify-center text-primary-400">
            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
            </svg>
          </div>
          <div>
            <div class="flex items-center gap-2">
              <h3 class="text-lg font-bold text-white">{{ entry.name }}</h3>
              <span class="px-2 py-0.5 rounded-lg bg-white/5 text-[var(--text-muted)] text-[10px] font-mono">
                #{{ entry.id }}
              </span>
            </div>
            <div class="font-mono text-[var(--text-muted)] text-xs mt-0.5 flex items-center gap-2">
              <span class="text-primary-400/80">{{ entry.ip }}</span>
              <span class="opacity-20">|</span>
              <span>{{ entry.domain }}</span>
              <span class="opacity-20">|</span>
              <span>{{ entry.port }}</span>
            </div>
          </div>
        </div>
        
        <!-- Tags & Stats -->
        <div class="flex gap-2 mt-5 flex-wrap">
          <div class="p-2 px-3 bg-black/20 rounded-xl border border-white/5 text-[10px]">
            <div class="text-[var(--text-muted)] mb-0.5 uppercase tracking-tighter opacity-70">默认落地</div>
            <div class="text-white font-medium">{{ targetExitName }}</div>
          </div>
          <div class="p-2 px-3 bg-black/20 rounded-xl border border-white/5 text-[10px]">
            <div class="text-[var(--text-muted)] mb-0.5 uppercase tracking-tighter opacity-70">API 同步</div>
            <div :class="entry.v2board_url ? 'text-emerald-400' : 'text-rose-400'">
              {{ entry.v2board_url ? '已开启' : '未开启' }}
            </div>
          </div>
          <div class="p-2 px-3 bg-primary-500/10 rounded-xl border border-primary-500/20 text-[10px] min-w-[90px]">
            <div class="text-primary-400 mb-0.5 uppercase tracking-tighter font-bold">已用流量</div>
            <span class="font-mono text-white text-xs font-bold">{{ formatBytes(entryTraffic) }}</span>
          </div>
        </div>

        <!-- IP Controls -->
        <div class="flex gap-4 mt-6 items-center bg-white/5 p-3 rounded-2xl w-fit">
          <!-- Auto Rotate Toggle -->
          <label class="flex items-center gap-2 cursor-pointer group/toggle">
            <div class="relative">
              <input
                type="checkbox"
                :checked="entry.auto_rotate_ip"
                @change="toggleAutoRotate"
                class="sr-only peer"
              />
              <div class="w-8 h-4 bg-gray-600 rounded-full peer peer-checked:bg-primary-500 transition shadow-inner"></div>
              <div class="absolute left-0.5 top-0.5 w-3 h-3 bg-white rounded-full peer-checked:translate-x-4 transition shadow-md"></div>
            </div>
            <span class="text-[var(--text-muted)] text-[11px] font-medium group-hover/toggle:text-white transition">自动换IP</span>
          </label>
          
          <div class="w-px h-4 bg-white/10 mx-1"></div>

          <!-- Manual Rotate Button -->
          <button
            @click="rotateIP"
            :disabled="rotating"
            class="text-[11px] px-3 py-1.5 rounded-xl font-bold transition flex items-center gap-2 overflow-hidden relative"
            :class="rotating ? 'bg-amber-500/20 text-amber-500 cursor-not-allowed' : 'bg-amber-500 text-white hover:bg-amber-400 shadow-lg shadow-amber-500/20 active:scale-95'"
          >
            <svg v-if="!rotating" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            <span v-else class="animate-spin text-lg">⏳</span>
            {{ rotating ? '正在操作中...' : '手动换IP' }}
          </button>
        </div>
      </div>
      
      <!-- Actions -->
      <div class="flex flex-row md:flex-col gap-2 relative z-10 w-full md:w-auto">
        <button
          @click="$emit('edit', entry)"
          class="flex-1 md:flex-none p-3 glass rounded-2xl text-primary-400 hover:bg-primary-500 hover:text-white transition-all active:scale-90 flex items-center justify-center gap-2 md:block"
          title="编辑"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z"></path>
          </svg>
          <span class="md:hidden text-sm font-bold">编辑设置</span>
        </button>
        <button
          @click="handleDelete"
          class="flex-1 md:flex-none p-3 glass rounded-2xl text-rose-500 hover:bg-rose-600 hover:text-white transition-all active:scale-90 flex items-center justify-center gap-2 md:block"
          title="删除"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path>
          </svg>
          <span class="md:hidden text-sm font-bold">删除节点</span>
        </button>
      </div>
    </div>

    <!-- Decorative glow -->
    <div class="absolute -right-10 -bottom-10 w-32 h-32 bg-primary-500/5 rounded-full blur-3xl pointer-events-none group-hover:bg-primary-500/10 transition-colors"></div>
  </div>
</template>
