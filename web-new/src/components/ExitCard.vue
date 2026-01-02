<script setup>
import { computed } from 'vue'
import { useApi } from '../composables/useApi'

const props = defineProps({
  exit: Object
})

const emit = defineEmits(['edit', 'refresh'])

const { apiDelete } = useApi()

const config = computed(() => {
  try {
    return JSON.parse(props.exit.config)
  } catch {
    return {}
  }
})

async function handleDelete() {
  if (!confirm('确定删除此落地节点?')) return
  try {
    await apiDelete(`/api/v1/exits/${props.exit.id}`)
    emit('refresh')
  } catch (e) {
    alert('删除失败: ' + e.message)
  }
}
</script>

<template>
  <div class="glass p-5 rounded-3xl group border-l-4 border-emerald-500/30 shadow-lg hover:shadow-xl transition-shadow">
    <div class="flex justify-between items-start mb-3">
      <div>
        <div class="font-bold flex items-center gap-2">
          {{ exit.name }}
          <span class="bg-emerald-500/10 text-emerald-400 text-xs p-1 px-1.5 rounded uppercase">
            {{ exit.protocol }}
          </span>
        </div>
        <div class="text-xs text-[var(--text-muted)] mt-1 font-mono truncate max-w-[150px]">
          {{ config.server }}:{{ config.server_port }}
        </div>
      </div>
      
      <!-- Actions -->
      <div class="flex gap-1 opacity-0 group-hover:opacity-100 transition relative z-10">
        <button
          @click="$emit('edit', exit)"
          class="p-1.5 glass rounded-lg text-emerald-400 hover:scale-110 cursor-pointer"
        >
          <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z"></path>
          </svg>
        </button>
        <button
          @click="handleDelete"
          class="p-1.5 glass rounded-lg text-rose-500 hover:scale-110 cursor-pointer"
        >
          <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path>
          </svg>
        </button>
      </div>
    </div>
  </div>
</template>
