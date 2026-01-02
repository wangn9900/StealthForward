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
  if (!props.entry.cloud_provider || props.entry.cloud_provider === 'none') {
    if (confirm('此入口尚未绑定云实例，是否尝试根据当前 IP 自动识别并绑定？')) {
      rotating.value = true
      try {
        const res = await apiGet(`/api/v1/cloud/auto-detect?ip=${props.entry.ip}`)
        await apiPost('/api/v1/entries', {
          ...props.entry,
          cloud_provider: res.provider,
          cloud_region: res.region,
          cloud_instance_id: res.instance_id,
          cloud_record_name: res.record_name || (props.entry.domain.split('.')[0])
        })
        alert(`识别成功: ${res.provider} (${res.region})。已自动绑定并保存。`)
        emit('refresh')
      } catch (e) {
        alert('自动识别失败: ' + e.message)
        rotating.value = false
        return
      }
    } else {
      return
    }
  }

  if (!confirm('确定要更换此入口节点的 IP?')) return
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

function copyUpdateCommand() {
  // 智能生成针对该节点的更新命令
  const controllerUrl = window.location.origin
  const nodeId = props.entry.id
  const token = settings?.value?.['admin_token'] || ''
  const cmd = `curl -fsSL ${controllerUrl}/static/install.sh | bash -s -- --node-id ${nodeId} --controller ${controllerUrl} --token ${token}`
  
  navigator.clipboard.writeText(cmd).then(() => {
    alert('更新脚本已复制！请在服务器 SSH 中粘贴运行。')
  }).catch(err => {
    console.error('Could not copy text: ', err)
    // 兼容性兜底
    const el = document.createElement('textarea')
    el.value = cmd
    document.body.appendChild(el)
    el.select()
    document.execCommand('copy')
    document.body.removeChild(el)
    alert('更新脚本已复制 (兼容模式)！')
  })
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
  if (!seconds) return 'OFFLINE'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (h > 24) return `${Math.floor(h/24)}d ${h%24}h`
  return `${h}h ${m}m`
}
</script>

<template>
  <div class="entry-card group">
    <!-- Main Content Grid -->
    <div class="card-grid">
      
      <!-- Side A: Node Identity & Control -->
      <div class="side-identity">
        <div class="header-section">
          <div class="icon-box">
            <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
            </svg>
          </div>
          <div class="title-area">
            <div class="flex items-center gap-2">
              <h3 class="node-title">{{ entry.name }}</h3>
              <span class="node-id-badge">#{{ entry.id }}</span>
            </div>
            <p class="node-address">{{ entry.ip }} <span class="sep">/</span> <span class="domain">{{ entry.domain }}</span></p>
          </div>
        </div>

        <div class="badge-row">
          <div class="info-pill">
            <span class="label">默认落地</span>
            <span class="val">{{ targetExitName }}</span>
          </div>
          <div class="info-pill" :class="{ 'active': entry.v2board_url }">
            <span class="label">V2BOARD SYNC</span>
            <span class="val">{{ entry.v2board_url ? 'ON' : 'OFF' }}</span>
          </div>
        </div>

        <div class="control-footer">
          <div class="rotate-toggle" @click="toggleAutoRotate">
            <div class="switch" :class="{ 'on': entry.auto_rotate_ip }">
              <div class="thumb"></div>
            </div>
            <span>自动换IP</span>
          </div>
          
          <div class="flex gap-2">
            <button @click="rotateIP" :disabled="rotating" class="btn-action amber">
              <span v-if="rotating" class="animate-spin text-xs">⌛</span>
              <svg v-else class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" /></svg>
              <span>{{ rotating ? '操作中' : '手动换IP' }}</span>
            </button>
            <button @click="copyUpdateCommand" class="btn-icon-bg" title="复制一键更新指令">
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" /></svg>
            </button>
          </div>
        </div>
      </div>

      <!-- Side B: The Pulse (Realtime Stats) -->
      <div class="side-stats">
        <template v-if="nodeStats">
          <div class="stats-container">
            <!-- Left: High-Level Gauges -->
            <div class="gauge-stack">
              <div class="gauge-item">
                <div class="gauge-info">
                  <span>CPU</span>
                  <span class="val" :class="{ 'critical': nodeStats.cpu > 80 }">{{ nodeStats.cpu.toFixed(0) }}%</span>
                </div>
                <div class="progress-track"><div class="progress-bar cpu" :style="{ width: nodeStats.cpu + '%' }"></div></div>
              </div>
              <div class="gauge-item">
                <div class="gauge-info">
                  <span>MEM</span>
                  <span class="val">{{ nodeStats.mem.toFixed(0) }}%</span>
                </div>
                <div class="progress-track"><div class="progress-bar mem" :style="{ width: nodeStats.mem + '%' }"></div></div>
              </div>
              <div class="gauge-item">
                <div class="gauge-info">
                  <span>DISK</span>
                  <span class="val">{{ nodeStats.disk.toFixed(0) }}%</span>
                </div>
                <div class="progress-track"><div class="progress-bar disk" :style="{ width: nodeStats.disk + '%' }"></div></div>
              </div>
            </div>

            <!-- Right: Performance Metrics Grid -->
            <div class="metrics-grid">
              <div class="metric-card">
                <span class="m-label">NETWORK THROUGHPUT</span>
                <div class="m-val-pair">
                  <span class="up">↑ {{ formatSpeed(nodeStats.net_out) }}</span>
                  <span class="down">↓ {{ formatSpeed(nodeStats.net_in) }}</span>
                </div>
              </div>
              <div class="metric-mini-grid">
                <div class="m-box">
                  <span class="sl">LOAD</span>
                  <span class="sv">{{ nodeStats.load1.toFixed(1) }}</span>
                </div>
                <div class="m-box">
                  <span class="sl">TRAFFIC</span>
                  <span class="sv">{{ formatBytes(entryTraffic) }}</span>
                </div>
                <div class="m-box full">
                  <span class="sl">UPTIME</span>
                  <span class="sv uptime">{{ formatUptime(nodeStats.uptime) }}</span>
                </div>
              </div>
            </div>
          </div>
        </template>
        <div v-else class="empty-stats">
          <div class="pulse-loader"></div>
          <p>AWATING AGENT SIGNAL...</p>
        </div>
      </div>

      <!-- Actions: Floating Utility -->
      <div class="side-actions">
        <button @click="$emit('edit', entry)" class="action-btn edit"><svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z"></path></svg></button>
        <button @click="handleDelete" class="action-btn delete"><svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path></svg></button>
      </div>

    </div>
  </div>
</template>

<style scoped>
.entry-card {
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 2rem;
  padding: 1.5rem;
  margin-bottom: 1.5rem;
  transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  overflow: hidden;
}

.entry-card:hover {
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(var(--primary-rgb, 99, 102, 241), 0.3);
  box-shadow: 0 30px 60px -15px rgba(0, 0, 0, 0.7);
  transform: translateY(-4px);
}

.card-grid {
  display: flex;
  flex-direction: row;
  gap: 2rem;
  align-items: stretch;
}

/* SIDE A: Identity */
.side-identity {
  flex: 0 0 320px;
  display: flex;
  flex-direction: column;
}

.header-section {
  display: flex;
  gap: 1.25rem;
  align-items: flex-start;
  margin-bottom: 1.5rem;
}

.icon-box {
  width: 4rem;
  height: 4rem;
  background: linear-gradient(135deg, rgba(var(--primary-rgb, 99, 102, 241), 0.25), rgba(var(--primary-rgb, 99, 102, 241), 0.05));
  border-radius: 1.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #818cf8;
  border: 1px solid rgba(129, 140, 248, 0.2);
}

.node-title {
  font-size: 1.35rem;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.03em;
  line-height: 1.2;
}

.node-id-badge {
  background: rgba(255,255,255,0.08);
  padding: 3px 10px;
  border-radius: 8px;
  font-size: 0.7rem;
  font-weight: 900;
  color: rgba(255,255,255,0.5);
  letter-spacing: 0.02em;
}

.node-address {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 0.75rem;
  color: rgba(255,255,255,0.35);
  margin-top: 0.4rem;
  word-break: break-all;
}

.node-address .sep { opacity: 0.2; margin: 0 4px; }
.node-address .domain { color: #818cf8; font-weight: 600; }

.badge-row {
  display: flex;
  gap: 1rem;
  margin-bottom: 2rem;
}

.info-pill {
  flex: 1;
  background: rgba(0,0,0,0.3);
  border: 1px solid rgba(255,255,255,0.04);
  padding: 0.65rem 1rem;
  border-radius: 1.25rem;
  display: flex;
  flex-direction: column;
}

.info-pill .label {
  font-size: 0.55rem;
  text-transform: uppercase;
  font-weight: 900;
  color: rgba(255,255,255,0.25);
  letter-spacing: 0.12em;
}

.info-pill .val {
  font-size: 0.9rem;
  font-weight: 800;
  color: #fff;
  margin-top: 4px;
  letter-spacing: -0.01em;
}

.info-pill.active .val { color: #34d399; text-shadow: 0 0 10px rgba(52, 211, 153, 0.3); }

.control-footer {
  margin-top: auto;
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: rgba(255,255,255,0.04);
  padding: 0.75rem 1rem;
  border-radius: 1.5rem;
  border: 1px solid rgba(255,255,255,0.02);
}

.rotate-toggle {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  cursor: pointer;
  font-size: 0.8rem;
  font-weight: 700;
  color: rgba(255,255,255,0.5);
  user-select: none;
}

.rotate-toggle:hover { color: #fff; }

.switch {
  width: 36px;
  height: 20px;
  background: rgba(255,255,255,0.12);
  border-radius: 20px;
  position: relative;
  transition: all 0.4s cubic-bezier(0.19, 1, 0.22, 1);
}

.switch.on { background: #6366f1; }
.switch .thumb {
  width: 14px;
  height: 14px;
  background: #fff;
  border-radius: 50%;
  position: absolute;
  top: 3px;
  left: 3px;
  transition: all 0.4s cubic-bezier(0.19, 1, 0.22, 1);
  box-shadow: 0 2px 4px rgba(0,0,0,0.2);
}
.switch.on .thumb { transform: translateX(16px); }

.btn-action {
  background: #6366f1;
  color: #fff;
  padding: 0.6rem 1.25rem;
  border-radius: 1.15rem;
  font-size: 0.75rem;
  font-weight: 900;
  display: flex;
  align-items: center;
  gap: 0.6rem;
  transition: all 0.3s cubic-bezier(0.19, 1, 0.22, 1);
  box-shadow: 0 10px 25px -5px rgba(99, 102, 241, 0.4);
}

.btn-action.amber {
  background: linear-gradient(135deg, #f59e0b, #d97706);
  box-shadow: 0 10px 25px -5px rgba(245, 158, 11, 0.4);
}

.btn-action:hover {
  transform: scale(1.05) translateY(-2px);
  filter: brightness(1.15);
}

.btn-icon-bg {
  width: 2.75rem;
  height: 2.75rem;
  background: rgba(255,255,255,0.07);
  border-radius: 1.15rem;
  display: flex;
  align-items: center;
  justify-content: center;
  color: rgba(255,255,255,0.45);
  transition: all 0.3s;
  border: 1px solid rgba(255,255,255,0.03);
}

.btn-icon-bg:hover {
  background: rgba(255,255,255,0.15);
  color: #fff;
  border-color: rgba(255,255,255,0.1);
}

/* SIDE B: Stats Pulse */
.side-stats {
  flex: 1;
  background: rgba(0,0,0,0.35);
  border-radius: 2rem;
  padding: 1.75rem;
  border: 1px solid rgba(255,255,255,0.05);
  display: flex;
  align-items: center;
}

.stats-container {
  display: flex;
  gap: 2.5rem;
  width: 100%;
}

.gauge-stack {
  flex: 0 0 180px;
  display: flex;
  flex-direction: column;
  justify-content: space-around;
  gap: 1.25rem;
}

.gauge-item { width: 100%; }

.gauge-info {
  display: flex;
  justify-content: space-between;
  font-size: 0.65rem;
  font-weight: 900;
  color: rgba(255,255,255,0.25);
  margin-bottom: 0.5rem;
  letter-spacing: 0.15em;
}

.gauge-info .val { font-family: 'JetBrains Mono', monospace; color: rgba(255,255,255,0.7); font-size: 0.8rem; }
.gauge-info .val.critical { color: #ef4444; font-weight: 900; }

.progress-track {
  height: 8px;
  background: rgba(255,255,255,0.06);
  border-radius: 10px;
  overflow: hidden;
  box-shadow: inset 0 1px 2px rgba(0,0,0,0.2);
}

.progress-bar {
  height: 100%;
  border-radius: 10px;
  transition: width 1.5s cubic-bezier(0.19, 1, 0.22, 1);
  position: relative;
}

.progress-bar::after {
  content: '';
  position: absolute;
  top: 0; left: 0; right: 0; bottom: 0;
  background: linear-gradient(90deg, transparent, rgba(255,255,255,0.2), transparent);
  animation: shine 2s infinite;
}

@keyframes shine { 0% { transform: translateX(-100%); } 100% { transform: translateX(100%); } }

.progress-bar.cpu { background: linear-gradient(90deg, #10b981, #059669); }
.progress-bar.mem { background: linear-gradient(90deg, #3b82f6, #2563eb); }
.progress-bar.disk { background: linear-gradient(90deg, #f59e0b, #d97706); }

.metrics-grid {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.metric-card {
  background: rgba(255,255,255,0.03);
  padding: 1rem 1.25rem;
  border-radius: 1.5rem;
  border: 1px solid rgba(255,255,255,0.02);
  box-shadow: 0 4px 12px rgba(0,0,0,0.1);
}

.m-label {
  font-size: 0.6rem;
  font-weight: 900;
  color: rgba(255,255,255,0.2);
  letter-spacing: 0.2em;
  display: block;
  margin-bottom: 0.65rem;
}

.m-val-pair {
  display: flex;
  justify-content: space-between;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.95rem;
  font-weight: 800;
  letter-spacing: -0.02em;
}

.m-val-pair .up { color: #34d399; }
.m-val-pair .down { color: #60a5fa; }

.metric-mini-grid {
  display: grid;
  grid-template-cols: 1fr 1fr;
  gap: 1rem;
}

.m-box {
  background: rgba(255,255,255,0.03);
  padding: 0.85rem;
  border-radius: 1.25rem;
  display: flex;
  flex-direction: column;
  align-items: center;
  border: 1px solid rgba(255,255,255,0.01);
}

.m-box.full { grid-column: span 2; flex-direction: row; justify-content: space-between; padding: 0.75rem 1.25rem; }

.m-box .sl { font-size: 0.55rem; font-weight: 900; color: rgba(255,255,255,0.2); letter-spacing: 0.15em; }
.m-box .sv { font-size: 1.05rem; font-weight: 800; color: #fff; margin-top: 4px; font-family: 'JetBrains Mono', monospace; }
.m-box .sv.uptime { font-size: 0.8rem; color: #a5b4fc; }

.empty-stats {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: rgba(255,255,255,0.2);
  gap: 1rem;
}

.pulse-loader {
  width: 48px;
  height: 48px;
  border: 3px solid rgba(99, 102, 241, 0.2);
  border-top-color: #6366f1;
  border-radius: 50%;
  animation: spin 1.5s linear infinite;
}

@keyframes spin { to { transform: rotate(360deg); } }

.empty-stats p { font-size: 0.7rem; font-weight: 900; letter-spacing: 0.25em; color: rgba(255,255,255,0.3); }

/* SIDE C: Utility Actions */
.side-actions {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  justify-content: center;
}

.action-btn {
  width: 3.25rem;
  height: 3.25rem;
  border-radius: 1.25rem;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255,255,255,0.03);
  color: rgba(255,255,255,0.25);
  transition: all 0.3s cubic-bezier(0.19, 1, 0.22, 1);
  border: 1px solid rgba(255,255,255,0.02);
}

.action-btn:hover { background: rgba(255,255,255,0.1); color: #fff; transform: translateX(4px); }
.action-btn.edit:hover { color: #818cf8; border-color: rgba(129, 140, 248, 0.3); box-shadow: 0 10px 20px rgba(0,0,0,0.2); }
.action-btn.delete:hover { color: #f87171; border-color: rgba(248, 113, 113, 0.3); box-shadow: 0 10px 20px rgba(0,0,0,0.2); }

@media (max-width: 1280px) {
  .card-grid { flex-direction: column; }
  .side-identity { flex: none; width: 100%; }
  .side-stats { width: 100%; }
  .side-actions { flex-direction: row; padding-top: 1rem; }
  .action-btn:hover { transform: translateY(-4px); }
}
</style>
