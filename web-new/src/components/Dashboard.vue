<script setup>
import { inject, ref, computed } from 'vue'
import EntryCard from './EntryCard.vue'
import ExitCard from './ExitCard.vue'
import EntryModal from './modals/EntryModal.vue'
import ExitModal from './modals/ExitModal.vue'
import ProvisionModal from './modals/ProvisionModal.vue'

const emit = defineEmits(['refresh'])

const entries = inject('entries')
const exits = inject('exits')
const rules = inject('rules')
const trafficStats = inject('trafficStats')

// Modals
const showEntryModal = ref(false)
const showExitModal = ref(false)
const showProvisionModal = ref(false)
const editingEntry = ref(null)
const editingExit = ref(null)

function openAddEntry() {
  editingEntry.value = null
  showEntryModal.value = true
}

function openEditEntry(entry) {
  editingEntry.value = { ...entry }
  showEntryModal.value = true
}

function openAddExit() {
  editingExit.value = null
  showExitModal.value = true
}

function openEditExit(exit) {
  editingExit.value = { ...exit }
  showExitModal.value = true
}

function handleSaved() {
  showEntryModal.value = false
  showExitModal.value = false
  showProvisionModal.value = false
  emit('refresh')
}
</script>

<template>
  <div class="space-y-8 animate-fade-in">
    <!-- Entry Nodes Section -->
    <div>
      <div class="flex justify-between items-center px-2 mb-4">
        <h2 class="text-xl font-bold flex items-center gap-2">
          <span class="w-2 h-2 bg-primary-500 rounded-full animate-pulse"></span>
          入站入口 (Entry)
        </h2>
        <div class="flex gap-3">
          <button
            @click="showProvisionModal = true"
            class="text-sm font-bold text-transparent bg-clip-text bg-gradient-to-r from-amber-400 to-orange-500 hover:scale-105 transition flex items-center gap-1"
          >
            <svg class="w-4 h-4 text-amber-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
            </svg>
            新建云端节点
          </button>
          <button @click="openAddEntry" class="text-sm text-primary-400 hover:underline">
            + 手动新增
          </button>
        </div>
      </div>
      
      <div class="grid grid-cols-1 gap-4">
        <EntryCard
          v-for="entry in entries"
          :key="entry.id"
          :entry="entry"
          @edit="openEditEntry"
          @refresh="emit('refresh')"
        />
        <div v-if="entries.length === 0" class="glass p-12 rounded-3xl text-center text-[var(--text-muted)]">
          暂无入口节点，点击右上角添加
        </div>
      </div>
    </div>

    <!-- Exit Nodes Section -->
    <div>
      <div class="flex justify-between items-center px-2 mb-4">
        <h2 class="text-xl font-bold flex items-center gap-2">分流落地 (Exit / Nodes)</h2>
        <button @click="openAddExit" class="text-sm text-emerald-400 hover:underline">+ 新增</button>
      </div>
      
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <ExitCard
          v-for="exit in exits"
          :key="exit.id"
          :exit="exit"
          @edit="openEditExit"
          @refresh="emit('refresh')"
        />
        <div v-if="exits.length === 0" class="glass p-8 rounded-3xl text-center text-[var(--text-muted)] col-span-full">
          暂无落地节点
        </div>
      </div>
    </div>

    <!-- Modals -->
    <EntryModal
      v-if="showEntryModal"
      :entry="editingEntry"
      @close="showEntryModal = false"
      @saved="handleSaved"
    />
    
    <ExitModal
      v-if="showExitModal"
      :exit="editingExit"
      @close="showExitModal = false"
      @saved="handleSaved"
    />
    
    <ProvisionModal
      v-if="showProvisionModal"
      @close="showProvisionModal = false"
      @saved="handleSaved"
    />
  </div>
</template>
