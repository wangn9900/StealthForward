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

const nodeStats = computed(() => {
  if (!trafficStats.value || !trafficStats.value.node_stats) return null
  return trafficStats.value.node_stats[props.entry.id] || null
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
        alert('自动识别失败: ' + e.message + '。请检查 IP 是否属于已配置 Ox 的 AWS 账号。')
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

function formatSpeed(bytesPerSec) {
  if (!bytesPerSec) return '0 B/s'
  const k = 1024
  const sizes = ['B/s', 'KB/s', 'MB/s', 'GB/s']
  const i = Math.floor(Math.log(bytesPerSec) / Math.log(k))
  return parseFloat((bytesPerSec / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function formatUptime(seconds) {
  if (!seconds) return 'Offline'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (h > 24) {
    return `${Math.floor(h/24)}d ${h%24}h`
  }
  return `${h}h ${m}m`
}
</script>

<template>
  <div class="glass p-6 rounded-3xl relative overflow-hidden group hover:shadow-lg transition-all border border-white/5">
    <div class="flex flex-col lg:flex-row justify-between items-stretch gap-8">
      <!-- Left: Basic Info -->
      <div class="flex-1 min-w-[300px]">
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
            </div>
          </div>
        </div>
        
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
        </div>

        <!-- IP Controls -->
        <div class="flex gap-4 mt-6 items-center bg-white/5 p-3 rounded-2xl w-fit">
          <label class="flex items-center gap-2 cursor-pointer group/toggle">
            <div class="relative">
              <input type="checkbox" :checked="entry.auto_rotate_ip" @change="toggleAutoRotate" class="sr-only peer" />
              <div class="w-8 h-4 bg-gray-600 rounded-full peer peer-checked:bg-primary-500 transition shadow-inner"></div>
              <div class="absolute left-0.5 top-0.5 w-3 h-3 bg-white rounded-full peer-checked:translate-x-4 transition shadow-md"></div>
            </div>
            <span class="text-[var(--text-muted)] text-[11px] font-medium group-hover/toggle:text-white transition">自动换IP</span>
          </label>
          <div class="w-px h-4 bg-white/10"></div>
          <button @click="rotateIP" :disabled="rotating" class="text-[11px] px-3 py-1.5 bg-amber-500 text-white hover:bg-amber-400 rounded-xl font-bold transition flex items-center gap-2 shadow-lg shadow-amber-500/20 active:scale-95 disabled:opacity-50">
            <svg v-if="!rotating" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            <span v-else class="animate-spin text-[10px]">⏳</span>
            {{ rotating ? '操作中...' : '手动换IP' }}
          </button>
        </div>
      </div>

      <!-- Middle: Realtime Stats (The Probe) -->
      <div class="flex-1 flex flex-col justify-center min-w-[320px] bg-white/5 p-4 rounded-3xl border border-white/5">
        <template v-if="nodeStats">
          <div class="grid grid-cols-2 gap-4">
            <!-- Left: Bars -->
            <div class="space-y-3">
              <div class="space-y-1">
                <div class="flex justify-between text-[10px] text-[var(--text-muted)] uppercase font-bold tracking-tight">
                  <span>CPU</span>
                  <span :class="nodeStats.cpu > 80 ? 'text-rose-400' : 'text-emerald-400'">{{ nodeStats.cpu.toFixed(0) }}%</span>
                </div>
                <div class="h-1.5 w-full bg-white/5 rounded-full overflow-hidden">
                  <div class="h-full bg-emerald-500 transition-all duration-500" :style="{ width: nodeStats.cpu + '%' }"></div>
                </div>
              </div>
              <div class="space-y-1">
                <div class="flex justify-between text-[10px] text-[var(--text-muted)] uppercase font-bold tracking-tight">
                  <span>内存</span>
                  <span>{{ nodeStats.mem.toFixed(0) }}%</span>
                </div>
                <div class="h-1.5 w-full bg-white/5 rounded-full overflow-hidden">
                  <div class="h-full bg-blue-500 transition-all duration-500" :style="{ width: nodeStats.mem + '%' }"></div>
                </div>
              </div>
              <div class="space-y-1">
                <div class="flex justify-between text-[10px] text-[var(--text-muted)] uppercase font-bold tracking-tight">
                  <span>硬盘</span>
                  <span>{{ nodeStats.disk.toFixed(0) }}%</span>
                </div>
                <div class="h-1.5 w-full bg-white/5 rounded-full overflow-hidden">
                  <div class="h-full bg-amber-500 transition-all duration-500" :style="{ width: nodeStats.disk + '%' }"></div>
                </div>
              </div>
            </div>
            
            <!-- Right: Numbers -->
            <div class="grid grid-cols-1 gap-2 text-[10px]">
              <div class="flex justify-between items-center bg-black/20 p-2 rounded-xl border border-white/5">
                <span class="text-[var(--text-muted)] uppercase">网速 ↑↓</span>
                <span class="font-mono text-emerald-400 font-bold">{{ formatSpeed(nodeStats.net_out) }} / {{ formatSpeed(nodeStats.net_in) }}</span>
              </div>
              <div class="flex justify-between items-center bg-black/20 p-2 rounded-xl border border-white/5">
                <span class="text-[var(--text-muted)] uppercase">负载</span>
                <span class="font-mono text-white">{{ nodeStats.load1.toFixed(1) }} | {{ nodeStats.load5.toFixed(1) }}</span>
              </div>
              <div class="flex justify-between items-center bg-black/20 p-2 rounded-xl border border-white/5">
                <span class="text-[var(--text-muted)] uppercase">流量计</span>
                <span class="font-mono text-primary-400 font-bold">{{ formatBytes(entryTraffic) }}</span>
              </div>
              <div class="flex justify-between items-center bg-black/20 p-2 rounded-xl border border-white/5">
                <span class="text-[var(--text-muted)] uppercase">已在线</span>
                <span class="font-mono text-white">{{ formatUptime(nodeStats.uptime) }}</span>
              </div>
            </div>
          </div>
        </template>
        <div v-else class="h-full flex flex-col items-center justify-center text-[var(--text-muted)] opacity-50 space-y-2">
          <svg class="w-8 h-8 animate-pulse" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
          </svg>
          <span class="text-[10px] font-bold tracking-widest uppercase">等待 Agent 上报状态...</span>
        </div>
      </div>

      <!-- Right: Actions -->
      <div class="flex flex-row lg:flex-col gap-2 relative z-10">
        <button
          @click="$emit('edit', entry)"
          class="flex-1 md:flex-none p-3 glass rounded-2xl text-primary-400 hover:bg-primary-500 hover:text-white transition-all active:scale-90 flex items-center justify-center"
          title="编辑"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z"></path>
          </svg>
        </button>
        <button
          @click="handleDelete"
          class="flex-1 md:flex-none p-3 glass rounded-2xl text-rose-500 hover:bg-rose-600 hover:text-white transition-all active:scale-90 flex items-center justify-center"
          title="删除"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path>
          </svg>
        </button>
      </div>
    </div>

    <!-- Decorative glow -->
    <div class="absolute -right-10 -bottom-10 w-32 h-32 bg-primary-500/5 rounded-full blur-3xl pointer-events-none group-hover:bg-primary-500/10 transition-colors"></div>
  </div>
</template>
