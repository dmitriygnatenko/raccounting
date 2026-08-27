window.App = window.App || {};

function monthKey(dateStr) {
  const d = new Date(dateStr)
  return `${d.getFullYear()}-${d.getMonth()}`
}

App.DashboardView = {
  components: {
    'skeleton-block': App.SkeletonBlock,
    'donut-chart': App.DonutChart,
  },
  template: `
    <div v-if="finance.state.loading || loading" class="space-y-5">
      <div class="grid grid-cols-2 lg:grid-cols-4 gap-3 md:gap-4">
        <skeleton-block v-for="i in 4" :key="i" class="h-24" />
      </div>
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <skeleton-block class="h-80 lg:col-span-2" />
        <skeleton-block class="h-80" />
      </div>
    </div>

    <div v-else class="space-y-5">
      <div class="grid grid-cols-2 lg:grid-cols-4 gap-3 md:gap-4">
        <div class="rounded-xl bg-white border border-ink-200 p-4">
          <p class="text-xs font-medium text-ink-500 mb-1.5">{{ App.t('Общий баланс') }}</p>
          <p class="text-xl md:text-2xl font-semibold text-ink-950">{{ formatMoney(finance.totalBalanceBase, finance.state.baseCurrency) }}</p>
        </div>
        <div class="rounded-xl bg-white border border-ink-200 p-4">
          <p class="text-xs font-medium text-ink-500 mb-1.5">{{ App.t('Доходы, {month}', { month: monthLabel }) }}</p>
          <p class="text-xl md:text-2xl font-semibold" :class="totalIncome ? 'text-money-pos' : 'text-ink-950'">{{ totalIncome ? '+' : '' }}{{ formatMoney(totalIncome, finance.state.baseCurrency) }}</p>
        </div>
        <div class="rounded-xl bg-white border border-ink-200 p-4">
          <p class="text-xs font-medium text-ink-500 mb-1.5">{{ App.t('Расходы, {month}', { month: monthLabel }) }}</p>
          <p class="text-xl md:text-2xl font-semibold" :class="totalExpense ? 'text-money-neg' : 'text-ink-950'">{{ totalExpense ? '−' : '' }}{{ formatMoney(totalExpense, finance.state.baseCurrency) }}</p>
        </div>
        <div class="rounded-xl bg-white border border-ink-200 p-4">
          <p class="text-xs font-medium text-ink-500 mb-1.5">{{ App.t('Долг') }}</p>
          <p class="text-xl md:text-2xl font-semibold" :class="totalDebt ? 'text-money-neg' : 'text-ink-950'">{{ formatMoney(totalDebt, finance.state.baseCurrency) }}</p>
        </div>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <div class="rounded-xl bg-white border border-ink-200 p-4 md:p-5 lg:col-span-2">
          <div class="flex items-center justify-between mb-3">
            <h2 class="font-semibold text-ink-950 text-sm">{{ App.t('Последние операции') }}</h2>
            <button type="button" class="text-xs font-medium text-ink-500 hover:text-ink-800 cursor-pointer" @click="App.router.push('/transactions')">{{ App.t('Все операции') }}</button>
          </div>
          <ul v-if="recentTransactions.length" class="divide-y divide-ink-100 -mx-1">
            <li v-for="t in recentTransactions" :key="t.id">
              <button type="button" class="w-full text-left flex items-center gap-3 px-1 py-2.5 rounded-lg hover:bg-ink-50/60 transition-colors cursor-pointer"
                @click="ui.openEditTransaction(t)">
                <span class="w-9 h-9 rounded-full flex items-center justify-center shrink-0 text-xs font-semibold"
                  :style="{ background: txColor(t) + '1a', color: txColor(t) }">
                  {{ txTitle(t).slice(0, 1).toUpperCase() }}
                </span>
                <span class="min-w-0 flex-1">
                  <span class="flex items-center justify-between gap-2">
                    <span class="text-sm font-medium text-ink-900 truncate">{{ txTitle(t) }}</span>
                    <span class="text-sm font-semibold shrink-0" :class="t.amount < 0 ? 'text-money-neg' : 'text-money-pos'">
                      {{ t.amount < 0 ? '−' : '+' }}{{ formatMoney(Math.abs(t.amount), finance.accountById.get(t.accountId)?.currency) }}
                    </span>
                  </span>
                  <span class="flex items-center justify-between gap-2 mt-0.5">
                    <span class="text-xs text-ink-400 truncate">{{ App.t(finance.accountById.get(t.accountId)?.name) }}<template v-if="t.memo"> · {{ t.memo }}</template></span>
                    <span class="text-xs text-ink-400 shrink-0">{{ App.formatDate(t.date) }}</span>
                  </span>
                </span>
              </button>
            </li>
          </ul>
          <p v-else class="text-sm text-ink-400 py-8 text-center">{{ App.t('Пока нет операций') }}</p>
        </div>

        <div class="rounded-xl bg-white border border-ink-200 p-4 md:p-5">
          <h2 class="font-semibold text-ink-950 text-sm mb-1">{{ App.t('Топ категорий трат') }}</h2>
          <p class="text-xs text-ink-500 mb-3">{{ monthLabel }}</p>
          <div v-if="topCategories.length" class="h-40 relative">
            <donut-chart :labels="topCategories.map(c => App.t(c.category?.name ?? 'Без категории'))" :values="topCategories.map(c => c.value)" :colors="topCategories.map(c => c.category?.color ?? '#94a3b8')" />
            <div class="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
              <span class="text-[11px] text-ink-500">{{ App.t('Всего') }}</span>
              <span class="text-sm font-semibold text-ink-950">{{ formatMoney(totalExpense, finance.state.baseCurrency) }}</span>
            </div>
          </div>
          <p v-else class="text-sm text-ink-400 py-8 text-center">{{ App.t('Нет расходов в этом месяце') }}</p>
          <ul class="mt-4 space-y-2">
            <li v-for="c in topCategories" :key="c.category?.id" class="flex items-center justify-between text-sm">
              <span class="flex items-center gap-2 text-ink-700 min-w-0">
                <span class="w-2.5 h-2.5 rounded-full shrink-0" :style="{ background: c.category?.color }"></span>
                <span class="truncate">{{ App.t(c.category?.name ?? 'Без категории') }}</span>
              </span>
              <span class="font-medium text-ink-950 shrink-0 ml-2">{{ formatMoney(c.value, finance.state.baseCurrency) }}</span>
            </li>
          </ul>
        </div>
      </div>
    </div>
  `,
  data() {
    const now = new Date()
    return {
      App,
      finance: App.financeStore,
      ui: App.uiStore,
      now,
      currentMonthKey: `${now.getFullYear()}-${now.getMonth()}`,
      // Current calendar month only — that's all the stat cards and the Топ категорий donut need
      // (see loadTransactions).
      monthTransactions: [],
      // The few newest transactions across all time, for the Последние операции list.
      recentTransactions: [],
      loading: true,
    }
  },
  computed: {
    monthLabel() {
      return App.formatMonthLabel(this.now.getFullYear(), this.now.getMonth())
    },
    monthExpenses() {
      return this.monthTransactions.filter((t) => t.type !== App.TransactionType.TRANSFER && t.amount < 0 && monthKey(t.date) === this.currentMonthKey)
    },
    monthIncome() {
      return this.monthTransactions.filter((t) => t.type !== App.TransactionType.TRANSFER && t.amount > 0 && monthKey(t.date) === this.currentMonthKey)
    },
    totalExpense() {
      return this.monthExpenses.reduce((s, t) => s + Math.abs(this.finance.amountInBase(t)), 0)
    },
    totalIncome() {
      return this.monthIncome.reduce((s, t) => s + this.finance.amountInBase(t), 0)
    },
    totalDebt() {
      const debtTypes = [App.AccountType.CREDIT_CARD, App.AccountType.DEBT]
      return this.finance.activeAccounts
        .filter((a) => debtTypes.includes(a.type))
        // Only a negative balance is debt — an overpaid credit card (positive balance) isn't.
        .reduce((sum, a) => sum + Math.max(0, -this.finance.toBase(a.balance, a.currency)), 0)
    },
    topCategories() {
      const sums = new Map()
      for (const t of this.monthExpenses) {
        const key = t.categoryId ?? 'other-expense'
        sums.set(key, (sums.get(key) ?? 0) + Math.abs(this.finance.amountInBase(t)))
      }
      return [...sums.entries()]
        .sort((a, b) => b[1] - a[1])
        .slice(0, 5)
        .map(([id, value]) => ({ category: App.getCategory(id), value }))
    },
  },
  watch: {
    'finance.state.transactionsVersion'() { this.loadTransactions() },
  },
  async mounted() {
    await this.loadTransactions()
  },
  methods: {
    formatMoney: App.formatMoney,
    txColor(t) {
      if (t.type === App.TransactionType.TRANSFER) return '#94a3b8'
      return this.finance.categoryById.get(t.categoryId ?? '')?.color ?? '#94a3b8'
    },
    txTitle(t) {
      if (t.type === App.TransactionType.TRANSFER) return App.t('Перевод')
      return App.t(this.finance.categoryById.get(t.categoryId ?? '')?.name ?? 'Без категории')
    },
    async loadTransactions() {
      this.loading = true
      const dateFrom = App.dateStr(new Date(this.now.getFullYear(), this.now.getMonth(), 1))
      const dateTo = App.dateStr(this.now)
      try {
        const [monthTx, recent] = await Promise.all([
          App.api.getAllTransactions({ dateFrom, dateTo }),
          App.api.getTransactions({ page: 1, pageSize: 8 }),
        ])
        this.monthTransactions = monthTx
        this.recentTransactions = recent.transactions
      } finally {
        this.loading = false
      }
    },
  },
}
