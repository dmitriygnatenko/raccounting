window.App = window.App || {};

App.LoginView = {
  template: `
    <div class="min-h-screen flex items-center justify-center bg-ink-50 px-4">
      <div class="w-full max-w-sm">
        <p class="text-center mb-4">
          <a href="https://raccounting.ru" target="_blank" rel="noopener" class="text-xl font-semibold text-ink-950 hover:text-brand-600 transition-colors">Raccounting</a>
        </p>
        <div class="rounded-xl bg-white border border-ink-200 shadow-sm p-5 md:p-6">
          <h1 class="text-lg font-semibold text-ink-950 mb-1">{{ App.t('Вход в аккаунт') }}</h1>
          <p class="text-sm text-ink-400 mb-5">{{ App.t('Введите имя пользователя и пароль, чтобы продолжить') }}</p>
          <form class="space-y-4" @submit.prevent="submit">
            <p v-if="error" class="text-sm text-money-neg">{{ error }}</p>
            <div>
              <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Имя пользователя') }}</label>
              <input v-model="username" type="text" required autocomplete="username" :placeholder="App.t('Например, Иван Иванов')"
                class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
            </div>
            <div>
              <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Пароль') }}</label>
              <input v-model="password" type="password" required autocomplete="current-password" placeholder="••••••••"
                class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
            </div>
            <button type="submit" :disabled="loading"
              class="w-full rounded-lg bg-brand-600 text-white text-sm font-medium py-2.5 hover:bg-brand-500 transition-colors cursor-pointer disabled:opacity-50">
              {{ loading ? App.t('Вход…') : App.t('Войти') }}
            </button>
          </form>
        </div>
      </div>
    </div>
  `,
  data() {
    return {
      App,
      username: '',
      password: '',
      loading: false,
      error: '',
    }
  },
  methods: {
    async submit() {
      this.error = ''
      this.loading = true
      try {
        await App.authStore.login(this.username, this.password)
        App.router.push('/')
      } catch {
        this.error = App.t('Не удалось войти. Проверьте имя пользователя и пароль.')
      } finally {
        this.loading = false
      }
    },
  },
}
