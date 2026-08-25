window.App = window.App || {};

;(function () {
const { reactive } = Vue

function initials(name) {
  return (name || '')
    .split(' ')
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0].toUpperCase())
    .join('')
}

const state = reactive({ user: null, checking: true })

// The backend is the source of truth for the user's saved language once one exists (see
// login.UseCase in the Go backend) — apply whatever it reports so the UI matches it, even if that
// differs from what's currently in localStorage.
function applyUserLanguage(user) {
  const language = user?.settings?.language
  if (language) App.setLocale(language)
}

// Same idea as applyUserLanguage, for the saved UI theme.
function applyUserTheme(user) {
  const theme = user?.settings?.theme
  if (theme) App.setTheme(theme)
}

async function restoreSession() {
  try {
    state.user = await App.api.me()
    applyUserLanguage(state.user)
    applyUserTheme(state.user)
  } catch {
    state.user = null
  } finally {
    state.checking = false
  }
}

async function login(username, password) {
  const user = await App.api.login({ username, password, language: App.i18nStore.locale })
  state.user = user
  applyUserLanguage(user)
  applyUserTheme(user)
  return user
}

async function logout() {
  await App.api.logout()
  state.user = null
}

async function changeCredentials(currentPassword, newUsername, newPassword) {
  const user = await App.api.changeCredentials({ currentPassword, newUsername, newPassword })
  state.user = user
  return user
}

// Changing the language in-app persists it to the backend first, then applies whatever it echoes
// back — the same round-trip login/restoreSession use, rather than optimistically switching the UI
// before the save is confirmed. UpdateSettings overwrites the whole settings blob (see the Go
// backend), so the current theme rides along unchanged.
async function changeLanguage(language) {
  const settings = await App.api.updateSettings({ language, theme: App.themeStore.theme })
  if (state.user) state.user = { ...state.user, settings }
  applyUserLanguage({ settings })
}

// Same round-trip as changeLanguage, for the theme.
async function changeTheme(theme) {
  const settings = await App.api.updateSettings({ language: App.i18nStore.locale, theme })
  if (state.user) state.user = { ...state.user, settings }
  applyUserTheme({ settings })
}

App.authStore = {
  state,
  get isAuthenticated() {
    return !!state.user
  },
  initials,
  login,
  logout,
  changeCredentials,
  changeLanguage,
  changeTheme,
  restoreSession,
}

// Fire-and-forget, same as js/data/i18n.js initializing its locale at module-load time — no need to
// gate this behind a Vue lifecycle hook.
App.authStore.restoreSession()
})();
