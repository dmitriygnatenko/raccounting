window.App = window.App || {};

App.AppShell = {
  components: {
    'app-icon': App.AppIcon,
    'transaction-modal': App.TransactionModal,
    'account-modal': App.AccountModal,
    'currency-modal': App.CurrencyModal,
    'category-modal': App.CategoryModal,
    'tag-modal': App.TagModal,
  },
  template: `
    <div class="min-h-screen bg-ink-50 text-ink-900 md:flex">
      <aside class="hidden md:flex md:w-60 md:flex-col md:shrink-0 md:sticky md:top-0 md:h-screen border-r border-ink-200 bg-white">
        <div class="flex items-center gap-2 px-5 h-16 border-b border-ink-200">
          <span class="font-semibold text-ink-950 tracking-tight">Raccounting</span>
        </div>
        <nav class="flex-1 px-3 py-4 space-y-1">
          <a v-for="item in navItems" :key="item.to" :href="'#' + item.to"
            class="flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors"
            :class="path === item.to ? 'bg-brand-100 text-brand-600' : 'text-ink-500 hover:bg-ink-100 hover:text-ink-900'">
            <app-icon :name="item.icon" :size="18" />
            {{ App.t(item.label) }}
          </a>
        </nav>
        <div class="p-3 border-t border-ink-200">
          <button class="w-full flex items-center justify-center gap-2 rounded-lg bg-brand-600 text-white text-sm font-medium py-2.5 hover:bg-brand-500 transition-colors cursor-pointer"
            @click="ui.openNewTransaction()">
            <app-icon name="plus" :size="16" />
            {{ App.t('Новая операция') }}
          </button>
        </div>
      </aside>

      <div class="flex-1 min-w-0 flex flex-col">
        <header class="h-16 shrink-0 flex items-center justify-between px-4 md:px-8 border-b border-ink-200 bg-white/80 backdrop-blur sticky top-0 z-30">
          <div class="flex items-center gap-3">
            <button class="md:hidden -ml-1 p-2 rounded-lg text-ink-500 hover:bg-ink-100 cursor-pointer" :aria-label="App.t('Меню')" @click="mobileMenuOpen = true">
              <app-icon name="menu" :size="22" />
            </button>
            <h1 class="text-lg md:text-xl font-semibold text-ink-950">{{ App.t(pageTitle) }}</h1>
          </div>
          <div class="flex items-center gap-2">
            <div class="relative hidden sm:block">
              <button class="relative flex p-2 rounded-lg text-ink-500 hover:bg-ink-100 cursor-pointer" :aria-label="App.t('Уведомления')" @click="notifications.toggle()">
                <app-icon name="bell" :size="20" />
                <span v-if="hasUnseenAlerts" class="absolute top-1.5 right-1.5 flex h-2.5 w-2.5">
                  <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-red-400 opacity-75"></span>
                  <span class="relative inline-flex rounded-full h-2.5 w-2.5 bg-red-500"></span>
                </span>
              </button>
              <Transition name="fade">
                <div v-if="notifications.state.panelOpen" class="fixed inset-0 z-40" @click="notifications.close()"></div>
              </Transition>
              <Transition name="fade">
                <div v-if="notifications.state.panelOpen" class="absolute right-0 top-11 z-50 w-80 rounded-xl bg-white border border-ink-200 shadow-lg py-1.5">
                  <div class="px-3.5 py-2 text-sm font-semibold text-ink-950 border-b border-ink-200">{{ App.t('Уведомления') }}</div>
                  <div v-if="!notifications.state.panelAlerts.length" class="px-3.5 py-6 text-center text-sm text-ink-400">
                    {{ App.t('Новых уведомлений нет') }}
                  </div>
                  <ul v-else class="max-h-80 overflow-y-auto divide-y divide-ink-100">
                    <li v-for="a in notifications.state.panelAlerts" :key="a.categoryId + ':' + a.monthKey" class="px-3.5 py-2.5">
                      <div class="flex items-center gap-2 text-sm text-ink-900 font-medium">
                        <span class="w-2 h-2 rounded-full shrink-0" :style="{ background: a.category?.color ?? '#94a3b8' }"></span>
                        {{ App.t('Бюджет превышен: {name}', { name: App.t(a.category?.name ?? 'Без категории') }) }}
                      </div>
                      <p class="text-xs text-ink-500 mt-0.5">
                        {{ App.t('Потрачено {spent} из {budgeted} — превышение {over}', {
                          spent: App.formatMoney(a.spent, finance.state.baseCurrency),
                          budgeted: App.formatMoney(a.budgeted, finance.state.baseCurrency),
                          over: App.formatMoney(a.overBy, finance.state.baseCurrency),
                        }) }}
                      </p>
                    </li>
                  </ul>
                </div>
              </Transition>
            </div>
            <div class="relative">
              <button class="w-9 h-9 rounded-full bg-ink-200 flex items-center justify-center text-sm font-medium text-ink-700 cursor-pointer hover:bg-ink-300 transition-colors"
                :aria-label="App.t('Профиль')" @click="profileMenuOpen = !profileMenuOpen">
                {{ initials }}
              </button>
              <Transition name="fade">
                <div v-if="profileMenuOpen" class="fixed inset-0 z-40" @click="profileMenuOpen = false"></div>
              </Transition>
              <Transition name="fade">
                <div v-if="profileMenuOpen" class="absolute right-0 top-11 z-50 w-52 rounded-xl bg-white border border-ink-200 shadow-lg py-1.5">
                  <button class="w-full flex items-center gap-2 px-3 py-2 text-sm text-money-neg hover:bg-red-50 cursor-pointer" @click="logout">
                    <app-icon name="logout" :size="16" />
                    {{ App.t('Выход') }}
                  </button>
                </div>
              </Transition>
            </div>
          </div>
        </header>

        <main class="flex-1 px-4 md:px-8 py-5 md:py-6 pb-40 md:pb-6 max-w-[1400px] w-full mx-auto">
          <component :is="currentView" />
        </main>
      </div>

      <nav class="md:hidden fixed bottom-0 inset-x-0 z-30 bg-white border-t border-ink-200 flex items-stretch pb-[env(safe-area-inset-bottom)]">
        <a v-for="item in navItems" :key="item.to" :href="'#' + item.to"
          class="flex-1 flex flex-col items-center justify-center gap-0.5 py-2 text-xs font-medium"
          :class="path === item.to ? 'text-brand-600' : 'text-ink-400'">
          <app-icon :name="item.icon" :size="20" />
          {{ App.t(item.label) }}
        </a>
      </nav>

      <button class="md:hidden fixed right-4 bottom-20 z-30 w-14 h-14 rounded-full bg-brand-600 text-white shadow-lg shadow-brand-600/30 flex items-center justify-center cursor-pointer active:scale-95 transition-transform"
        :aria-label="App.t('Новая операция')" @click="ui.openNewTransaction()">
        <app-icon name="plus" :size="24" />
      </button>

      <Transition name="fade">
        <div v-if="mobileMenuOpen" class="md:hidden fixed inset-0 z-40 bg-ink-950/40" @click="mobileMenuOpen = false"></div>
      </Transition>
      <Transition name="slide">
        <aside v-if="mobileMenuOpen" class="md:hidden fixed inset-y-0 left-0 z-50 w-72 bg-white flex flex-col shadow-xl">
          <div class="flex items-center justify-between px-5 h-16 border-b border-ink-200">
            <div class="flex items-center gap-2">
              <span class="font-semibold text-ink-950 tracking-tight">Raccounting</span>
            </div>
            <button class="p-2 rounded-lg text-ink-500 hover:bg-ink-100 cursor-pointer" @click="mobileMenuOpen = false">
              <app-icon name="close" :size="20" />
            </button>
          </div>
          <nav class="flex-1 px-3 py-4 space-y-1">
            <a v-for="item in navItems" :key="item.to" :href="'#' + item.to"
              class="flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors"
              :class="path === item.to ? 'bg-brand-100 text-brand-600' : 'text-ink-500 hover:bg-ink-100 hover:text-ink-900'"
              @click="mobileMenuOpen = false">
              <app-icon :name="item.icon" :size="18" />
              {{ App.t(item.label) }}
            </a>
          </nav>
        </aside>
      </Transition>

      <transaction-modal></transaction-modal>
      <account-modal></account-modal>
      <currency-modal></currency-modal>
      <category-modal></category-modal>
      <tag-modal></tag-modal>
    </div>
  `,
  data() {
    return {
      App,
      ui: App.uiStore,
      finance: App.financeStore,
      notifications: App.notificationsStore,
      router: App.router,
      authStore: App.authStore,
      mobileMenuOpen: false,
      profileMenuOpen: false,
      navItems: [
        { to: '/', label: 'Обзор', icon: 'home' },
        { to: '/accounts', label: 'Счета', icon: 'wallet' },
        { to: '/transactions', label: 'Операции', icon: 'list' },
        { to: '/reports', label: 'Отчёты', icon: 'chart' },
        { to: '/budget', label: 'Бюджет', icon: 'target' },
        { to: '/settings', label: 'Настройки', icon: 'settings' },
      ],
    }
  },
  computed: {
    path() {
      return this.router.state.path
    },
    pageTitle() {
      return this.router.current.title
    },
    currentView() {
      return this.router.current.view
    },
    initials() {
      return App.authStore.initials(this.authStore.state.user?.username) || '?'
    },
    hasUnseenAlerts() {
      return this.notifications.unseenAlerts.length > 0
    },
  },
  methods: {
    async logout() {
      this.profileMenuOpen = false
      await this.authStore.logout()
      this.router.push('/')
    },
  },
}
