<script setup>
import { ref } from 'vue'

const user = ref({
  id: '64342184',
  group: '普通用户',
  plan: '星辰网络转发 - 1t',
  expired_at: '2026-02-01 21:24:01',
  renewal_price: 40,
  used_traffic: '844.67 GiB',
  total_traffic: '1024.00 GiB',
  max_rules: 30,
  balance: 0,
  auto_renewal: true
})
</script>

<template>
  <div class="space-y-6 animate-fade-in pb-20">
    <!-- User Info Section -->
    <div class="glass rounded-2xl border border-black/5 dark:border-white/5 bg-white dark:bg-[#1a1c1e] overflow-hidden">
      <div class="p-6 border-b border-gray-100 dark:border-gray-800">
        <h3 class="font-bold text-lg">用户信息</h3>
      </div>
      <div class="p-8 space-y-6">
        <div class="grid grid-cols-1 md:grid-cols-2 gap-y-6 gap-x-12 text-sm">
           <div class="flex flex-col gap-1">
              <span class="text-gray-400 text-xs">用户名</span>
              <span class="font-bold">{{ user.id }}</span>
           </div>
           <div class="flex flex-col gap-1">
              <span class="text-gray-400 text-xs">用户组</span>
              <span class="font-bold text-primary-500">{{ user.group }}</span>
           </div>
           <div class="flex flex-col gap-1">
              <span class="text-gray-400 text-xs">当前状态</span>
              <span class="font-bold">{{ user.plan }}</span>
           </div>
           <div class="flex flex-col gap-1">
              <span class="text-gray-400 text-xs">到期时间</span>
              <span class="font-bold text-rose-500">{{ user.expired_at }}</span>
           </div>
           <div class="flex flex-col gap-1">
              <span class="text-gray-400 text-xs">续费价格</span>
              <div class="flex items-center gap-4">
                 <span class="font-bold">{{ user.renewal_price }} 元</span>
                 <button class="px-3 py-1 rounded-lg bg-primary-500/10 text-primary-500 text-[10px] font-bold border border-primary-500/20 active:scale-95 transition">立即续费</button>
              </div>
           </div>
           <div class="flex flex-col gap-1">
              <span class="text-gray-400 text-xs">流量消耗</span>
              <span class="font-bold">{{ user.used_traffic }} / {{ user.total_traffic }}</span>
           </div>
           <div class="flex flex-col gap-1">
              <span class="text-gray-400 text-xs">钱包余额</span>
              <div class="flex items-center gap-4">
                 <span class="font-bold">{{ user.balance }} 元</span>
                 <button class="px-3 py-1 rounded-lg bg-emerald-500/10 text-emerald-500 text-[10px] font-bold border border-emerald-500/20 active:scale-95 transition">去充值</button>
              </div>
           </div>
           <div class="flex flex-col gap-1">
              <span class="text-gray-400 text-xs">Telegram 关联</span>
              <div class="flex items-center gap-2">
                 <button class="px-3 py-1 rounded-lg glass text-[10px] font-bold text-gray-500 active:scale-95 transition">绑定账号</button>
              </div>
           </div>
        </div>
      </div>
    </div>

    <!-- Settings Section -->
    <div class="glass rounded-2xl border border-black/5 dark:border-white/5 bg-white dark:bg-[#1a1c1e] overflow-hidden">
       <div class="p-6 border-b border-gray-100 dark:border-gray-800">
        <h3 class="font-bold text-lg">账户设置</h3>
      </div>
      <div class="p-8">
         <div class="flex items-center justify-between">
            <div class="space-y-1">
               <div class="font-bold">自动续费</div>
               <div class="text-[10px] text-gray-400 max-w-md">如果您的套餐临近到期，或者流量用完，系统将自动从余额扣费。请保证余额充足，否则会续费失败。</div>
            </div>
            <label class="relative inline-flex items-center cursor-pointer">
              <input type="checkbox" v-model="user.auto_renewal" class="sr-only peer">
              <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-primary-500"></div>
            </label>
         </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.glass {
  backdrop-filter: blur(12px);
}
</style>
