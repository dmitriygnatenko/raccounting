window.App = window.App || {};

App.ReportsView = {
  components: { 'donut-chart': App.DonutChart, 'line-chart': App.LineChart, 'skeleton-block': App.SkeletonBlock },
  template: `
    <div v-if="finance.state.loading" class="space-y-4">
      <skeleton-block class="h-12" />
      <skeleton-block class="h-96" />
    </div>

    <div v-else class="space-y-5">
      <div class="flex gap-1.5 overflow-x-auto pb-1 -mx-1 px-1">
        <button v-for="opt in periodOptions" :key="opt.value"
          class="shrink-0 px-3.5 py-1.5 rounded-full text-sm font-medium transition-colors cursor-pointer"
          :class="period === opt.value ? 'bg-brand-600 text-white' : 'bg-white border border-ink-200 text-ink-500 hover:text-ink-900'"
          @click="period = opt.value">
          {{ App.t(opt.label) }}
        </button>
      </div>
      <div v-if="period === 'custom'" class="flex flex-wrap gap-2.5">
        <input v-model="dateFrom" type="date" class="rounded-lg border border-ink-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
        <input v-model="dateTo" type="date" class="rounded-lg border border-ink-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
      </div>

      <div class="grid grid-cols-3 gap-3 md:gap-4">
        <div class="rounded-xl bg-white border border-ink-200 p-4">
          <p class="text-xs font-medium text-ink-500 mb-1.5">{{ App.t('Доходы') }}</p>
          <p class="text-base md:text-xl font-semibold text-money-pos">+{{ App.formatMoney(summary.income, finance.state.baseCurrency) }}</p>
        </div>
        <div class="rounded-xl bg-white border border-ink-200 p-4">
          <p class="text-xs font-medium text-ink-500 mb-1.5">{{ App.t('Расходы') }}</p>
          <p class="text-base md:text-xl font-semibold text-money-neg">−{{ App.formatMoney(summary.expense, finance.state.baseCurrency) }}</p>
        </div>
        <div class="rounded-xl bg-white border border-ink-200 p-4">
          <p class="text-xs font-medium text-ink-500 mb-1.5">{{ App.t('Итого') }}</p>
          <p class="text-base md:text-xl font-semibold" :class="summary.net >= 0 ? 'text-ink-950' : 'text-money-neg'">
            {{ summary.net >= 0 ? '+' : '−' }}{{ App.formatMoney(Math.abs(summary.net), finance.state.baseCurrency) }}
          </p>
        </div>
      </div>

      <div class="rounded-xl bg-white border border-ink-200 p-4 md:p-5">
        <div class="flex flex-wrap items-center justify-between gap-3 mb-4">
          <h2 class="font-semibold text-ink-950 text-sm">{{ App.t('Расходы по категориям') }}</h2>
          <div class="flex items-center gap-2">
            <div class="flex rounded-lg bg-ink-100 p-1 text-xs font-medium">
              <button class="px-2.5 py-1 rounded-md transition-colors cursor-pointer" :class="type === 'expense' ? 'bg-white shadow-sm text-ink-900' : 'text-ink-500'" @click="type = 'expense'">{{ App.t('Расходы') }}</button>
              <button class="px-2.5 py-1 rounded-md transition-colors cursor-pointer" :class="type === 'income' ? 'bg-white shadow-sm text-ink-900' : 'text-ink-500'" @click="type = 'income'">{{ App.t('Доходы') }}</button>
            </div>
            <div class="flex rounded-lg bg-ink-100 p-1 text-xs font-medium">
              <button class="px-2.5 py-1 rounded-md transition-colors cursor-pointer" :class="chartMode === 'donut' ? 'bg-white shadow-sm text-ink-900' : 'text-ink-500'" @click="chartMode = 'donut'">{{ App.t('Круговая') }}</button>
              <button class="px-2.5 py-1 rounded-md transition-colors cursor-pointer" :class="chartMode === 'bar' ? 'bg-white shadow-sm text-ink-900' : 'text-ink-500'" @click="chartMode = 'bar'">{{ App.t('Список') }}</button>
            </div>
          </div>
        </div>

        <div v-if="!categoryBreakdown.length" class="py-12 text-center text-sm text-ink-400">{{ App.t('Нет данных за этот период') }}</div>
        <div v-else class="grid grid-cols-1 md:grid-cols-2 gap-6 items-center">
          <div v-if="chartMode === 'donut'" class="h-64 relative">
            <donut-chart :labels="categoryBreakdown.map(c => App.t(c.category?.name ?? '—'))" :values="categoryBreakdown.map(c => c.value)" :colors="categoryBreakdown.map(c => c.category?.color ?? '#94a3b8')" />
            <div class="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
              <span class="text-xs text-ink-500">{{ App.t('Всего') }}</span>
              <span class="text-base font-semibold text-ink-950">{{ App.formatMoney(breakdownTotal, finance.state.baseCurrency) }}</span>
            </div>
          </div>
          <div v-else class="space-y-3">
            <div v-for="c in categoryBreakdown" :key="c.category?.id">
              <div class="flex items-center justify-between text-sm mb-1">
                <span class="flex items-center gap-2 text-ink-700">
                  <span class="w-2.5 h-2.5 rounded-full" :style="{ background: c.category?.color }"></span>
                  {{ App.t(c.category?.name ?? 'Без категории') }}
                </span>
                <span class="font-medium text-ink-950">{{ App.formatMoney(c.value, finance.state.baseCurrency) }}</span>
              </div>
              <div class="h-1.5 rounded-full bg-ink-100 overflow-hidden">
                <div class="h-full rounded-full" :style="{ width: c.pct + '%', background: c.category?.color }"></div>
              </div>
            </div>
          </div>

          <ul v-if="chartMode === 'donut'" class="space-y-2.5">
            <li v-for="c in categoryBreakdown" :key="c.category?.id" class="flex items-center justify-between text-sm">
              <span class="flex items-center gap-2 text-ink-700 min-w-0">
                <span class="w-2.5 h-2.5 rounded-full shrink-0" :style="{ background: c.category?.color }"></span>
                <span class="truncate">{{ App.t(c.category?.name ?? 'Без категории') }}</span>
              </span>
              <span class="flex items-center gap-2 shrink-0 ml-2">
                <span class="text-ink-400 text-xs">{{ c.pct }}%</span>
                <span class="font-medium text-ink-950 w-24 text-right">{{ App.formatMoney(c.value, finance.state.baseCurrency) }}</span>
              </span>
            </li>
          </ul>
        </div>
      </div>

      <div class="rounded-xl bg-white border border-ink-200 p-4 md:p-5">
        <h2 class="font-semibold text-ink-950 text-sm mb-1">{{ App.t('Динамика баланса') }}</h2>
        <p class="text-xs text-ink-500 mb-4">{{ App.t('Все счета, в {currency}, последние 30 дней', { currency: finance.state.baseCurrency }) }}</p>
        <div v-if="balanceHistory.length" class="h-64">
          <line-chart :labels="balanceLabels" :values="balanceHistory.map(p => p.balance)" />
        </div>
        <p v-else class="py-12 text-center text-sm text-ink-400">{{ App.t('Недостаточно данных') }}</p>
      </div>

      <div class="rounded-xl bg-white border border-ink-200 p-4 md:p-5">
        <div class="flex items-center justify-between mb-4 flex-wrap gap-2">
          <h2 class="font-semibold text-ink-950 text-sm">{{ App.t('Бюджет по категориям') }}</h2>
          <div class="flex gap-1.5">
            <button v-for="opt in budgetPeriodOptions" :key="opt.value"
              class="shrink-0 px-2.5 py-1 rounded-full text-xs font-medium transition-colors cursor-pointer"
              :class="budgetPeriod === opt.value ? 'bg-brand-600 text-white' : 'bg-ink-100 text-ink-500 hover:text-ink-900'"
              @click="budgetPeriod = opt.value">
              {{ App.t(opt.label) }}
            </button>
          </div>
        </div>
        <ul class="space-y-4">
          <li v-for="row in budgetRows" :key="row.category?.id">
            <div class="flex items-center justify-between text-sm mb-1.5">
              <span class="flex items-center gap-2 text-ink-700">
                <span class="w-2.5 h-2.5 rounded-full" :style="{ background: row.category?.color }"></span>
                {{ App.t(row.category?.name) }}
              </span>
              <span class="text-xs">
                <span :class="row.over ? 'text-money-neg font-medium' : 'text-ink-950 font-medium'">{{ App.formatMoney(row.spent, finance.state.baseCurrency) }}</span>
                <span class="text-ink-400"> / {{ App.formatMoney(row.budgeted, finance.state.baseCurrency) }}</span>
              </span>
            </div>
            <div class="h-2 rounded-full bg-ink-100 overflow-hidden">
              <div class="h-full rounded-full transition-all" :class="row.over ? 'bg-money-neg' : 'bg-money-pos'" :style="{ width: row.pct + '%' }"></div>
            </div>
          </li>
        </ul>
      </div>
    </div>
  `,
  data() {
    return {
      App,
      finance: App.financeStore,
      period: 'month',
      dateFrom: '',
      dateTo: '',
      type: 'expense',
      chartMode: 'donut',
      periodOptions: [
        { value: 'month', label: 'Этот месяц' },
        { value: 'lastMonth', label: 'Прошлый месяц' },
        { value: 'year', label: 'Этот год' },
        { value: 'all', label: 'Всё время' },
        { value: 'custom', label: 'Диапазон дат' },
      ],
      budgetPeriod: 'month',
      budgetPeriodOptions: [
        { value: 'month', label: 'Этот месяц' },
        { value: 'lastMonth', label: 'Прошлый месяц' },
      ],
    }
  },
  computed: {
    periodTransactions() {
      return this.finance.state.transactions.filter((t) => !t.scheduled && this.inPeriod(t.date, this.period, this.dateFrom, this.dateTo))
    },
    summary() {
      const nonTransfer = this.periodTransactions.filter((t) => t.type !== 'transfer')
      const income = nonTransfer.filter((t) => t.amount > 0).reduce((s, t) => s + this.finance.amountInBase(t), 0)
      const expense = nonTransfer.filter((t) => t.amount < 0).reduce((s, t) => s + Math.abs(this.finance.amountInBase(t)), 0)
      return { income, expense, net: income - expense }
    },
    categoryBreakdown() {
      const sums = new Map()
      for (const t of this.periodTransactions) {
        if (t.type === 'transfer') continue
        const matchesType = this.type === 'expense' ? t.amount < 0 : t.amount > 0
        if (!matchesType) continue
        const key = t.categoryId ?? (this.type === 'expense' ? 'other-expense' : 'other-income')
        sums.set(key, (sums.get(key) ?? 0) + Math.abs(this.finance.amountInBase(t)))
      }
      const total = [...sums.values()].reduce((s, v) => s + v, 0)
      return [...sums.entries()]
        .sort((a, b) => b[1] - a[1])
        .map(([id, value]) => ({ category: App.getCategory(id), value, pct: total ? Math.round((value / total) * 100) : 0 }))
    },
    breakdownTotal() {
      return this.categoryBreakdown.reduce((s, c) => s + c.value, 0)
    },
    balanceHistory() {
      const relevant = this.finance.state.transactions
        .filter((t) => !t.scheduled)
        .slice()
        .sort((a, b) => (a.date < b.date ? -1 : 1))

      const byDate = new Map()
      for (const t of relevant) byDate.set(t.date, (byDate.get(t.date) ?? 0) + this.finance.amountInBase(t))

      const dates = [...byDate.keys()].sort()
      if (!dates.length) return []

      let runningBalance = this.finance.totalBalanceBase
      const points = []
      for (let i = dates.length - 1; i >= 0; i--) {
        points.push({ date: dates[i], balance: runningBalance })
        runningBalance -= byDate.get(dates[i])
      }
      points.reverse()

      const cutoff = new Date()
      cutoff.setDate(cutoff.getDate() - 30)
      const cutoffStr = cutoff.toISOString().slice(0, 10)
      return points.filter((p) => p.date >= cutoffStr)
    },
    balanceLabels() {
      return this.balanceHistory.map((p) => new Intl.DateTimeFormat('ru-RU', { day: '2-digit', month: 'short' }).format(new Date(p.date)))
    },
    budgetTargetDate() {
      const now = new Date()
      if (this.budgetPeriod === 'lastMonth') return new Date(now.getFullYear(), now.getMonth() - 1, 1)
      return new Date(now.getFullYear(), now.getMonth(), 1)
    },
    budgetMonthKey() {
      const target = this.budgetTargetDate
      return `${target.getFullYear()}-${String(target.getMonth() + 1).padStart(2, '0')}`
    },
    budgetRows() {
      const target = this.budgetTargetDate
      const monthKey = this.budgetMonthKey
      const spentByCategory = new Map()
      for (const t of this.finance.state.transactions) {
        if (t.scheduled || t.amount >= 0 || t.type === 'transfer') continue
        const d = new Date(t.date)
        if (d.getFullYear() !== target.getFullYear() || d.getMonth() !== target.getMonth()) continue
        const key = t.categoryId ?? 'other-expense'
        spentByCategory.set(key, (spentByCategory.get(key) ?? 0) + Math.abs(this.finance.amountInBase(t)))
      }
      const categoryIds = new Set([...Object.keys(this.finance.state.categoryBudgets), ...spentByCategory.keys()])
      return [...categoryIds]
        .map((categoryId) => {
          const budgeted = this.finance.budgetFor(categoryId, monthKey)
          const spent = spentByCategory.get(categoryId) ?? 0
          return {
            category: App.getCategory(categoryId),
            budgeted,
            spent,
            pct: budgeted ? Math.min(100, Math.round((spent / budgeted) * 100)) : (spent > 0 ? 100 : 0),
            over: budgeted ? spent > budgeted : spent > 0,
          }
        })
        .filter((r) => r.spent > 0 || r.budgeted > 0)
        .sort((a, b) => b.spent - a.spent)
    },
  },
  methods: {
    inPeriod(dateStr, p, from, to) {
      if (p === 'all') return true
      if (p === 'custom') {
        if (from && dateStr < from) return false
        if (to && dateStr > to) return false
        return true
      }
      const d = new Date(dateStr)
      const now = new Date()
      if (p === 'month') return d.getFullYear() === now.getFullYear() && d.getMonth() === now.getMonth()
      if (p === 'lastMonth') {
        const last = new Date(now.getFullYear(), now.getMonth() - 1, 1)
        return d.getFullYear() === last.getFullYear() && d.getMonth() === last.getMonth()
      }
      return d.getFullYear() === now.getFullYear()
    },
  },
}
