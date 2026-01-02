export function useApi() {
    function getToken() {
        return localStorage.getItem('stealth_token') || ''
    }

    async function apiGet(url) {
        const res = await fetch(url, {
            headers: { 'Authorization': getToken() }
        })

        if (res.status === 401) {
            throw new Error('401 Unauthorized')
        }

        if (!res.ok) {
            const err = await res.json().catch(() => ({}))
            throw new Error(err.error || res.statusText)
        }

        return res.json()
    }

    async function apiPost(url, data) {
        const res = await fetch(url, {
            method: 'POST',
            body: JSON.stringify(data),
            headers: {
                'Content-Type': 'application/json',
                'Authorization': getToken()
            }
        })

        if (res.status === 401) {
            throw new Error('401 Unauthorized')
        }

        if (!res.ok) {
            const err = await res.json().catch(() => ({}))
            throw new Error(err.error || res.statusText)
        }

        return res.json()
    }

    async function apiDelete(url) {
        const res = await fetch(url, {
            method: 'DELETE',
            headers: { 'Authorization': getToken() }
        })

        if (!res.ok) {
            const err = await res.json().catch(() => ({}))
            throw new Error(err.error || res.statusText)
        }

        return true
    }

    return {
        apiGet,
        apiPost,
        apiDelete
    }
}
