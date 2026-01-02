<script setup>
import { ref, watch } from 'vue'
import { useApi } from '../../composables/useApi'

const emit = defineEmits(['close', 'saved'])

const { apiGet, apiPost } = useApi()

const platform = ref('ec2') // 'ec2' or 'lightsail'
const tab = ref('create') // 'create' or 'destroy'
const loading = ref(false)
const regionsLoading = ref(false)
const imagesLoading = ref(false)

const provisionResult = ref(null)

// EC2
const ec2Regions = ref([])
const ec2Images = ref([])
const ec2Form = ref({
  region: '',
  instance_type: 't3.micro',
  image_id: '',
  root_password: ''
})

// Lightsail
const lsRegions = ref([])
const lsBundles = ref([])
const lsBlueprints = ref([])
const lsForm = ref({
  region: '',
  bundle_id: '',
  blueprint_id: '',
  root_password: ''
})

// Destroy
const destroyId = ref('')

// Fetch regions on platform change
watch(platform, () => {
  fetchRegions()
}, { immediate: true })

async function fetchRegions() {
  regionsLoading.value = true
  try {
    if (platform.value === 'ec2') {
      const res = await apiGet('/api/v1/cloud/regions')
      ec2Regions.value = res || []
      if (ec2Regions.value.includes('ap-northeast-1')) {
        ec2Form.value.region = 'ap-northeast-1'
        fetchImages()
      }
    } else {
      const res = await apiGet('/api/v1/cloud/lightsail/regions')
      lsRegions.value = res || []
      if (lsRegions.value.includes('ap-northeast-1')) {
        lsForm.value.region = 'ap-northeast-1'
        fetchBundlesAndBlueprints()
      }
    }
  } catch (e) {
    console.error(e)
  } finally {
    regionsLoading.value = false
  }
}

async function fetchImages() {
  if (!ec2Form.value.region) return
  imagesLoading.value = true
  try {
    const res = await apiGet(`/api/v1/cloud/images?region=${ec2Form.value.region}`)
    ec2Images.value = res || []
    const debian = ec2Images.value.find(i => i.name.includes('Debian 12'))
    if (debian) ec2Form.value.image_id = debian.id
    else if (ec2Images.value.length) ec2Form.value.image_id = ec2Images.value[0].id
  } catch (e) {
    console.error(e)
  } finally {
    imagesLoading.value = false
  }
}

async function fetchBundlesAndBlueprints() {
  if (!lsForm.value.region) return
  imagesLoading.value = true
  try {
    const [bun, blu] = await Promise.all([
      apiGet(`/api/v1/cloud/lightsail/bundles?region=${lsForm.value.region}`),
      apiGet(`/api/v1/cloud/lightsail/blueprints?region=${lsForm.value.region}`)
    ])
    lsBundles.value = bun || []
    lsBlueprints.value = blu || []
    if (lsBundles.value.length) lsForm.value.bundle_id = lsBundles.value[0].id
    const deb = lsBlueprints.value.find(b => b.name.toLowerCase().includes('debian 12'))
    if (deb) lsForm.value.blueprint_id = deb.id
    else if (lsBlueprints.value.length) lsForm.value.blueprint_id = lsBlueprints.value[0].id
  } catch (e) {
    console.error(e)
  } finally {
    imagesLoading.value = false
  }
}

async function handleProvision() {
  loading.value = true
  provisionResult.value = null
  try {
    if (platform.value === 'ec2') {
      if (!ec2Form.value.region || !ec2Form.value.image_id || !ec2Form.value.root_password) {
        alert('请填写所有必填项')
        return
      }
      const res = await apiPost('/api/v1/cloud/instances', ec2Form.value)
      provisionResult.value = { ...res, type: 'ec2', region: ec2Form.value.region }
    } else {
      if (!lsForm.value.region || !lsForm.value.bundle_id || !lsForm.value.blueprint_id || !lsForm.value.root_password) {
        alert('请填写所有必填项')
        return
      }
      const res = await apiPost('/api/v1/cloud/lightsail/instances', lsForm.value)
      provisionResult.value = { ...res, type: 'lightsail', region: lsForm.value.region }
    }
  } catch (e) {
    alert('创建失败: ' + e.message)
  } finally {
    loading.value = false
  }
}

async function handleTerminate() {
  if (!destroyId.value) return
  if (!confirm(`⚠️ 危险警告 ⚠️\n即将永久销毁: ${destroyId.value}\n此操作不可逆!`)) return
  
  loading.value = true
  try {
    const region = platform.value === 'ec2' ? ec2Form.value.region : lsForm.value.region
    if (platform.value === 'ec2') {
      await apiPost('/api/v1/cloud/instances/terminate', { region, instance_id: destroyId.value })
    } else {
      await apiPost('/api/v1/cloud/lightsail/terminate', { region, instance_name: destroyId.value })
    }
    alert('销毁指令已发送')
    emit('saved')
  } catch (e) {
    alert('销毁失败: ' + e.message)
  } finally {
    loading.value = false
  }
}

function downloadKey() {
  const token = localStorage.getItem('stealth_token')
  const name = `StealthKey_${provisionResult.value.region}.pem`
  window.open(`/api/v1/cloud/keys/${name}?token=${token}`, '_blank')
}
</script>

<template>
  <div class="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4 scrollbar-hide" @click.self="$emit('close')">
    <div class="glass w-full max-w-xl p-8 rounded-3xl animate-slide-up max-h-[90vh] overflow-y-auto custom-scrollbar">
      
      <div v-if="provisionResult" class="text-center py-6 animate-fade-in">
        <div class="w-20 h-20 bg-emerald-500/20 text-emerald-500 rounded-full flex items-center justify-center mx-auto mb-6">
          <svg class="w-10 h-10" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" />
          </svg>
        </div>
        <h3 class="text-2xl font-bold mb-2">开通成功！</h3>
        <p class="text-[var(--text-muted)] mb-8">机器已启动，请保存好以下信息</p>
        
        <div class="space-y-4 text-left bg-black/20 p-6 rounded-2xl mb-8 border border-white/5 font-mono">
          <div class="flex justify-between items-center border-b border-white/5 pb-2">
            <span class="text-xs text-[var(--text-muted)]">公网 IP</span>
            <span class="text-emerald-400 font-bold">{{ provisionResult.public_ip }}</span>
          </div>
          <div class="flex justify-between items-center border-b border-white/5 pb-2">
            <span class="text-xs text-[var(--text-muted)]">实例 ID</span>
            <span class="text-primary-400">{{ provisionResult.instance_id || provisionResult.instance_name }}</span>
          </div>
          <div class="flex justify-between items-center pt-2">
            <span class="text-xs text-[var(--text-muted)]">SSH 登录名</span>
            <span class="text-white">root</span>
          </div>
        </div>

        <div class="flex flex-col gap-3">
          <button 
            v-if="provisionResult.type === 'ec2'"
            @click="downloadKey" 
            class="w-full p-4 bg-amber-500 hover:bg-amber-400 text-white rounded-2xl font-bold transition flex items-center justify-center gap-2"
          >
            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a2 2 0 002 2h12a2 2 0 002-2v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
            下载 SSH 密钥 (.pem)
          </button>
          <button @click="$emit('saved')" class="w-full p-4 bg-[var(--bg-secondary)] hover:bg-white/5 rounded-2xl font-bold transition">返回列表</button>
        </div>
      </div>

      <template v-else>
        <h3 class="text-2xl font-bold mb-6">云端节点管理</h3>
        
        <!-- Platform Toggle -->
        <div class="flex justify-center mb-6">
          <div class="bg-[var(--bg-secondary)] p-1 rounded-2xl flex gap-1 border border-white/5">
            <button
              @click="platform = 'ec2'"
              :class="['px-6 py-2 rounded-xl transition font-bold text-sm', platform === 'ec2' ? 'bg-amber-500 text-white shadow-lg shadow-amber-500/20' : 'text-[var(--text-muted)]']"
            >
              AWS EC2
            </button>
            <button
              @click="platform = 'lightsail'"
              :class="['px-6 py-2 rounded-xl transition font-bold text-sm', platform === 'lightsail' ? 'bg-purple-500 text-white shadow-lg shadow-purple-500/20' : 'text-[var(--text-muted)]']"
            >
              Lightsail
            </button>
          </div>
        </div>

        <!-- Tab Toggle -->
        <div class="flex justify-center mb-6">
          <div class="flex gap-2 text-sm bg-white/5 p-1 rounded-xl">
            <button
              @click="tab = 'create'"
              :class="['px-4 py-2 rounded-lg transition font-medium', tab === 'create' ? 'bg-primary-500 text-white' : 'text-[var(--text-muted)]']"
            >
              ✨ 开通新机
            </button>
            <button
              @click="tab = 'destroy'"
              :class="['px-4 py-2 rounded-lg transition font-medium', tab === 'destroy' ? 'bg-rose-500 text-white' : 'text-[var(--text-muted)]']"
            >
              💀 销毁管理
            </button>
          </div>
        </div>

        <!-- Create Tab -->
        <div v-if="tab === 'create'" class="space-y-4 text-sm">
          <!-- EC2 Form -->
          <template v-if="platform === 'ec2'">
            <label class="flex flex-col gap-1.5 text-[var(--text-muted)]">
              区域 (Region)
              <select v-model="ec2Form.region" @change="fetchImages" :disabled="regionsLoading">
                <option disabled value="">{{ regionsLoading ? 'Loading...' : 'Select Region' }}</option>
                <option v-for="r in ec2Regions" :key="r" :value="r">{{ r }}</option>
              </select>
            </label>
            <label class="flex flex-col gap-1.5 text-[var(--text-muted)]">
              操作系统
              <select v-model="ec2Form.image_id" :disabled="imagesLoading">
                <option disabled value="">{{ imagesLoading ? 'Loading...' : 'Select OS' }}</option>
                <option v-for="img in ec2Images" :key="img.id" :value="img.id">{{ img.name }}</option>
              </select>
            </label>
          </template>

          <!-- Lightsail Form -->
          <template v-else>
            <label class="flex flex-col gap-1.5 text-[var(--text-muted)]">
              区域 (Region)
              <select v-model="lsForm.region" @change="fetchBundlesAndBlueprints" :disabled="regionsLoading">
                <option disabled value="">{{ regionsLoading ? 'Loading...' : 'Select Region' }}</option>
                <option v-for="r in lsRegions" :key="r" :value="r">{{ r }}</option>
              </select>
            </label>
            <label class="flex flex-col gap-1.5 text-[var(--text-muted)]">
              套餐 (Bundle)
              <select v-model="lsForm.bundle_id" :disabled="imagesLoading">
                <option disabled value="">{{ imagesLoading ? 'Loading...' : 'Select Bundle' }}</option>
                <option v-for="b in lsBundles" :key="b.id" :value="b.id">{{ b.name }}</option>
              </select>
            </label>
            <label class="flex flex-col gap-1.5 text-[var(--text-muted)]">
              操作系统 (Blueprint)
              <select v-model="lsForm.blueprint_id" :disabled="imagesLoading">
                <option disabled value="">{{ imagesLoading ? 'Loading...' : 'Select OS' }}</option>
                <option v-for="bp in lsBlueprints" :key="bp.id" :value="bp.id">{{ bp.name }}</option>
              </select>
            </label>
          </template>

          <!-- Password -->
          <div class="mt-4 p-4 rounded-3xl border transition-colors duration-500" :class="platform === 'ec2' ? 'bg-amber-500/10 border-amber-500/20' : 'bg-purple-500/10 border-purple-500/20'">
            <label class="flex flex-col gap-1.5 font-bold" :class="platform === 'ec2' ? 'text-amber-500' : 'text-purple-500'">
              Root 密码 (SSH)
              <input
                v-if="platform === 'ec2'"
                v-model="ec2Form.root_password"
                type="text"
                placeholder="密码 (用于 SSH 密码登录)"
                class="font-mono text-center mt-2"
              />
              <input
                v-else
                v-model="lsForm.root_password"
                type="text"
                placeholder="密码 (用于 SSH 密码登录)"
                class="font-mono text-center mt-2"
              />
            </label>
            <p class="text-xs opacity-70 mt-3 text-center leading-relaxed">
              {{ platform === 'ec2' ? '* 本地会保存一份备用 SSH 密钥 (.pem)' : '* Lightsail 自动附加公网 IP 并放行全部安全组端口' }}
            </p>
          </div>

          <div class="flex gap-4 mt-8">
            <button @click="$emit('close')" class="flex-1 p-4 bg-[var(--bg-secondary)] hover:bg-white/5 transition rounded-2xl">取消</button>
            <button
              @click="handleProvision"
              :disabled="loading"
              :class="['flex-1 p-4 text-white rounded-2xl font-bold disabled:opacity-50 transition transform active:scale-95', platform === 'ec2' ? 'bg-amber-500 hover:bg-amber-400 shadow-lg shadow-amber-500/20' : 'bg-purple-600 hover:bg-purple-500 shadow-lg shadow-purple-500/20']"
            >
              {{ loading ? '购买中...' : '立即开通' }}
            </button>
          </div>
        </div>

        <!-- Destroy Tab -->
        <div v-else class="space-y-4">
          <label class="flex flex-col gap-1.5 text-[var(--text-muted)]">
            实例 ID / 名称
            <input v-model="destroyId" placeholder="i-0123... 或 stealth-node-1" class="font-mono" />
          </label>
          
          <div class="flex gap-4 mt-8">
            <button @click="$emit('close')" class="flex-1 p-4 bg-[var(--bg-secondary)] rounded-2xl">取消</button>
            <button
              @click="handleTerminate"
              :disabled="loading"
              class="flex-1 p-4 bg-rose-600 hover:bg-rose-500 text-white rounded-2xl font-bold disabled:opacity-50 transition shadow-lg shadow-rose-500/20"
            >
              {{ loading ? '销毁中...' : '确认销毁' }}
            </button>
          </div>
        </div>
      </template>

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
