window.App = window.App || {};

;(function () {
// Minimal hash-based router — no build step, no server rewrite rules needed,
// works straight off the filesystem (file://) as well as any static host.
const { reactive } = Vue

function parseHash() {
  const raw = window.location.hash.replace(/^#/, '') || '/'
  const [path, queryStr] = raw.split('?')
  const query = Object.fromEntries(new URLSearchParams(queryStr ?? ''))
  return { path: path || '/', query }
}

const routes = {
  '/': { title: 'Обзор', view: 'dashboard-view' },
  '/accounts': { title: 'Счета', view: 'accounts-view' },
  '/transactions': { title: 'Операции', view: 'transactions-view' },
  '/reports': { title: 'Отчёты', view: 'reports-view' },
  '/budget': { title: 'Бюджет', view: 'budget-view' },
  '/settings': { title: 'Настройки', view: 'settings-view' },
}

const routerState = reactive(parseHash())

window.addEventListener('hashchange', () => {
  const next = parseHash()
  routerState.path = next.path
  routerState.query = next.query
})

App.router = {
  state: routerState,
  routes,
  get current() {
    return routes[routerState.path] ?? routes['/']
  },
  push(path) {
    window.location.hash = path
  },
}
})();
