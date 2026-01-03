<script setup>
defineProps({
  activeTab: String,
  isDark: Boolean
})

const emit = defineEmits(['update:activeTab', 'toggle-theme', 'refresh', 'logout'])

const tabs = [
  { key: 'dashboard', label: '概览' },
  { key: 'mappings', label: '配置' },
  { key: 'settings', label: '系统' }
]
</script>

<template>
  <div class="flex flex-col md:flex-row justify-between items-start md:items-center mb-8 gap-4">
    <!-- Logo & Title -->
    <div>
      <h1 class="text-4xl font-extrabold tracking-tighter gradient-text">StealthForward v3.4.7</h1>
      <p class="text-[var(--text-muted)] text-sm mt-1">First-Principles 架构 | 隐形中转分流中心</p>
    </div>

    <!-- Controls -->
    <div class="flex gap-3 items-center">
      <!-- Tab Switcher -->
      <div class="glass flex p-1 rounded-2xl items-center">
        <div
          v-for="tab in tabs"
          :key="tab.key"
          @click="$emit('update:activeTab', tab.key)"
          :class="['tab-btn', activeTab === tab.key ? 'active' : '']"
        >
          {{ tab.label }}
        </div>
      </div>

      <!-- Theme Toggle -->
      <button
        @click="$emit('toggle-theme')"
        class="p-3 glass rounded-2xl hover:bg-[var(--bg-secondary)] transition"
        :title="isDark ? '切换到浅色模式' : '切换到深色模式'"
      >
        <!-- Sun icon (shown in dark mode) -->
        <svg v-if="isDark" class="w-5 h-5 text-amber-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
        </svg>
        <!-- Moon icon (shown in light mode) -->
        <svg v-else class="w-5 h-5 text-primary-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
        </svg>
      </button>

      <!-- Refresh -->
      <button
        @click="$emit('refresh')"
        class="p-3 px-5 glass rounded-2xl hover:bg-[var(--bg-secondary)] transition flex items-center gap-2"
      >
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
        刷新
      </button>

      <!-- Logout -->
      <button
        @click="$emit('logout')"
        class="p-3 px-5 glass rounded-2xl hover:bg-rose-500/10 transition flex items-center gap-2 text-rose-500"
      >
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
        </svg>
        退出
      </button>
    </div>
  </div>
</template>
