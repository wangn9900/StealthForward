<script setup>
import { ref } from 'vue'

const plans = ref([
  { id: 1, name: '基础套餐 - 200g', price: 15, traffic: '200.00 GB', rules: 10, type: '月付' },
  { id: 2, name: '进阶套餐 - 500g', price: 25, traffic: '500.00 GB', rules: 20, type: '月付' },
  { id: 3, name: '专业套餐 - 1t', price: 40, traffic: '1024.00 GB', rules: 30, type: '月付' },
  { id: 4, name: '旗舰套餐 - 2t', price: 75, traffic: '2048.00 GB', rules: 50, type: '月付' }
])
</script>

<template>
  <div class="space-y-8 animate-fade-in pb-20">
    <!-- Wallet Section -->
    <div class="glass rounded-2xl p-8 border border-black/5 dark:border-white/5 bg-white dark:bg-[#1a1c1e]">
       <h3 class="font-bold text-lg mb-6">钱包充值</h3>
       <div class="flex flex-wrap items-end gap-6">
          <div class="space-y-2">
             <label class="text-[10px] uppercase font-bold text-gray-400 pl-1">充值金额 (CNY)</label>
             <input type="number" placeholder="100.00" class="w-48 bg-gray-50 dark:bg-black/20 border border-gray-100 dark:border-white/5 rounded-xl p-3 outline-none focus:border-primary-500 transition" />
          </div>
          <button class="px-8 py-3 rounded-xl bg-primary-500 text-white font-bold text-sm shadow-xl shadow-primary-500/20 active:scale-95 transition">立即充值</button>
          <div class="text-[10px] text-gray-400 pb-3">最小充值金额: 10 元</div>
       </div>
    </div>

    <!-- Plans Section -->
    <div>
       <h3 class="font-bold text-lg mb-6 pl-2">订购套餐</h3>
       <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
          <div v-for="p in plans" :key="p.id" class="glass rounded-2xl p-6 border border-black/5 dark:border-white/5 bg-white dark:bg-[#1a1c1e] hover:border-primary-500/50 transition-all group hover:-translate-y-1">
             <div class="text-sm font-bold mb-4">{{ p.name }}</div>
             <div class="space-y-3 mb-6 text-xs text-gray-500">
                <div class="flex justify-between">
                   <span>类型</span>
                   <span class="text-gray-900 dark:text-gray-100 font-medium">{{ p.type }}</span>
                </div>
                <div class="flex justify-between">
                   <span>最大规则数</span>
                   <span class="text-gray-900 dark:text-gray-100 font-medium">{{ p.rules }}</span>
                </div>
                <div class="flex justify-between">
                   <span>包含流量</span>
                   <span class="text-gray-900 dark:text-gray-100 font-medium">{{ p.traffic }}</span>
                </div>
             </div>
             <div class="flex items-baseline gap-1 mb-6">
                <span class="text-2xl font-black text-primary-500">{{ p.price }}</span>
                <span class="text-[10px] font-bold text-gray-400">元 / {{ p.type }}</span>
             </div>
             <button class="w-full py-3 rounded-xl bg-gray-50 dark:bg-white/5 hover:bg-primary-500 hover:text-white transition-all font-bold text-xs">立即订购</button>
          </div>
       </div>
    </div>
  </div>
</template>
