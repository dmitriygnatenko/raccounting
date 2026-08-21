window.App = window.App || {};

;(function () {
const { reactive, computed } = Vue

const SEEN_KEY = 'raccounting.seenBudgetAlerts'

function alertKey(a) {
  return `${a.categoryId}:${a.monthKey}`
}

function loadSeen() {
  try {
    return new Set(JSON.parse(localStorage.getItem(SEEN_KEY) || '[]'))
  } catch {
    return new Set()
  }
}

function saveSeen(seen) {
  try {
    localStorage.setItem(SEEN_KEY, JSON.stringify([...seen]))
  } catch {}
}

function createNotificationsStore() {
  const finance = App.financeStore
  // reactive(): unseenAlerts below reads seen.has(...) inside a computed, and Vue only tracks that
  // as a dependency — re-running the computed when an alert is marked seen — if the Set itself is
  // reactive. A plain Set's mutations are invisible to Vue's reactivity system.
  const seen = reactive(loadSeen())

  const state = reactive({
    panelOpen: false,
    // A snapshot of what was unseen at the moment the panel opened. toggle() marks those alerts
    // seen immediately (so the badge clears right away), but keeps showing this snapshot rather
    // than the now-empty unseenAlerts list while the panel stays open.
    panelAlerts: [],
  })

  // Budget-over alerts the user hasn't acknowledged yet — drives the bell's blinking badge. Each
  // alert is keyed by category + month, so once shown it stays dismissed even across reloads,
  // unless that category goes over budget again in a different month.
  const unseenAlerts = computed(() => finance.state.budgetAlerts.filter((a) => !seen.has(alertKey(a))))

  function toggle() {
    if (state.panelOpen) {
      state.panelOpen = false
      return
    }
    state.panelAlerts = unseenAlerts.value.slice()
    for (const a of state.panelAlerts) seen.add(alertKey(a))
    saveSeen(seen)
    state.panelOpen = true
  }

  function close() {
    state.panelOpen = false
  }

  return { state, unseenAlerts, toggle, close }
}

App.notificationsStore = createNotificationsStore()
})();
