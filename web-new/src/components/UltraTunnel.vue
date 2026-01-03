<script setup>
import { ref, onMounted } from 'vue'
import { useApi } from '../composables/useApi'
import UltraNodeModal from './modals/UltraNodeModal.vue'
import UltraRuleModal from './modals/UltraRuleModal.vue'

const { apiGet, apiDelete, apiPost } = useApi()

const nodes = ref([])
const rules = ref([])
const showNodeModal = ref(false)
const showRuleModal = ref(false)
const editingNode = ref(null)
const editingRule = ref(null)

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

async function refreshData() {
  try {
    const [nRes, rRes] = await Promise.all([
      apiGet('/api/v1/ultra/nodes'),
      apiGet('/api/v1/ultra/rules')
    ])
    nodes.value = nRes || []
    rules.value = rRes || []
  } catch (e) {
    console.error('Failed to fetch ultra tunnel data:', e)
  }
}

onMounted(refreshData)

function openAddNode() {
  editingNode.value = null
  showNodeModal.value = true
}

function openAddRule() {
  editingRule.value = null
  showRuleModal.value = true
}

async function handleDeleteRule(id) {
  if (!confirm('确定删除此转发规则?')) return
  await apiDelete(`/api/v1/ultra/rules/${id}`)
  refreshData()
}

async function deployNode(node) {
  if (!confirm(`确定开始对 ${node.name} 进行 SSH 自动部署吗? 这将安装 Stealth-Pass Agent。`)) return
  node.status = 'deploying'
  try {
    await apiPost(`/api/v1/ultra/nodes/${node.id}/deploy`)
    alert('部署任务已在后台启动，请稍后刷新查看状态。')
    refreshData()
  } catch (e) {
    alert('部署启动失败: ' + e.message)
    refreshData()
  }
}
</script>

<template>
  <div class="space-y-6 animate-fade-in">
    <!-- 头部统计与操作 -->
    <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
      <div class="glass p-8 rounded-3xl border-l-4 border-primary-500">
        <div class="flex justify-between items-start">
          <div>
            <h2 class="text-2xl font-bold bg-gradient-to-r from-primary-400 to-indigo-400 bg-clip-text text-transparent">高级隧道 (Independent Relay)</h2>
            <p class="text-[var(--text-muted)] text-sm mt-2">自研 Stealth-Pass 传输协议 | GFW 零特征识别 | 独立于 V2Board</p>
          </div>
          <div class="flex gap-2">
             <button @click="openAddNode" class="p-2 px-4 rounded-full bg-primary-500/10 text-primary-400 hover:bg-primary-500 hover:text-white transition text-xs font-bold">
              添加入口机
            </button>
            <button @click="openAddRule" class="p-2 px-6 rounded-full bg-gradient-to-r from-primary-500 to-indigo-600 text-white shadow-lg shadow-primary-500/20 active:scale-95 transition text-xs font-bold">
              添加转发规则
            </button>
          </div>
        </div>
      </div>
      
      <div class="glass p-8 rounded-3xl flex items-center justify-around">
        <div class="text-center">
          <div class="text-3xl font-bold font-mono text-primary-400">{{ nodes.length }}</div>
          <div class="text-[var(--text-muted)] text-xs uppercase mt-1">中转机</div>
        </div>
        <div class="w-px h-12 bg-[var(--border-color)]"></div>
        <div class="text-center">
          <div class="text-3xl font-bold font-mono text-emerald-400">{{ rules.length }}</div>
          <div class="text-[var(--text-muted)] text-xs uppercase mt-1">活跃隧道</div>
        </div>
        <div class="w-px h-12 bg-[var(--border-color)]"></div>
        <div class="text-center">
          <div class="text-3xl font-bold font-mono text-amber-400">Stable</div>
          <div class="text-[var(--text-muted)] text-xs uppercase mt-1">协议状态</div>
        </div>
      </div>
    </div>

    <!-- 转发规则表 -->
    <div class="glass p-8 rounded-3xl">
      <h3 class="text-lg font-bold mb-6 flex items-center gap-2">
        <span class="w-2 h-2 rounded-full bg-primary-500"></span>
        转发规则管理
      </h3>
      <div class="overflow-hidden rounded-2xl border border-[var(--border-color)]">
        <table class="w-full text-left border-collapse">
          <thead class="bg-[var(--bg-secondary)] text-[var(--text-muted)] text-xs uppercase tracking-wider font-semibold">
            <tr>
              <th class="py-4 px-6">规则名称</th>
              <th class="py-4 px-6">入口 (Transit)</th>
              <th class="py-4 px-6">隧道出口 (Exit)</th>
              <th class="py-4 px-6">还原目标 (Dest)</th>
              <th class="py-4 px-6">已用流量 (Up/Down)</th>
              <th class="py-4 px-6">协议/密钥</th>
              <th class="py-4 px-6 text-right">操作</th>
            </tr>
          </thead>
          <tbody class="text-sm divide-y divide-[var(--border-color)]">
            <tr v-for="r in rules" :key="r.id" class="hover:bg-primary-500/5 transition group">
              <td class="py-4 px-6">
                <div class="font-bold">{{ r.name }}</div>
                <div class="text-[var(--text-muted)] text-xs">ID: {{ r.id }}</div>
              </td>
              <td class="py-4 px-6">
                <div class="flex flex-col">
                  <span class="font-mono text-indigo-400">Port: {{ r.listen_port }}</span>
                  <span class="text-xs text-[var(--text-muted)]">从节点 #{{ r.node_id }}</span>
                </div>
              </td>
              <td class="py-4 px-6">
                <span class="text-emerald-400 font-mono text-xs">{{ r.exit_addr }}</span>
              </td>
              <td class="py-4 px-6">
                <span class="text-amber-400 font-mono text-xs">{{ r.local_dest }}</span>
              </td>
              <td class="py-4 px-6">
                <div class="flex flex-col gap-0.5">
                   <div class="flex items-center gap-1">
                      <svg class="w-3 h-3 text-primary-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                         <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 11l5-5m0 0l5 5m-5-5v12" />
                      </svg>
                      <span class="font-mono text-[10px] text-primary-400">{{ formatBytes(r.upload) }}</span>
                   </div>
                    <div class="flex items-center gap-1">
                      <svg class="w-3 h-3 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                         <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 13l-5 5m0 0l-5-5m5-5v12" />
                      </svg>
                      <span class="font-mono text-[10px] text-emerald-400">{{ formatBytes(r.download) }}</span>
                   </div>
                </div>
              </td>
              <td class="py-4 px-6">
                <div class="flex items-center gap-2">
                   <span class="px-2 py-0.5 rounded bg-primary-500/10 text-primary-400 text-[10px] font-bold">STEALTH-PASS</span>
                   <span class="text-[var(--text-muted)] text-[10px] font-mono select-all">Key: {{ r.key.substring(0,6) }}...</span>
                </div>
              </td>
              <td class="py-4 px-6 text-right">
                <button @click="handleDeleteRule(r.id)" class="p-2 text-rose-500 hover:bg-rose-500/10 rounded-lg transition">
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                  </svg>
                </button>
              </td>
            </tr>
            <tr v-if="rules.length === 0">
               <td colspan="6" class="py-12 text-center text-[var(--text-muted)] italic">暂无高级中转规则</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- 中转机节点列表 -->
    <div class="glass p-8 rounded-3xl">
      <h3 class="text-lg font-bold mb-6 flex items-center gap-2">
        <span class="w-2 h-2 rounded-full bg-indigo-500"></span>
        中转机基础资产 (Independent Nodes)
      </h3>
      <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div v-for="n in nodes" :key="n.id" class="p-6 rounded-2xl bg-[var(--bg-secondary)] border border-[var(--border-color)] group hover:border-primary-500/50 transition">
          <div class="flex justify-between items-start mb-4">
            <div class="flex items-center gap-2">
              <div class="w-3 h-3 rounded-full" :class="n.status === 'online' ? 'bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.5)]' : 'bg-rose-500'"></div>
              <span class="font-bold text-sm">{{ n.name }}</span>
            </div>
            <span class="text-[10px] uppercase font-bold text-[var(--text-muted)]">{{ n.status }}</span>
          </div>
          <div class="space-y-2 mb-6">
            <div class="text-[10px] text-[var(--text-muted)] uppercase tracking-tighter">Public Address</div>
            <div class="font-mono text-xs text-primary-400 select-all">{{ n.public_addr }}</div>
            <div class="text-[10px] text-[var(--text-muted)] uppercase tracking-tighter mt-4">SSH INFO</div>
            <div class="font-mono text-[10px] text-[var(--text-muted)] break-all">{{ n.ssh_user }}@{{ n.ssh_host }}:{{ n.ssh_port }}</div>
          </div>
          <button @click="deployNode(n)" class="w-full py-2 rounded-xl bg-primary-500/10 text-primary-400 hover:bg-primary-500 hover:text-white transition text-[10px] font-bold">
            一键部署 Agent
          </button>
        </div>
      </div>
    </div>

    <!-- Modals -->
    <UltraNodeModal v-if="showNodeModal" @close="showNodeModal = false" @saved="refreshData" />
    <UltraRuleModal v-if="showRuleModal" :nodes="nodes" @close="showRuleModal = false" @saved="refreshData" />
  </div>
</template>
