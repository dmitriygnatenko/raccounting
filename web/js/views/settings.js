window.App = window.App || {};

App.SettingsView = {
  components: { 'app-icon': App.AppIcon, 'skeleton-block': App.SkeletonBlock },
  template: `
    <div v-if="finance.state.loading" class="space-y-4">
      <skeleton-block class="h-12" />
      <skeleton-block class="h-64" />
    </div>

    <div v-else class="space-y-5">
      <div class="flex rounded-lg bg-ink-100 p-1 w-fit text-sm font-medium overflow-x-auto max-w-full">
        <button class="px-3.5 py-1.5 rounded-md transition-colors cursor-pointer shrink-0" :class="tab === 'categories' ? 'bg-white shadow-sm text-ink-900' : 'text-ink-500'" @click="tab = 'categories'">{{ App.t('Категории') }}</button>
        <button class="px-3.5 py-1.5 rounded-md transition-colors cursor-pointer shrink-0" :class="tab === 'accounts' ? 'bg-white shadow-sm text-ink-900' : 'text-ink-500'" @click="tab = 'accounts'">{{ App.t('Карты и счета') }}</button>
        <button class="px-3.5 py-1.5 rounded-md transition-colors cursor-pointer shrink-0" :class="tab === 'currencies' ? 'bg-white shadow-sm text-ink-900' : 'text-ink-500'" @click="tab = 'currencies'">{{ App.t('Валюты') }}</button>
        <button class="px-3.5 py-1.5 rounded-md transition-colors cursor-pointer shrink-0" :class="tab === 'language' ? 'bg-white shadow-sm text-ink-900' : 'text-ink-500'" @click="tab = 'language'">{{ App.t('Язык') }}</button>
        <button class="px-3.5 py-1.5 rounded-md transition-colors cursor-pointer shrink-0" :class="tab === 'account' ? 'bg-white shadow-sm text-ink-900' : 'text-ink-500'" @click="tab = 'account'">{{ App.t('Аккаунт') }}</button>
      </div>

      <div v-if="tab === 'accounts'" class="rounded-xl bg-white border border-ink-200 overflow-hidden">
        <div class="flex items-center justify-between px-4 md:px-5 py-3.5 border-b border-ink-200">
          <div>
            <h2 class="text-sm font-semibold text-ink-900">{{ App.t('Карты и счета') }}</h2>
            <p class="text-xs text-ink-400 mt-0.5">{{ finance.state.accounts.length }} {{ App.t('всего') }}</p>
          </div>
          <button class="flex items-center gap-1.5 rounded-lg bg-brand-600 text-white text-sm font-medium px-3.5 py-2 hover:bg-brand-500 transition-colors cursor-pointer"
            @click="App.uiStore.openNewAccount()">
            <app-icon name="plus" :size="16" />
            <span class="hidden sm:inline">{{ App.t('Добавить счёт') }}</span>
          </button>
        </div>
        <ul class="divide-y divide-ink-100">
          <li v-for="a in finance.state.accounts" :key="a.id" class="flex items-center justify-between gap-3 px-4 md:px-5 py-3.5">
            <span class="flex items-center gap-3 min-w-0">
              <span class="w-10 h-10 rounded-xl bg-brand-100 text-brand-600 flex items-center justify-center shrink-0" :class="{ 'opacity-40 grayscale': a.archived }">
                <app-icon :name="App.accountIcon[a.type]" :size="18" />
              </span>
              <span class="min-w-0">
                <span class="flex items-center gap-2">
                  <span class="block text-sm font-medium text-ink-900 truncate">{{ App.t(a.name) }}</span>
                  <span v-if="a.archived" class="shrink-0 text-[10px] font-medium uppercase tracking-wide text-ink-400 bg-ink-100 rounded px-1.5 py-0.5">{{ App.t('Деактивирован') }}</span>
                </span>
                <span class="block text-xs text-ink-400 truncate">{{ App.t(App.accountTypeLabel[a.type]) }} · {{ a.currency }}</span>
              </span>
            </span>
            <span class="flex items-center gap-1 shrink-0">
              <span class="text-sm font-medium text-ink-950 mr-1 hidden sm:inline">{{ App.formatMoney(a.balance, a.currency) }}</span>
              <button class="p-2 rounded-lg text-ink-500 hover:bg-ink-100 cursor-pointer" :aria-label="App.t('Изменить')" :title="App.t('Изменить данные счёта')" @click="App.uiStore.openEditAccount(a)">
                <app-icon name="edit" :size="16" />
              </button>
              <button v-if="!a.archived" class="p-2 rounded-lg text-ink-500 hover:bg-ink-100 cursor-pointer" :aria-label="App.t('Деактивировать')" :title="App.t('Скрыть из активных счетов, история операций сохранится')" @click="App.financeStore.archiveAccount(a.id)">
                <app-icon name="archive" :size="16" />
              </button>
              <button v-if="!finance.isAccountInUse(a.id)" class="p-2 rounded-lg text-money-neg hover:bg-red-50 cursor-pointer" :aria-label="App.t('Удалить')" :title="App.t('Удалить счёт безвозвратно')" @click="App.financeStore.deleteAccount(a.id)">
                <app-icon name="trash" :size="16" />
              </button>
            </span>
          </li>
        </ul>
      </div>

      <div v-if="tab === 'currencies'" class="rounded-xl bg-white border border-ink-200 p-4 md:p-5">
        <h2 class="text-sm font-semibold text-ink-900 mb-1">{{ App.t('Основная валюта') }}</h2>
        <p class="text-xs text-ink-400 mb-3">{{ App.t('В ней считаются баланс, отчёты и бюджет') }}</p>
        <select :value="finance.state.baseCurrency" @change="App.financeStore.setDefaultCurrency($event.target.value)"
          class="w-full sm:w-64 rounded-lg border border-ink-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500">
          <option v-for="c in activeCurrencies" :key="c.code" :value="c.code">{{ c.code }} — {{ App.t(c.name) }}</option>
        </select>
      </div>

      <div v-if="tab === 'currencies'" class="rounded-xl bg-white border border-ink-200 overflow-hidden">
        <div class="flex items-center justify-between px-4 md:px-5 py-3.5 border-b border-ink-200">
          <div>
            <h2 class="text-sm font-semibold text-ink-900">{{ App.t('Валюты') }}</h2>
            <p class="text-xs text-ink-400 mt-0.5">{{ finance.state.currencies.length }} {{ App.t('всего') }}</p>
          </div>
          <button class="flex items-center gap-1.5 rounded-lg bg-brand-600 text-white text-sm font-medium px-3.5 py-2 hover:bg-brand-500 transition-colors cursor-pointer"
            @click="App.uiStore.openNewCurrency()">
            <app-icon name="plus" :size="16" />
            <span class="hidden sm:inline">{{ App.t('Добавить валюту') }}</span>
          </button>
        </div>
        <ul class="divide-y divide-ink-100">
          <li v-for="c in finance.state.currencies" :key="c.code" class="flex items-center justify-between gap-3 px-4 md:px-5 py-3.5">
            <span class="flex items-center gap-3 min-w-0">
              <span class="w-10 h-10 rounded-xl bg-ink-100 text-ink-700 flex items-center justify-center shrink-0 font-semibold text-sm" :class="{ 'opacity-40 grayscale': c.archived }">{{ c.symbol }}</span>
              <span class="min-w-0">
                <span class="flex items-center gap-2">
                  <span class="block text-sm font-medium text-ink-900 truncate">{{ App.t(c.name) }}</span>
                  <span v-if="c.code === finance.state.baseCurrency" class="shrink-0 text-[10px] font-medium uppercase tracking-wide text-brand-600 bg-brand-100 rounded px-1.5 py-0.5">{{ App.t('Основная') }}</span>
                  <span v-if="c.archived" class="shrink-0 text-[10px] font-medium uppercase tracking-wide text-ink-400 bg-ink-100 rounded px-1.5 py-0.5">{{ App.t('Деактивирована') }}</span>
                </span>
                <span class="block text-xs text-ink-400 truncate">{{ c.code }}<span v-if="c.code !== finance.state.baseCurrency"> · 1 {{ c.code }} = {{ c.rate }} {{ baseCurrencySymbol }}</span></span>
              </span>
            </span>
            <span class="flex items-center gap-1 shrink-0">
              <button class="p-2 rounded-lg text-ink-500 hover:bg-ink-100 cursor-pointer" :aria-label="App.t('Изменить')" :title="App.t('Изменить код, символ или название')" @click="App.uiStore.openEditCurrency(c)">
                <app-icon name="edit" :size="16" />
              </button>
              <button v-if="!c.archived" class="p-2 rounded-lg text-ink-500 hover:bg-ink-100 cursor-pointer" :aria-label="App.t('Деактивировать')" :title="App.t('Скрыть из выбора при создании счетов')" @click="App.financeStore.archiveCurrency(c.code)">
                <app-icon name="archive" :size="16" />
              </button>
              <button v-else class="p-2 rounded-lg text-ink-500 hover:bg-ink-100 cursor-pointer" :aria-label="App.t('Активировать')" :title="App.t('Вернуть в выбор при создании счетов')" @click="App.financeStore.unarchiveCurrency(c.code)">
                <app-icon name="restore" :size="16" />
              </button>
              <button v-if="!finance.isCurrencyInUse(c.code)" class="p-2 rounded-lg text-money-neg hover:bg-red-50 cursor-pointer" :aria-label="App.t('Удалить')" :title="App.t('Удалить валюту безвозвратно')" @click="App.financeStore.deleteCurrency(c.code)">
                <app-icon name="trash" :size="16" />
              </button>
            </span>
          </li>
        </ul>
      </div>

      <div v-if="tab === 'categories'" class="rounded-xl bg-white border border-ink-200 overflow-hidden">
        <div class="flex items-center justify-between px-4 md:px-5 py-3.5 border-b border-ink-200">
          <div>
            <h2 class="text-sm font-semibold text-ink-900">{{ App.t('Категории') }}</h2>
            <p class="text-xs text-ink-400 mt-0.5">{{ App.tCount(filteredCategories.length, categoryType === App.CategoryType.EXPENSE ? 'expenseCategories' : 'incomeCategories') }}</p>
          </div>
          <button class="flex items-center gap-1.5 rounded-lg bg-brand-600 text-white text-sm font-medium px-3.5 py-2 hover:bg-brand-500 transition-colors cursor-pointer"
            @click="App.uiStore.openNewCategory(categoryType)">
            <app-icon name="plus" :size="16" />
            <span class="hidden sm:inline">{{ App.t('Добавить категорию') }}</span>
          </button>
        </div>
        <div class="px-4 md:px-5 pt-3">
          <div class="flex rounded-lg bg-ink-100 p-1 w-fit text-xs font-medium">
            <button class="px-3 py-1.5 rounded-md transition-colors cursor-pointer" :class="categoryType === App.CategoryType.EXPENSE ? 'bg-white shadow-sm text-ink-900' : 'text-ink-500'" @click="categoryType = App.CategoryType.EXPENSE">{{ App.t('Расходы') }}</button>
            <button class="px-3 py-1.5 rounded-md transition-colors cursor-pointer" :class="categoryType === App.CategoryType.INCOME ? 'bg-white shadow-sm text-ink-900' : 'text-ink-500'" @click="categoryType = App.CategoryType.INCOME">{{ App.t('Доходы') }}</button>
          </div>
        </div>
        <ul class="divide-y divide-ink-100 mt-1">
          <li v-for="c in filteredCategories" :key="c.id" class="flex items-center justify-between gap-3 px-4 md:px-5 py-3.5">
            <span class="flex items-center gap-3 min-w-0">
              <span class="w-8 h-8 rounded-full shrink-0" :class="{ 'opacity-40 grayscale': c.archived }" :style="{ background: c.color }"></span>
              <span class="min-w-0">
                <span class="flex items-center gap-2">
                  <span class="block text-sm font-medium text-ink-900 truncate">{{ App.t(c.name) }}</span>
                  <span v-if="c.archived" class="shrink-0 text-[10px] font-medium uppercase tracking-wide text-ink-400 bg-ink-100 rounded px-1.5 py-0.5">{{ App.t('Деактивирована') }}</span>
                </span>
              </span>
            </span>
            <span class="flex items-center gap-1 shrink-0">
              <button class="p-2 rounded-lg text-ink-500 hover:bg-ink-100 cursor-pointer" :aria-label="App.t('Изменить')" :title="App.t('Изменить название, тип или цвет')" @click="App.uiStore.openEditCategory(c)">
                <app-icon name="edit" :size="16" />
              </button>
              <button v-if="!c.archived" class="p-2 rounded-lg text-ink-500 hover:bg-ink-100 cursor-pointer" :aria-label="App.t('Деактивировать')" :title="App.t('Скрыть из выбора категории при новых операциях')" @click="App.financeStore.archiveCategory(c.id)">
                <app-icon name="archive" :size="16" />
              </button>
              <button v-else class="p-2 rounded-lg text-ink-500 hover:bg-ink-100 cursor-pointer" :aria-label="App.t('Активировать')" :title="App.t('Вернуть в выбор категории при новых операциях')" @click="App.financeStore.unarchiveCategory(c.id)">
                <app-icon name="restore" :size="16" />
              </button>
              <button v-if="!finance.isCategoryInUse(c.id)" class="p-2 rounded-lg text-money-neg hover:bg-red-50 cursor-pointer" :aria-label="App.t('Удалить')" :title="App.t('Удалить категорию безвозвратно')" @click="App.financeStore.deleteCategory(c.id)">
                <app-icon name="trash" :size="16" />
              </button>
            </span>
          </li>
          <li v-if="!filteredCategories.length" class="px-4 md:px-5 py-8 text-center text-sm text-ink-400">{{ App.t('Нет категорий') }}</li>
        </ul>
      </div>

      <div v-if="tab === 'language'" class="rounded-xl bg-white border border-ink-200 p-4 md:p-5">
        <h2 class="text-sm font-semibold text-ink-900 mb-1">{{ App.t('Язык интерфейса') }}</h2>
        <p class="text-xs text-ink-400 mb-3">{{ App.t('Выберите язык, на котором отображается приложение') }}</p>
        <select :value="App.i18nStore.locale" @change="App.authStore.changeLanguage($event.target.value)"
          class="w-full sm:w-64 rounded-lg border border-ink-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500">
          <option v-for="code in App.SUPPORTED_LOCALES" :key="code" :value="code">{{ App.LOCALE_LABELS[code] }}</option>
        </select>
      </div>

      <div v-if="tab === 'account'" class="rounded-xl bg-white border border-ink-200 p-4 md:p-5 max-w-md">
        <h2 class="text-sm font-semibold text-ink-900 mb-1">{{ App.t('Логин и пароль') }}</h2>
        <p class="text-xs text-ink-400 mb-4">{{ App.t('Измените имя пользователя или пароль для входа') }}</p>
        <form class="space-y-4" @submit.prevent="submitAccount">
          <div>
            <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Текущий пароль') }}</label>
            <input v-model="accountForm.currentPassword" type="password" required autocomplete="current-password"
              class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
          </div>
          <p v-if="accountError" class="text-sm text-money-neg">{{ accountError }}</p>
          <div>
            <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Имя пользователя') }}</label>
            <input v-model="accountForm.newUsername" type="text" required autocomplete="username"
              class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
          </div>
          <div>
            <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Новый пароль') }}</label>
            <input v-model="accountForm.newPassword" type="password" autocomplete="new-password" :placeholder="App.t('Оставьте пустым, чтобы не менять')"
              class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
          </div>
          <div v-if="accountForm.newPassword">
            <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Повторите новый пароль') }}</label>
            <input v-model="accountForm.newPasswordConfirm" type="password" autocomplete="new-password"
              class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
          </div>
          <p v-if="accountSuccess" class="text-sm text-money-pos">{{ accountSuccess }}</p>
          <button type="submit" :disabled="accountSaving"
            class="w-full sm:w-auto rounded-lg bg-brand-600 text-white text-sm font-medium py-2.5 px-5 hover:bg-brand-500 transition-colors cursor-pointer disabled:opacity-50">
            {{ accountSaving ? App.t('Сохранение…') : App.t('Сохранить') }}
          </button>
        </form>
      </div>
    </div>
  `,
  data() {
    return {
      App,
      finance: App.financeStore,
      tab: 'categories',
      categoryType: App.CategoryType.EXPENSE,
      accountForm: {
        currentPassword: '',
        newUsername: App.authStore.state.user?.username || '',
        newPassword: '',
        newPasswordConfirm: '',
      },
      accountError: '',
      accountSuccess: '',
      accountSaving: false,
    }
  },
  computed: {
    baseCurrencySymbol() {
      return this.finance.currencyByCode.get(this.finance.state.baseCurrency)?.symbol ?? this.finance.state.baseCurrency
    },
    filteredCategories() {
      return this.finance.state.categories.filter((c) => c.type === this.categoryType)
    },
    activeCurrencies() {
      const list = this.finance.state.currencies.filter((c) => !c.archived)
      if (!list.some((c) => c.code === this.finance.state.baseCurrency)) {
        const current = this.finance.state.currencies.find((c) => c.code === this.finance.state.baseCurrency)
        if (current) list.push(current)
      }
      return list
    },
  },
  methods: {
    async submitAccount() {
      this.accountError = ''
      this.accountSuccess = ''
      if (this.accountForm.newPassword && this.accountForm.newPassword !== this.accountForm.newPasswordConfirm) {
        this.accountError = App.t('Новые пароли не совпадают')
        return
      }
      this.accountSaving = true
      try {
        await App.authStore.changeCredentials(
          this.accountForm.currentPassword,
          this.accountForm.newUsername,
          this.accountForm.newPassword,
        )
        this.accountSuccess = App.t('Данные для входа обновлены')
        this.accountForm.currentPassword = ''
        this.accountForm.newPassword = ''
        this.accountForm.newPasswordConfirm = ''
      } catch {
        this.accountError = App.t('Неверный текущий пароль')
      } finally {
        this.accountSaving = false
      }
    },
  },
}
