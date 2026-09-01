window.App = window.App || {};

App.BudgetView = {
  components: { 'app-icon': App.AppIcon, 'skeleton-block': App.SkeletonBlock },
  template: `
    <div v-if="finance.state.loading" class="space-y-4">
      <skeleton-block class="h-64" />
    </div>

    <div v-else class="rounded-xl bg-white border border-ink-200 overflow-hidden">
      <div class="flex items-center justify-between px-4 md:px-5 py-3.5 border-b border-ink-200 flex-wrap gap-2">
        <div>
          <h2 class="text-sm font-semibold text-ink-900">{{ App.t('Бюджет по категориям') }}</h2>
          <p class="text-xs text-ink-400 mt-0.5">{{ App.t('Расходный лимит на месяц, в {currency}', { currency: finance.state.baseCurrency }) }}</p>
        </div>
        <div class="flex items-center gap-1 rounded-lg border border-ink-200">
          <button type="button" class="p-2 text-ink-500 hover:bg-ink-100 rounded-l-lg cursor-pointer" :aria-label="App.t('Предыдущий месяц')" @click="shiftMonth(-1)">
            <app-icon name="chevron" :size="16" class="rotate-180" />
          </button>
          <span class="text-sm font-medium text-ink-900 min-w-[9rem] text-center select-none">{{ monthLabel }}</span>
          <button type="button" class="p-2 text-ink-500 hover:bg-ink-100 rounded-r-lg cursor-pointer" :aria-label="App.t('Следующий месяц')" @click="shiftMonth(1)">
            <app-icon name="chevron" :size="16" />
          </button>
        </div>
      </div>
      <div v-if="!hasBudgetThisMonth" class="px-4 md:px-5 py-3 border-b border-ink-200 bg-ink-50/60 flex items-center justify-between gap-2">
        <p class="text-xs text-ink-500">{{ App.t('На этот месяц бюджет ещё не задан') }}</p>
        <button class="shrink-0 text-xs font-medium text-brand-600 hover:underline cursor-pointer" @click="copyFromPreviousMonth">
          {{ App.t('Скопировать из прошлого месяца') }}
        </button>
      </div>
      <div class="flex items-center justify-between gap-3 px-4 md:px-5 py-3 border-b border-ink-200 bg-ink-50/60">
        <span class="text-sm font-semibold text-ink-900">{{ App.t('Всего') }}</span>
        <span class="text-sm font-semibold text-ink-900 tabular-nums">{{ App.formatMoney(budgetTotal, finance.state.baseCurrency) }}</span>
      </div>
      <ul class="divide-y divide-ink-100">
        <li v-for="c in budgetCategories" :key="c.id" class="flex items-center justify-between gap-3 px-4 md:px-5 py-3.5">
          <span class="flex items-center gap-2.5 min-w-0">
            <span class="w-2.5 h-2.5 rounded-full shrink-0" :class="{ 'opacity-40 grayscale': c.archived }" :style="{ background: c.color }"></span>
            <span class="text-sm text-ink-900 truncate" :class="{ 'text-ink-400': c.archived }">{{ App.t(c.name) }}</span>
            <span v-if="c.archived" class="shrink-0 text-[10px] font-medium uppercase tracking-wide text-ink-400 bg-ink-100 rounded px-1.5 py-0.5">{{ App.t('Деактивирована') }}</span>
          </span>
          <input type="number" min="0" step="100" placeholder="0"
            :value="budgetAmount(c.id) || ''"
            @change="updateBudget(c.id, $event.target.value)"
            class="w-32 rounded-lg border border-ink-200 px-3 py-1.5 text-sm text-right focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
        </li>
      </ul>
    </div>
  `,
  data() {
    const now = new Date()
    return {
      App,
      finance: App.financeStore,
      budgetMonth: `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`,
    }
  },
  computed: {
    budgetCategories() {
      // Archived categories stay listed here (unlike the transaction form's category picker) so a
      // budget already set for one remains visible/editable after it's archived — only deleting the
      // category removes it from the budget (see App.financeStore.deleteCategory).
      return this.finance.state.categories.filter((c) => c.type === App.CategoryType.EXPENSE)
    },
    monthLabel() {
      return App.formatMonthYear(this.budgetMonth)
    },
    previousBudgetMonth() {
      return this.shiftMonthKey(this.budgetMonth, -1)
    },
    hasBudgetThisMonth() {
      return this.budgetCategories.some((c) => this.budgetAmount(c.id) > 0)
    },
    budgetTotal() {
      return this.budgetCategories.reduce((sum, c) => sum + this.budgetAmount(c.id), 0)
    },
  },
  methods: {
    shiftMonthKey(monthKey, delta) {
      const [y, m] = monthKey.split('-').map(Number)
      const d = new Date(y, m - 1 + delta, 1)
      return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
    },
    shiftMonth(delta) {
      this.budgetMonth = this.shiftMonthKey(this.budgetMonth, delta)
    },
    budgetAmount(categoryId) {
      return this.finance.budgetFor(categoryId, this.budgetMonth)
    },
    updateBudget(categoryId, value) {
      App.financeStore.setCategoryBudget(categoryId, this.budgetMonth, Math.round(Number(value) || 0))
    },
    copyFromPreviousMonth() {
      for (const c of this.budgetCategories) {
        const prev = this.finance.budgetFor(c.id, this.previousBudgetMonth)
        if (prev > 0) App.financeStore.setCategoryBudget(c.id, this.budgetMonth, prev)
      }
    },
  },
}
