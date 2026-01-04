import { ref, readonly } from 'vue'
import { useApi } from './useApi'

const licenseInfo = ref(null)
const loading = ref(false)
const error = ref(null)

export function useLicense() {
    const { apiGet } = useApi()

    async function fetchLicenseInfo() {
        loading.value = true
        error.value = null
        try {
            const res = await apiGet('/api/v1/license/info')
            licenseInfo.value = res
            return res
        } catch (e) {
            error.value = e.message
            // 安全回退：获取失败默认为 Basic，防止权限泄露
            licenseInfo.value = {
                level: 'basic',
                expires_at: '-',
                limits: {
                    protocols: ['anytls'],
                    max_entries: 5,
                    max_exits: 5,
                    cloud_enabled: false
                },
                usage: { entries: 0, exits: 0 }
            }
            return licenseInfo.value
        } finally {
            loading.value = false
        }
    }

    function isProtocolAllowed(protocol) {
        if (!licenseInfo.value) return protocol.toLowerCase() === 'anytls'
        const protocols = licenseInfo.value.limits?.protocols || []
        return protocols.includes('*') || protocols.includes(protocol.toLowerCase())
    }

    function isCloudEnabled() {
        if (!licenseInfo.value) return false
        return licenseInfo.value.limits?.cloud_enabled ?? false
    }

    function canAddEntry() {
        if (!licenseInfo.value) return false
        const max = licenseInfo.value.limits?.max_entries || 0
        const current = licenseInfo.value.usage?.entries || 0
        return current < max
    }

    function canAddExit() {
        if (!licenseInfo.value) return false
        const max = licenseInfo.value.limits?.max_exits || 0
        const current = licenseInfo.value.usage?.exits || 0
        return current < max
    }

    function getLevel() {
        return licenseInfo.value?.level || 'basic'
    }

    function isAdmin() {
        const level = getLevel()
        return level === 'admin' || level === 'super_admin'
    }

    function isPro() {
        const level = getLevel()
        return level === 'pro' || level === 'admin' || level === 'super_admin'
    }

    return {
        licenseInfo: readonly(licenseInfo),
        loading: readonly(loading),
        error: readonly(error),
        fetchLicenseInfo,
        isProtocolAllowed,
        isCloudEnabled,
        canAddEntry,
        canAddExit,
        getLevel,
        isAdmin,
        isPro
    }
}
