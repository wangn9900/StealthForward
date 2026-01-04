import { ref, readonly } from 'vue'
import { useApi } from './useApi'

const licenseInfo = ref(null)
const loading = ref(false)
const error = ref(null)

export function useLicense() {
    const { apiGet } = useApi()

    async function fetchLicenseInfo() {
        if (licenseInfo.value) return licenseInfo.value

        loading.value = true
        error.value = null
        try {
            const res = await apiGet('/api/v1/license/info')
            licenseInfo.value = res
            return res
        } catch (e) {
            error.value = e.message
            // 如果获取失败，假设是admin模式（开发环境）
            licenseInfo.value = {
                level: 'admin',
                expires_at: '永久',
                limits: {
                    protocols: ['*'],
                    max_entries: 999999,
                    max_exits: 999999,
                    cloud_enabled: true
                },
                usage: { entries: 0, exits: 0 }
            }
            return licenseInfo.value
        } finally {
            loading.value = false
        }
    }

    function isProtocolAllowed(protocol) {
        if (!licenseInfo.value) return true // 未加载时允许
        const protocols = licenseInfo.value.limits?.protocols || []
        return protocols.includes('*') || protocols.includes(protocol.toLowerCase())
    }

    function isCloudEnabled() {
        if (!licenseInfo.value) return true
        return licenseInfo.value.limits?.cloud_enabled ?? true
    }

    function canAddEntry() {
        if (!licenseInfo.value) return true
        const max = licenseInfo.value.limits?.max_entries || 999999
        const current = licenseInfo.value.usage?.entries || 0
        return current < max
    }

    function canAddExit() {
        if (!licenseInfo.value) return true
        const max = licenseInfo.value.limits?.max_exits || 999999
        const current = licenseInfo.value.usage?.exits || 0
        return current < max
    }

    function getLevel() {
        return licenseInfo.value?.level || 'admin'
    }

    function isAdmin() {
        return getLevel() === 'admin'
    }

    function isPro() {
        const level = getLevel()
        return level === 'pro' || level === 'admin'
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
