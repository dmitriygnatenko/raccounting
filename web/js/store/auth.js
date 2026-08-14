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

async function restoreSession() {
  try {
    state.user = await App.api.me()
  } catch {
    state.user = null
  } finally {
    state.checking = false
  }
}

async function login(username, password) {
  const user = await App.api.login({ username, password })
  state.user = user
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

App.authStore = {
  state,
  get isAuthenticated() {
    return !!state.user
  },
  initials,
  login,
  logout,
  changeCredentials,
  restoreSession,
}

// Fire-and-forget, same as js/data/i18n.js initializing its locale at module-load time — no need to
// gate this behind a Vue lifecycle hook.
App.authStore.restoreSession()
})();
