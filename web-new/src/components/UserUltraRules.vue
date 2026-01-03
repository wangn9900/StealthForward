<script setup>
import { ref, onMounted } from 'vue'
import { useApi } from '../composables/useApi'

const { apiGet, apiPost, apiDelete } = useApi()

const rules = ref([])
const userStats = ref({
  total_traffic: 1024 * 1024 * 1024 * 1024, // 1TB Default
  used_traffic: 771 * 1024 * 1024 * 1024,
  expired_at: '2026-02-01 21:24:01',
  max_rules: 30,
  current_rules: 0
})

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

async function fetchData() {
  try {
    const res = await apiGet('/api/v1/ultra/rules')
    rules.value = res || []
    userStats.value.current_rules = rules.value.length
  } catch (e) {
    console.error(e)
  }
}

onMounted(fetchData)

async function toggleStatus(rule) {
  rule.status = !rule.status
  // await apiPost(`/api/v1/ultra/rules/${rule.id}/toggle`)
}
</script>

<template>
  <div class="space-y-4 animate-fade-in text-[#333] dark:text-gray-200">
    <!-- Top Summary (Total Traffic, Rules, etc.) -->
    <div class="glass p-4 rounded-xl flex flex-wrap items-center gap-6 text-sm border border-black/5 dark:border-white/5 bg-white/50 dark:bg-black/50">
      <div class="flex items-center gap-2">
        <span class="text-xs uppercase font-bold text-gray-400">流量:</span>
        <span class="font-mono">{{ formatBytes(userStats.used_traffic) }} / {{ formatBytes(userStats.total_traffic) }}</span>
        <div class="w-32 h-2 bg-gray-200 dark:bg-gray-800 rounded-full overflow-hidden">
          <div class="h-full bg-primary-500" :style="{ width: (userStats.used_traffic / userStats.total_traffic * 100) + '%' }"></div>
        </div>
      </div>
      <div class="flex items-center gap-2 border-l border-gray-300 dark:border-gray-700 pl-6">
        <span class="text-xs uppercase font-bold text-gray-400">到期:</span>
        <span class="font-mono text-primary-500">{{ userStats.expired_at }}</span>
      </div>
      <div class="flex items-center gap-2 border-l border-gray-300 dark:border-gray-700 pl-6">
        <span class="text-xs uppercase font-bold text-gray-400">规则数:</span>
        <span class="font-mono">{{ userStats.current_rules }} / {{ userStats.max_rules }}</span>
      </div>
      <div class="ml-auto flex gap-2">
         <button class="p-2 px-4 rounded-lg bg-primary-500 text-white text-xs font-bold shadow-lg shadow-primary-500/20 active:scale-95 transition">添加规则</button>
         <button class="p-2 px-4 rounded-lg glass text-xs font-bold border border-gray-300 dark:border-gray-700 hover:bg-gray-100 dark:hover:bg-white/5 transition">批量导入</button>
      </div>
    </div>

    <!-- Rules Table -->
    <div class="glass rounded-xl overflow-hidden border border-black/5 dark:border-white/5 bg-white dark:bg-[#1a1c1e]">
      <table class="w-full text-left border-collapse">
        <thead class="bg-gray-50 dark:bg-gray-900/50 text-gray-500 dark:text-gray-400 text-[10px] uppercase font-bold">
          <tr>
            <th class="py-3 px-6"><input type="checkbox" class="rounded" /></th>
            <th class="py-3 px-6">规则名</th>
            <th class="py-3 px-6">入口</th>
            <th class="py-3 px-6">出口</th>
            <th class="py-3 px-6">已用流量</th>
            <th class="py-3 px-6">状态</th>
            <th class="py-3 px-6 text-right">操作</th>
          </tr>
        </thead>
        <tbody class="text-xs divide-y divide-gray-100 dark:divide-gray-800">
          <tr v-for="r in rules" :key="r.id" class="hover:bg-gray-50 dark:hover:bg-white/5 transition-colors group">
            <td class="py-4 px-6"><input type="checkbox" class="rounded" /></td>
            <td class="py-4 px-6">
              <div class="font-bold">{{ r.name }} <span class="text-gray-400 font-normal text-[10px] ml-1">(#{{ r.id }})</span></div>
            </td>
            <td class="py-4 px-6">
              <div class="flex flex-col gap-1">
                <div class="flex items-center gap-2">
                  <span class="font-medium">广州移动-2000Mbps</span>
                  <span class="px-1.5 py-0.5 rounded bg-green-500/10 text-green-500 text-[9px] font-bold border border-green-500/20">倍率 2.5</span>
                </div>
                <div class="font-mono text-[10px] text-primary-500 select-all">cu1.xcuuu.cn:{{ r.listen_port }}</div>
              </div>
            </td>
            <td class="py-4 px-6">
              <div class="flex flex-col gap-1">
                <div class="flex items-center gap-2">
                  <span class="font-medium text-gray-500">aws-香港(移动入口使用)</span>
                  <span class="px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-800 text-gray-400 text-[9px] font-bold">倍率 0</span>
                </div>
                <div class="font-mono text-[10px] text-gray-400 select-all">ynyn.808622.xyz:{{ r.tunnel_port }}</div>
              </div>
            </td>
            <td class="py-4 px-6">
              <div class="font-mono font-bold text-gray-600 dark:text-gray-300">
                {{ formatBytes(r.upload + r.download) }}
              </div>
            </td>
            <td class="py-4 px-6">
              <span class="inline-flex items-center gap-1.5" :class="r.status ? 'text-green-500' : 'text-gray-400'">
                <span class="w-1.5 h-1.5 rounded-full" :class="r.status ? 'bg-green-500 animate-pulse' : 'bg-gray-400'"></span>
                {{ r.status ? '正常' : '已停止' }}
              </span>
            </td>
            <td class="py-4 px-6">
              <div class="flex justify-end gap-1">
                <button @click="toggleStatus(r)" class="p-1.5 hover:bg-gray-100 dark:hover:bg-white/10 rounded-lg transition" title="暂停/启动">
                   <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                </button>
                <button class="p-1.5 hover:bg-gray-100 dark:hover:bg-white/10 rounded-lg transition" title="统计">
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 8v8m-4-5v5m-4-2v2m-2 4h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
                  </svg>
                </button>
                <button class="p-1.5 hover:bg-gray-100 dark:hover:bg-white/10 rounded-lg transition" title="编辑">
                   <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                  </svg>
                </button>
                 <button class="p-1.5 hover:bg-gray-100 dark:hover:bg-white/10 rounded-lg transition text-rose-500" title="删除">
                   <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                  </svg>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.glass {
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
}
</style>
