;(function () {
  const app = Vue.createApp({
    components: { 'app-shell': App.AppShell, 'login-view': App.LoginView },
    template: `
      <div v-if="authStore.state.checking" class="min-h-screen flex items-center justify-center bg-ink-50">
        <span class="text-xl font-semibold tracking-tight text-ink-300">Raccounting</span>
      </div>
      <login-view v-else-if="!authStore.isAuthenticated"></login-view>
      <app-shell v-else></app-shell>
    `,
    data() {
      return { authStore: App.authStore }
    },
    mounted() {
      if (App.authStore.isAuthenticated) App.financeStore.load()
    },
    watch: {
      'authStore.state.user'(user) {
        if (user) App.financeStore.load()
      },
    },
  })

  app.component('dashboard-view', App.DashboardView)
  app.component('accounts-view', App.AccountsView)
  app.component('transactions-view', App.TransactionsView)
  app.component('reports-view', App.ReportsView)
  app.component('settings-view', App.SettingsView)

  app.mount('#app')
})()
