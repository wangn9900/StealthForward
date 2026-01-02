import { ref, onMounted } from 'vue'

export function useTheme() {
    const isDark = ref(true) // Default to dark

    onMounted(() => {
        // Check saved preference
        const saved = localStorage.getItem('theme')
        if (saved) {
            isDark.value = saved === 'dark'
        } else {
            // Check system preference
            isDark.value = window.matchMedia('(prefers-color-scheme: dark)').matches
        }
        applyTheme()
    })

    function applyTheme() {
        if (isDark.value) {
            document.documentElement.classList.add('dark')
        } else {
            document.documentElement.classList.remove('dark')
        }
    }

    function toggleTheme() {
        isDark.value = !isDark.value
        localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
        applyTheme()
    }

    return {
        isDark,
        toggleTheme
    }
}
