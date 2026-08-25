window.App = window.App || {};

;(function () {
const { reactive } = Vue

const SUPPORTED_THEMES = ['light', 'dark']
const STORAGE_KEY = 'raccounting.theme'
const DEFAULT_THEME = 'light'

function getStoredTheme() {
  try {
    const v = localStorage.getItem(STORAGE_KEY)
    return SUPPORTED_THEMES.includes(v) ? v : null
  } catch {
    return null
  }
}

function setStoredTheme(theme) {
  try {
    localStorage.setItem(STORAGE_KEY, theme)
  } catch {}
}

const themeStore = reactive({ theme: getStoredTheme() || DEFAULT_THEME })

function applyThemeClass(theme) {
  document.documentElement.classList.toggle('dark', theme === 'dark')
}

// setTheme is also how a signed-in user's saved theme gets applied on load (see
// App.authStore's applyUserTheme) — it always writes through to localStorage so the choice
// sticks even before the backend round-trip confirms it.
function setTheme(theme) {
  if (!SUPPORTED_THEMES.includes(theme)) return
  themeStore.theme = theme
  setStoredTheme(theme)
  applyThemeClass(theme)
}

applyThemeClass(themeStore.theme)

App.themeStore = themeStore
App.SUPPORTED_THEMES = SUPPORTED_THEMES
App.THEME_LABELS = { light: 'Светлая', dark: 'Тёмная' }
App.setTheme = setTheme
})();
