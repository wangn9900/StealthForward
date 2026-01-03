<script setup>
import { ref } from 'vue'
import { useApi } from '../../composables/useApi'

const props = defineProps(['node'])
const emit = defineEmits(['close', 'saved'])

const { apiPost } = useApi()

const form = ref({
  name: '',
  public_addr: '',
  internal_addr: '',
  ssh_host: '',
  ssh_port: 22,
  ssh_user: 'root',
  ssh_pass: ''
})

async function save() {
  try {
    if (!form.value.internal_addr) form.value.internal_addr = form.value.public_addr
    await apiPost('/api/v1/ultra/nodes', form.value)
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
        <h3 class="text-xl font-bold">添加入口机 (Transit Asset)</h3>
        <button @click="emit('close')" class="p-2 hover:bg-white/10 rounded-full transition">
          <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <div class="space-y-4">
        <div class="space-y-1">
          <label class="text-[10px] uppercase font-bold text-[var(--text-muted)]">节点名称</label>
          <input v-model="form.name" type="text" placeholder="例如：广州移动中转-01" class="w-full bg-[var(--bg-secondary)] border border-[var(--border-color)] rounded-xl p-3 text-sm focus:border-primary-500 transition outline-none" />
        </div>

        <div class="grid grid-cols-2 gap-4">
           <div class="space-y-1">
            <label class="text-[10px] uppercase font-bold text-[var(--text-muted)]">公网访问地址</label>
            <input v-model="form.public_addr" type="text" placeholder="IP 或 域名" class="w-full bg-[var(--bg-secondary)] border border-[var(--border-color)] rounded-xl p-3 text-sm focus:border-primary-500 transition outline-none font-mono" />
          </div>
          <div class="space-y-1">
            <label class="text-[10px] uppercase font-bold text-[var(--text-muted)]">内网地址(可选)</label>
            <input v-model="form.internal_addr" type="text" placeholder="留空则同公网" class="w-full bg-[var(--bg-secondary)] border border-[var(--border-color)] rounded-xl p-3 text-sm focus:border-primary-500 transition outline-none font-mono" />
          </div>
        </div>

        <div class="p-4 rounded-xl bg-indigo-500/5 border border-indigo-500/10 mt-6">
          <div class="text-[10px] uppercase font-bold text-indigo-400 mb-4 flex items-center gap-1">
            <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
               <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
            </svg>
            SSH 自动部署配置 (加密存储)
          </div>
          <div class="grid grid-cols-2 gap-4">
             <div class="space-y-1">
              <label class="text-[10px] uppercase font-bold text-[var(--text-muted)]">SSH 主机</label>
              <input v-model="form.ssh_host" type="text" placeholder="通常同公网 IP" class="w-full bg-black/20 border border-white/5 rounded-lg p-2 text-xs font-mono outline-none" />
            </div>
             <div class="space-y-1">
              <label class="text-[10px] uppercase font-bold text-[var(--text-muted)]">SSH 端口</label>
              <input v-model.number="form.ssh_port" type="number" class="w-full bg-black/20 border border-white/5 rounded-lg p-2 text-xs font-mono outline-none" />
            </div>
            <div class="space-y-1">
              <label class="text-[10px] uppercase font-bold text-[var(--text-muted)]">SSH 用户</label>
              <input v-model="form.ssh_user" type="text" class="w-full bg-black/20 border border-white/5 rounded-lg p-2 text-xs font-mono outline-none" />
            </div>
            <div class="space-y-1">
              <label class="text-[10px] uppercase font-bold text-[var(--text-muted)]">SSH 密码</label>
              <input v-model="form.ssh_pass" type="password" class="w-full bg-black/20 border border-white/5 rounded-lg p-2 text-xs font-mono outline-none" />
            </div>
          </div>
        </div>

        <div class="flex gap-4 mt-8">
          <button @click="emit('close')" class="flex-1 py-3 rounded-2xl bg-[var(--bg-secondary)] hover:bg-white/5 transition font-bold text-sm">取消</button>
          <button @click="save" class="flex-[2] py-3 rounded-2xl bg-gradient-to-r from-primary-500 to-indigo-600 font-bold text-sm shadow-xl shadow-primary-500/20 active:scale-95 transition">保存并下一步</button>
        </div>
      </div>
    </div>
  </div>
</template>
