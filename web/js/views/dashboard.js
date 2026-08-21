window.App = window.App || {};

function monthKey(dateStr) {
  const d = new Date(dateStr)
  return `${d.getFullYear()}-${d.getMonth()}`
}

App.DashboardView = {
  components: {
    'skeleton-block': App.SkeletonBlock,
    'donut-chart': App.DonutChart,
    'bar-chart': App.BarChart,
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
          <div class="flex items-center justify-between mb-4">
            <h2 class="font-semibold text-ink-950 text-sm">{{ App.t('Доходы и расходы по месяцам') }}</h2>
          </div>
          <div class="h-64">
            <bar-chart :labels="monthlyTrend.map(m => m.label)" :income="monthlyTrend.map(m => m.income)" :expense="monthlyTrend.map(m => m.expense)" />
          </div>
          <div class="flex items-center gap-4 mt-3 text-xs text-ink-500">
            <span class="flex items-center gap-1.5"><span class="w-2.5 h-2.5 rounded-sm bg-[#86efac]"></span>{{ App.t('Доходы') }}</span>
            <span class="flex items-center gap-1.5"><span class="w-2.5 h-2.5 rounded-sm bg-[#fca5a5]"></span>{{ App.t('Расходы') }}</span>
          </div>
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
      now,
      currentMonthKey: `${now.getFullYear()}-${now.getMonth()}`,
      // Bounded to the last 6 months (see loadTransactions) rather than the whole transaction
      // history, since that's all monthExpenses/monthIncome/monthlyTrend below need.
      sixMonthTransactions: [],
      loading: true,
    }
  },
  computed: {
    monthLabel() {
      return App.formatMonthLabel(this.now.getFullYear(), this.now.getMonth())
    },
    monthExpenses() {
      return this.sixMonthTransactions.filter((t) => t.type !== App.TransactionType.TRANSFER && t.amount < 0 && monthKey(t.date) === this.currentMonthKey)
    },
    monthIncome() {
      return this.sixMonthTransactions.filter((t) => t.type !== App.TransactionType.TRANSFER && t.amount > 0 && monthKey(t.date) === this.currentMonthKey)
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
    monthlyTrend() {
      const months = []
      for (let i = 5; i >= 0; i--) {
        const d = new Date(this.now.getFullYear(), this.now.getMonth() - i, 1)
        months.push({ key: `${d.getFullYear()}-${d.getMonth()}`, label: App.formatMonthLabel(d.getFullYear(), d.getMonth()), income: 0, expense: 0 })
      }
      for (const t of this.sixMonthTransactions) {
        if (t.type === App.TransactionType.TRANSFER) continue
        const key = monthKey(t.date)
        const m = months.find((x) => x.key === key)
        if (!m) continue
        const amt = this.finance.amountInBase(t)
        if (amt > 0) m.income += amt
        else m.expense += Math.abs(amt)
      }
      return months
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
    async loadTransactions() {
      this.loading = true
      const dateFrom = App.dateStr(new Date(this.now.getFullYear(), this.now.getMonth() - 5, 1))
      const dateTo = App.dateStr(this.now)
      try {
        this.sixMonthTransactions = await App.api.getAllTransactions({ dateFrom, dateTo })
      } finally {
        this.loading = false
      }
    },
  },
}
