<script setup>
import { ref, inject, onMounted, watch, computed } from 'vue'
import { useApi } from '../../composables/useApi'
import { useLicense } from '../../composables/useLicense'

const props = defineProps({
  entry: Object
})

const emit = defineEmits(['close', 'saved'])

const exits = inject('exits')
const { apiPost, apiGet } = useApi()
const { fetchLicenseInfo, isProtocolAllowed, isCloudEnabled, isPro, isAdmin } = useLicense()

// 授权信息
const licenseLoaded = ref(false)
const licenseLevel = ref('admin')

const form = ref({
  id: null,
  name: '',
  domain: '',
  ip: '',
  port: 443,
  protocol: 'anytls', // 新增：入口协议
  transport: 'tcp',   // 传输层：tcp, grpc, ws, h2
  grpc_service: '',   // gRPC service name
  certificate: '',
  key: '',
  fallback: '127.0.0.1:80',
  target_exit_id: 0,
  v2board_url: '',
  v2board_key: '',
  v2board_node_id: null,
  v2board_type: 'v2ray',
  // 云平台绑定
  cloud_provider: 'none',
  cloud_region: '',
  cloud_instance_id: '',
  cloud_record_name: '',
  auto_rotate_ip: false
})

const saving = ref(false)
const detecting = ref(false)
const loadingInstances = ref(false)
const cloudInstances = ref([])

// 可用协议列表（根据授权等级）
const availableProtocols = computed(() => {
  const all = [
    { value: 'anytls', label: 'AnyTLS', proOnly: false },
    { value: 'vless', label: 'VLESS+Vision', proOnly: true },
    { value: 'vmess', label: 'VMess', proOnly: true },
    { value: 'trojan', label: 'Trojan', proOnly: true },
  ]
  return all.map(p => ({
    ...p,
    disabled: p.proOnly && !isPro()
  }))
})

// 可用传输层列表
const availableTransports = computed(() => {
  return [
    { value: 'tcp', label: 'TCP (直连最优)' },
    { value: 'grpc', label: 'gRPC (抗审查)' },
    { value: 'ws', label: 'WebSocket' },
    { value: 'h2', label: 'HTTP/2' },
  ]
})

onMounted(async () => {
  // 加载授权信息
  const info = await fetchLicenseInfo()
  licenseLevel.value = info?.level || 'admin'
  licenseLoaded.value = true

  if (props.entry) {
    form.value = { ...props.entry }
    if (form.value.cloud_provider !== 'none' && form.value.cloud_region) {
      fetchInstances()
    }
  }
})

// 监听云服务商和区域变化，自动拉取实例列表
watch([() => form.value.cloud_provider, () => form.value.cloud_region], () => {
  if (form.value.cloud_provider !== 'none' && form.value.cloud_region) {
    fetchInstances()
  } else {
    cloudInstances.value = []
  }
})

async function fetchInstances() {
  if (!form.value.cloud_region) return
  loadingInstances.value = true
  try {
    const res = await apiGet(`/api/v1/cloud/instances?provider=${form.value.cloud_provider}&region=${form.value.cloud_region}`)
    cloudInstances.value = res || []
  } catch (e) {
    console.error('拉取实例列表失败', e)
  } finally {
    loadingInstances.value = false
  }
}

async function autoDetect() {
  if (!form.value.ip) {
    alert('请先填入节点当前公网 IP')
    return
  }
  detecting.value = true
  try {
    const res = await apiGet(`/api/v1/cloud/auto-detect?ip=${form.value.ip}`)
    form.value.cloud_provider = res.provider
    form.value.cloud_region = res.region
    form.value.cloud_instance_id = res.instance_id
    if (res.record_name) form.value.cloud_record_name = res.record_name
    alert('识别成功！已自动填充云平台绑定信息。')
  } catch (e) {
    alert('识别失败: ' + e.message + '。请检查 IP 是否属于该账户名下的 AWS/Lightsail 且已开启。')
  } finally {
    detecting.value = false
  }
}

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
  <div class="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4 overflow-y-auto" @click.self="$emit('close')">
    <div class="glass w-full max-w-xl p-8 rounded-3xl animate-slide-up my-8">
      <h3 class="text-2xl font-bold mb-6 text-white">{{ entry ? '编辑' : '新增' }}入站节点</h3>
      
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm max-h-[70vh] overflow-y-auto pr-2 custom-scrollbar">
        <label class="flex flex-col gap-1.5 text-[var(--text-muted)]">
          显示名称
          <input v-model="form.name" placeholder="美国 01 / 日本入口" />
        </label>

        <label class="flex flex-col gap-1.5 text-[var(--text-muted)]">
          入口协议
          <select v-model="form.protocol">
            <option 
              v-for="p in availableProtocols" 
              :key="p.value" 
              :value="p.value"
              :disabled="p.disabled"
            >
              {{ p.label }}{{ p.disabled ? ' (Pro版)' : '' }}
            </option>
          </select>
          <span v-if="licenseLevel === 'basic'" class="text-[10px] text-amber-500/60">
            升级到Pro版可解锁 VLESS/VMess 等全协议
          </span>
        </label>

        <!-- 传输层选项 (AnyTLS 不支持传输层封装，自动隐藏) -->
        <label v-if="form.protocol !== 'anytls'" class="flex flex-col gap-1.5 text-[var(--text-muted)]">
          传输层
          <select v-model="form.transport">
            <option 
              v-for="t in availableTransports" 
              :key="t.value" 
              :value="t.value"
            >
              {{ t.label }}
            </option>
          </select>
        </label>

        <label v-if="form.protocol !== 'anytls' && form.transport === 'grpc'" class="flex flex-col gap-1.5 text-[var(--text-muted)]">
          gRPC Service Name
          <input v-model="form.grpc_service" placeholder="grpc (留空使用默认)" />
        </label>

        <label class="md:col-span-2 flex flex-col gap-1.5 text-[var(--text-muted)]">
          节点当前公网 IP
          <div class="flex gap-2">
            <input v-model="form.ip" placeholder="1.2.3.4" class="flex-1" />
            <button 
              @click="autoDetect" 
              :disabled="detecting || !form.ip"
              class="px-4 bg-amber-500/10 text-amber-400 border border-amber-500/20 rounded-xl hover:bg-amber-500 hover:text-white transition disabled:opacity-30 whitespace-nowrap text-xs font-bold"
            >
              {{ detecting ? '探测中...' : '🔍 自动识别云绑定' }}
            </button>
          </div>
          <span class="text-[10px] text-amber-500/60 leading-tight">输入 IP 后点击识别，可自动找回所属 AWS 区域和实例 ID</span>
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
        
        <!-- V2Board Section -->
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
        
        <!-- Cloud Binding Section -->
        <div class="md:col-span-2 text-amber-400 font-bold mt-4 flex items-center gap-2 border-t border-white/5 pt-4">
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z" />
          </svg>
          云平台绑定 (一键换IP)
        </div>
        
        <label class="flex flex-col gap-1.5 text-[var(--text-muted)]">
          云平台
          <select v-model="form.cloud_provider">
            <option value="none">无 (非云机器)</option>
            <option value="aws_ec2">AWS EC2</option>
            <option value="aws_lightsail">AWS Lightsail</option>
          </select>
        </label>
        
        <label class="flex flex-col gap-1.5 text-[var(--text-muted)]">
          区域 (Region)
          <input v-model="form.cloud_region" placeholder="ap-northeast-1" :disabled="form.cloud_provider === 'none'" />
        </label>
        
        <label class="md:col-span-2 flex flex-col gap-1.5 text-[var(--text-muted)]">
          选择云实例 (Instance)
          <select 
            v-model="form.cloud_instance_id" 
            :disabled="form.cloud_provider === 'none' || loadingInstances"
            class="w-full"
          >
            <option value="">{{ loadingInstances ? '加载列表中...' : '请选择实例 (从当前账号/区域拉取)' }}</option>
            <option v-for="inst in cloudInstances" :key="inst.id" :value="inst.id">
              {{ inst.name || 'Unnamed' }} ({{ inst.id }}) - {{ inst.public_ip }}
            </option>
          </select>
          <div v-if="cloudInstances.length === 0 && form.cloud_region" class="text-[10px] text-rose-400/80">未在该区域发现可用实例，请检查区域代码或账号权限。</div>
        </label>
        
        <label class="md:col-span-2 flex flex-col gap-1.5 text-[var(--text-muted)]">
          CF DNS 记录名
          <input v-model="form.cloud_record_name" placeholder="transitnode (不带域名后缀)" :disabled="form.cloud_provider === 'none'" />
        </label>
        
        <!-- Target Exit -->
        <div class="md:col-span-2 text-primary-400 font-bold mt-4 border-t border-white/5 pt-4">目标落地机 (转发目的地)</div>
        
        <select class="md:col-span-2" v-model.number="form.target_exit_id">
          <option :value="0">不绑定 (所有用户将无法连接)</option>
          <option v-for="ex in exits" :key="ex.id" :value="ex.id">{{ ex.name }} — 发往此机器</option>
        </select>
      </div>
      
      <div class="flex gap-4 mt-8">
        <button @click="$emit('close')" class="flex-1 p-4 bg-[var(--bg-secondary)] rounded-2xl hover:bg-white/5 transition">取消</button>
        <button
          @click="handleSubmit"
          :disabled="saving"
          class="flex-1 p-4 bg-primary-600 rounded-2xl font-bold disabled:opacity-50 hover:bg-primary-500 transition shadow-lg shadow-primary-500/20"
        >
          {{ saving ? '保存中...' : '提交节点' }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.custom-scrollbar::-webkit-scrollbar {
  width: 4px;
}
.custom-scrollbar::-webkit-scrollbar-track {
  background: rgba(255, 255, 255, 0.05);
}
.custom-scrollbar::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.1);
  border-radius: 10px;
}
</style>
