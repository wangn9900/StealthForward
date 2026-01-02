<script setup>
import { ref, inject, onMounted } from 'vue'
import { useApi } from '../../composables/useApi'

const props = defineProps({
  entry: Object
})

const emit = defineEmits(['close', 'saved'])

const exits = inject('exits')
const { apiPost } = useApi()

const form = ref({
  id: null,
  name: '',
  domain: '',
  port: 443,
  certificate: '',
  key: '',
  fallback: '127.0.0.1:80',
  target_exit_id: 0,
  v2board_url: '',
  v2board_key: '',
  v2board_node_id: null,
  v2board_type: 'v2ray'
})

const saving = ref(false)

onMounted(() => {
  if (props.entry) {
    form.value = { ...props.entry }
  }
})

async function handleSubmit() {
  saving.value = true
  try {
    await apiPost('/api/v1/entries', form.value)
    emit('saved')
  } catch (e) {
    alert('保存失败: ' + e.message)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4" @click.self="$emit('close')">
    <div class="glass w-full max-w-xl p-8 rounded-3xl animate-slide-up">
      <h3 class="text-2xl font-bold mb-6">{{ entry ? '编辑' : '新增' }}入站节点</h3>
      
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
        <label class="md:col-span-2 flex flex-col gap-1.5 text-[var(--text-muted)]">
          显示名称
          <input v-model="form.name" placeholder="美国 01 / 日本入口" />
        </label>
        
        <label class="flex flex-col gap-1.5 text-[var(--text-muted)]">
          解析域名 (TLS)
          <input v-model="form.domain" placeholder="example.com" />
        </label>
        
        <label class="flex flex-col gap-1.5 text-[var(--text-muted)]">
          监听端口
          <input type="number" v-model.number="form.port" placeholder="443" />
        </label>
        
        <label class="flex flex-col gap-1.5 text-[var(--text-muted)]">
          证书路径
          <input v-model="form.certificate" placeholder="/etc/stealthforward/certs/cert.crt" />
        </label>
        
        <label class="flex flex-col gap-1.5 text-[var(--text-muted)]">
          私钥路径
          <input v-model="form.key" placeholder="/etc/stealthforward/certs/cert.key" />
        </label>
        
        <label class="md:col-span-2 flex flex-col gap-1.5 text-[var(--text-muted)]">
          回落托管 (HTTP)
          <input v-model="form.fallback" placeholder="127.0.0.1:80" />
        </label>
        
        <div class="md:col-span-2 text-primary-400 font-bold mt-2">V2Board API 同步 (可选)</div>
        
        <input class="md:col-span-2" v-model="form.v2board_url" placeholder="API 地址: https://v2.mysite.com" />
        
        <input v-model="form.v2board_key" type="password" placeholder="通讯令牌 (Key)" />
        
        <div class="grid grid-cols-2 gap-2">
          <input type="number" v-model.number="form.v2board_node_id" placeholder="默认节点ID" />
          <select v-model="form.v2board_type">
            <option value="v2ray">V2ray</option>
            <option value="vless">VLESS</option>
            <option value="shadowsocks">Shadowsocks</option>
            <option value="anytls">AnyTLS</option>
          </select>
        </div>
        
        <div class="md:col-span-2 text-primary-400 font-bold mt-2">目标落地机 (流量转发目的地)</div>
        
        <select class="md:col-span-2" v-model.number="form.target_exit_id">
          <option :value="0">不绑定 (所有用户将无法连接)</option>
          <option v-for="ex in exits" :key="ex.id" :value="ex.id">{{ ex.name }} — 发往此机器</option>
        </select>
      </div>
      
      <div class="flex gap-4 mt-8">
        <button @click="$emit('close')" class="flex-1 p-4 bg-[var(--bg-secondary)] rounded-2xl">取消</button>
        <button
          @click="handleSubmit"
          :disabled="saving"
          class="flex-1 p-4 bg-primary-600 rounded-2xl font-bold disabled:opacity-50"
        >
          {{ saving ? '保存中...' : '保存节点' }}
        </button>
      </div>
    </div>
  </div>
</template>
