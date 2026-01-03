<script setup>
import { ref } from 'vue'
import { useApi } from '../../composables/useApi'

const props = defineProps(['nodes', 'line'])
const emit = defineEmits(['close', 'saved'])

const { apiPost } = useApi()

const form = ref(props.line ? { ...props.line } : {
  name: '',
  transit_node_id: null,
  exit_node_id: null,
  price: 1.0,
  is_public: true
})

async function save() {
  try {
    if (!form.value.transit_node_id || !form.value.exit_node_id) throw new Error('请选择入口和出口节点')
    await apiPost('/api/v1/ultra/lines', form.value)
    emit('saved')
    emit('close')
  } catch (e) {
    alert('保存失败: ' + e.message)
  }
}
</script>

<template>
  <div class="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-fade-in">
    <div class="glass w-full max-w-md p-8 rounded-3xl shadow-2xl animate-slide-up border border-primary-500/20">
      <h3 class="text-xl font-bold mb-6">{{ line ? '编辑线路' : '创建新线路 (Line)' }}</h3>
      
      <div class="space-y-4">
        <div class="space-y-1">
          <label class="text-[10px] uppercase font-bold text-gray-400">线路名称</label>
          <input v-model="form.name" type="text" placeholder="例如：上海-东京 游戏专线" class="w-full bg-[var(--bg-secondary)] border border-[var(--border-color)] rounded-xl p-3 text-sm focus:border-primary-500 transition outline-none" />
        </div>

        <div class="space-y-1">
          <label class="text-[10px] uppercase font-bold text-indigo-400">入口节点 (Transit)</label>
          <select v-model="form.transit_node_id" class="w-full bg-[var(--bg-secondary)] border border-[var(--border-color)] rounded-xl p-3 text-xs focus:border-primary-500 transition outline-none">
            <option v-for="n in nodes" :key="n.id" :value="n.id">{{ n.name }} ({{ n.public_addr }})</option>
          </select>
        </div>

        <div class="space-y-1">
          <label class="text-[10px] uppercase font-bold text-emerald-400">出口节点 (Exit)</label>
          <select v-model="form.exit_node_id" class="w-full bg-[var(--bg-secondary)] border border-[var(--border-color)] rounded-xl p-3 text-xs focus:border-primary-500 transition outline-none">
            <option v-for="n in nodes" :key="n.id" :value="n.id">{{ n.name }} ({{ n.public_addr }})</option>
          </select>
        </div>

        <div class="space-y-1">
          <label class="text-[10px] uppercase font-bold text-primary-500">流量倍率 (Multiplier)</label>
          <div class="flex items-center gap-4">
            <input v-model.number="form.price" type="number" step="0.1" class="w-full bg-[var(--bg-secondary)] border border-[var(--border-color)] rounded-xl p-3 text-sm font-mono focus:border-primary-500 transition outline-none" />
            <span class="text-xs text-gray-400 whitespace-nowrap">用户消耗 = 实际 * {{ form.price }}</span>
          </div>
        </div>

        <div class="flex gap-4 mt-8">
          <button @click="emit('close')" class="flex-1 py-3 rounded-2xl bg-[var(--bg-secondary)] hover:bg-white/5 transition font-bold text-sm">取消</button>
          <button @click="save" class="flex-[2] py-3 rounded-2xl bg-primary-500 font-bold text-sm shadow-xl shadow-primary-500/20 active:scale-95 transition">保存线路</button>
        </div>
      </div>
    </div>
  </div>
</template>
