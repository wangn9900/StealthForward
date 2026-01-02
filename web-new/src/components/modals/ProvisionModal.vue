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
  try {
    if (platform.value === 'ec2') {
      if (!ec2Form.value.region || !ec2Form.value.image_id || !ec2Form.value.root_password) {
        alert('请填写所有必填项')
        return
      }
      const res = await apiPost('/api/v1/cloud/instances', ec2Form.value)
      alert(`EC2 实例创建成功!\nID: ${res.instance_id}\nIP: ${res.public_ip}`)
    } else {
      if (!lsForm.value.region || !lsForm.value.bundle_id || !lsForm.value.blueprint_id || !lsForm.value.root_password) {
        alert('请填写所有必填项')
        return
      }
      const res = await apiPost('/api/v1/cloud/lightsail/instances', lsForm.value)
      alert(`Lightsail 实例创建成功!\nName: ${res.instance_name}\nIP: ${res.public_ip}`)
    }
    emit('saved')
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
</script>

<template>
  <div class="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4" @click.self="$emit('close')">
    <div class="glass w-full max-w-xl p-8 rounded-3xl animate-slide-up">
      <h3 class="text-2xl font-bold mb-6">云端节点管理</h3>
      
      <!-- Platform Toggle -->
      <div class="flex justify-center mb-6">
        <div class="bg-[var(--bg-secondary)] p-1 rounded-2xl flex gap-1">
          <button
            @click="platform = 'ec2'"
            :class="['px-6 py-2 rounded-xl transition', platform === 'ec2' ? 'bg-amber-500 text-white' : '']"
          >
            AWS EC2
          </button>
          <button
            @click="platform = 'lightsail'"
            :class="['px-6 py-2 rounded-xl transition', platform === 'lightsail' ? 'bg-purple-500 text-white' : '']"
          >
            Lightsail
          </button>
        </div>
      </div>

      <!-- Tab Toggle -->
      <div class="flex justify-center mb-6">
        <div class="flex gap-2 text-sm">
          <button
            @click="tab = 'create'"
            :class="['px-4 py-2 rounded-lg transition', tab === 'create' ? 'bg-primary-500/20 text-primary-400' : 'text-[var(--text-muted)]']"
          >
            ✨ 开通新机
          </button>
          <button
            @click="tab = 'destroy'"
            :class="['px-4 py-2 rounded-lg transition', tab === 'destroy' ? 'bg-rose-500/20 text-rose-400' : 'text-[var(--text-muted)]']"
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
        <div class="mt-4 p-4 rounded-xl border" :class="platform === 'ec2' ? 'bg-amber-500/10 border-amber-500/20' : 'bg-purple-500/10 border-purple-500/20'">
          <label class="flex flex-col gap-1.5 font-bold" :class="platform === 'ec2' ? 'text-amber-500' : 'text-purple-500'">
            Root 密码 (SSH)
            <input
              v-if="platform === 'ec2'"
              v-model="ec2Form.root_password"
              type="text"
              placeholder="Stealth123!@#"
              class="font-mono text-center"
            />
            <input
              v-else
              v-model="lsForm.root_password"
              type="text"
              placeholder="Stealth123!@#"
              class="font-mono text-center"
            />
          </label>
          <p class="text-xs opacity-70 mt-2 text-center">
            {{ platform === 'ec2' ? '* 本地会保存一份备用 SSH 密钥 (store/keys/)' : '* Lightsail 自动附加免费 Static IP 并放行全端口' }}
          </p>
        </div>

        <div class="flex gap-4 mt-6">
          <button @click="$emit('close')" class="flex-1 p-4 bg-[var(--bg-secondary)] rounded-2xl">取消</button>
          <button
            @click="handleProvision"
            :disabled="loading"
            :class="['flex-1 p-4 text-white rounded-2xl font-bold disabled:opacity-50', platform === 'ec2' ? 'bg-gradient-to-r from-amber-500 to-orange-600' : 'bg-gradient-to-r from-purple-500 to-indigo-600']"
          >
            {{ loading ? '购买中...' : '立即开通' }}
          </button>
        </div>
      </div>

      <!-- Destroy Tab -->
      <div v-else class="space-y-4">
        <label class="flex flex-col gap-1.5 text-[var(--text-muted)]">
          实例 ID / 名称
          <input v-model="destroyId" placeholder="i-0123456789abcdef0 或 stealth-xxx" class="font-mono" />
        </label>
        
        <div class="flex gap-4 mt-6">
          <button @click="$emit('close')" class="flex-1 p-4 bg-[var(--bg-secondary)] rounded-2xl">取消</button>
          <button
            @click="handleTerminate"
            :disabled="loading"
            class="flex-1 p-4 bg-rose-600 hover:bg-rose-500 text-white rounded-2xl font-bold disabled:opacity-50"
          >
            {{ loading ? '销毁中...' : '确认销毁' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
