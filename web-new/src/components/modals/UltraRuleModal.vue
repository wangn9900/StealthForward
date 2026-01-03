<script setup>
import { ref, computed } from 'vue'
import { useApi } from '../../composables/useApi'

const props = defineProps(['nodes'])
const emit = defineEmits(['close', 'saved'])

const { apiPost } = useApi()

const form = ref({
  name: '',
  node_id: null,
  exit_node_id: null,
  listen_port: null,
  tunnel_port: Math.floor(Math.random() * (50000 - 20000) + 20000), // 自动分配一个高位端口
  local_dest: '127.0.0.1:8443',
  key: Math.random().toString(36).substring(2, 34),
  status: true
})

const selectedExitNode = computed(() => {
  return props.nodes.find(n => n.id === form.value.exit_node_id)
})

async function save() {
  try {
    if (!form.value.node_id || !form.value.exit_node_id) throw new Error('请完整选择入口和出口机')
    await apiPost('/api/v1/ultra/rules', form.value)
    emit('saved')
    emit('close')
  } catch (e) {
    alert('保存失败: ' + e.message)
  }
}
</script>

<template>
  <div class="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-fade-in">
    <div class="glass w-full max-w-lg p-8 rounded-3xl shadow-2xl animate-slide-up border border-primary-500/20">
      <div class="flex justify-between items-center mb-6">
        <div>
          <h3 class="text-xl font-bold">添加专线规则 (Private Relay)</h3>
          <p class="text-[var(--text-muted)] text-[10px] mt-1">组合两台独立服务器，建立 Stealth-Pass 隐身隧道</p>
        </div>
        <button @click="emit('close')" class="p-2 hover:bg-white/10 rounded-full transition">
          <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <div class="space-y-4">
        <div class="space-y-1">
          <label class="text-[10px] uppercase font-bold text-[var(--text-muted)]">规则名称</label>
          <input v-model="form.name" type="text" placeholder="例如：广移-香港-游戏专线" class="w-full bg-[var(--bg-secondary)] border border-[var(--border-color)] rounded-xl p-3 text-sm focus:border-primary-500 transition outline-none" />
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div class="space-y-1">
            <label class="text-[10px] uppercase font-bold text-indigo-400">入口机 (Transit)</label>
            <select v-model="form.node_id" class="w-full bg-[var(--bg-secondary)] border border-[var(--border-color)] rounded-xl p-3 text-xs focus:border-primary-500 transition outline-none appearance-none">
              <option :value="null" disabled>选择入口节点...</option>
              <option v-for="n in nodes" :key="n.id" :value="n.id">{{ n.name }}</option>
            </select>
          </div>
          <div class="space-y-1">
            <label class="text-[10px] uppercase font-bold text-emerald-400">出口机 (Exit)</label>
            <select v-model="form.exit_node_id" class="w-full bg-[var(--bg-secondary)] border border-[var(--border-color)] rounded-xl p-3 text-xs focus:border-primary-500 transition outline-none appearance-none">
              <option :value="null" disabled>选择出口节点...</option>
              <option v-for="n in nodes" :key="n.id" :value="n.id">{{ n.name }}</option>
            </select>
          </div>
        </div>

        <div class="grid grid-cols-2 gap-4 bg-primary-500/5 p-4 rounded-2xl border border-primary-500/10">
           <div class="space-y-1">
            <label class="text-[10px] uppercase font-bold text-[var(--text-muted)]">入口监听端口</label>
            <input v-model.number="form.listen_port" type="number" placeholder="用户连接此端口" class="w-full bg-black/20 border border-white/5 rounded-lg p-2 text-xs font-mono outline-none" />
          </div>
          <div class="space-y-1">
            <label class="text-[10px] uppercase font-bold text-[var(--text-muted)]">隧道通讯端口</label>
            <input v-model.number="form.tunnel_port" type="number" placeholder="出口机监听端口" class="w-full bg-black/20 border border-white/5 rounded-lg p-2 text-xs font-mono outline-none" />
          </div>
        </div>

        <div class="space-y-1">
          <label class="text-[10px] uppercase font-bold text-amber-400">落地目标 (Destination)</label>
          <input v-model="form.local_dest" type="text" placeholder="127.0.0.1:8443 或 IP:Port" class="w-full bg-[var(--bg-secondary)] border border-[var(--border-color)] rounded-xl p-3 text-sm focus:border-amber-500 transition outline-none font-mono" />
        </div>

        <div class="p-4 rounded-xl border border-dashed border-[var(--border-color)] bg-black/10">
           <div class="text-[10px] uppercase font-bold text-[var(--text-muted)] mb-2">链路预览 (Topology)</div>
           <div class="flex items-center justify-between text-[10px] font-mono">
              <span class="text-indigo-400">User</span>
              <span class="text-[var(--text-muted)]">→</span>
              <span class="text-indigo-400">:{{ form.listen_port || '??' }}</span>
              <span class="text-[var(--text-muted)]">━━(StealthPass)━━</span>
              <span class="text-emerald-400">{{ selectedExitNode?.public_addr || '出口机' }}:{{ form.tunnel_port }}</span>
              <span class="text-[var(--text-muted)]">→</span>
              <span class="text-amber-400">{{ form.local_dest }}</span>
           </div>
        </div>

        <div class="flex gap-4 mt-6">
          <button @click="emit('close')" class="flex-1 py-3 rounded-2xl bg-[var(--bg-secondary)] hover:bg-white/5 transition font-bold text-sm">取消</button>
          <button @click="save" class="flex-[2] py-3 rounded-2xl bg-gradient-to-r from-primary-500 to-indigo-600 font-bold text-sm shadow-xl shadow-primary-500/20 active:scale-95 transition">确定组合</button>
        </div>
      </div>
    </div>
  </div>
</template>
