window.App = window.App || {};

App.TransactionsView = {
  components: { 'app-icon': App.AppIcon, 'skeleton-block': App.SkeletonBlock },
  template: `
    <div class="space-y-4">
      <div class="rounded-xl bg-white border border-ink-200 p-3 md:p-4">
        <div class="flex flex-col lg:flex-row gap-2.5">
          <div class="relative flex-1">
            <span class="absolute left-3 top-1/2 -translate-y-1/2 text-ink-400">
              <app-icon name="search" :size="16" />
            </span>
            <input v-model="filters.search" type="text" :placeholder="App.t('Поиск по заметке, категории…')"
              class="w-full rounded-lg border border-ink-200 pl-9 pr-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
          </div>
          <div class="grid grid-cols-2 lg:flex gap-2.5">
            <select v-model="filters.accountId" class="rounded-lg border border-ink-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500">
              <option value="">{{ App.t('Все счета') }}</option>
              <option v-for="a in finance.state.accounts" :key="a.id" :value="a.id">{{ App.t(a.name) }}</option>
            </select>
            <select v-model="filters.categoryId" class="rounded-lg border border-ink-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500">
              <option value="">{{ App.t('Все категории') }}</option>
              <option v-for="c in finance.state.categories" :key="c.id" :value="c.id">{{ App.t(c.name) }}</option>
            </select>
            <select v-model="filters.tagId" class="rounded-lg border border-ink-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500">
              <option value="">{{ App.t('Все теги') }}</option>
              <option v-for="t in finance.state.tags" :key="t.id" :value="t.id">{{ App.t(t.name) }}</option>
            </select>
            <select v-model="filters.direction" class="rounded-lg border border-ink-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500">
              <option value="all">{{ App.t('Все типы') }}</option>
              <option value="expense">{{ App.t('Расходы') }}</option>
              <option value="income">{{ App.t('Доходы') }}</option>
              <option value="transfer">{{ App.t('Переводы') }}</option>
            </select>
            <select v-model="filters.period" class="rounded-lg border border-ink-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500">
              <option value="all">{{ App.t('Всё время') }}</option>
              <option value="month">{{ App.t('Этот месяц') }}</option>
              <option value="lastMonth">{{ App.t('Прошлый месяц') }}</option>
              <option value="year">{{ App.t('Этот год') }}</option>
              <option value="custom">{{ App.t('Диапазон дат') }}</option>
            </select>
          </div>
          <div v-if="filters.period === 'custom'" class="grid grid-cols-2 lg:flex gap-2.5">
            <input v-model="filters.dateFrom" type="date" class="rounded-lg border border-ink-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
            <input v-model="filters.dateTo" type="date" class="rounded-lg border border-ink-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
          </div>
          <button v-if="hasActiveFilters" class="flex items-center justify-center gap-1.5 text-sm font-medium text-ink-500 hover:text-ink-900 px-2 cursor-pointer" @click="resetFilters">
            <app-icon name="x" :size="14" />
            {{ App.t('Сбросить') }}
          </button>
        </div>
      </div>

      <div class="flex items-center justify-between text-sm text-ink-500 px-1">
        <span>{{ App.t('Найдено') }}: {{ totalCount }}</span>
        <span>{{ App.t('Сумма') }}: <span class="font-semibold" :class="resultSum < 0 ? 'text-money-neg' : 'text-money-pos'">{{ App.formatMoney(resultSum, resultCurrency) }}</span></span>
      </div>

      <div v-if="finance.state.loading || loading" class="space-y-2">
        <skeleton-block v-for="i in 6" :key="i" class="h-14" />
      </div>

      <div v-else-if="!groups.length" class="rounded-xl bg-white border border-ink-200 py-16 text-center text-sm text-ink-400">
        {{ App.t('Операции не найдены') }}
      </div>

      <div v-else class="rounded-xl bg-white border border-ink-200 overflow-hidden">
        <div class="hidden lg:grid grid-cols-[160px_140px_1fr_120px_32px] gap-3 px-5 py-2.5 border-b border-ink-200 bg-ink-50/60 text-xs font-medium text-ink-400 uppercase tracking-wide">
          <span>{{ App.t('Категория') }}</span>
          <span>{{ App.t('Счёт') }}</span>
          <span>{{ App.t('Заметка') }}</span>
          <span class="text-right">{{ App.t('Сумма') }}</span>
          <span></span>
        </div>

        <div v-for="group in groups" :key="group.date">
          <div class="flex items-center justify-between px-4 md:px-5 py-2 bg-ink-50/40 border-b border-ink-100">
            <span class="text-xs font-medium text-ink-500">{{ App.formatDateLong(group.date) }}</span>
            <span class="text-xs font-medium" :class="group.total < 0 ? 'text-money-neg' : 'text-money-pos'">
              {{ group.total < 0 ? '−' : '+' }}{{ App.formatMoney(Math.abs(group.total), resultCurrency) }}
            </span>
          </div>

          <ul class="divide-y divide-ink-100">
            <li v-for="t in group.txs" :key="t.id">
              <button class="hidden lg:grid w-full text-left grid-cols-[160px_140px_1fr_120px_32px] gap-3 items-center px-5 py-3 hover:bg-ink-50/60 transition-colors cursor-pointer"
                @click="ui.openEditTransaction(t)">
                <span class="text-sm truncate">
                  <span v-if="t.type === App.TransactionType.TRANSFER" class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-ink-100 text-ink-500">
                    <app-icon name="transfer" :size="12" />
                    {{ App.t('Перевод') }}
                  </span>
                  <span v-else-if="finance.categoryById.get(t.categoryId ?? '')"
                    class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium"
                    :style="{ background: finance.categoryById.get(t.categoryId ?? '').color + '1a', color: finance.categoryById.get(t.categoryId ?? '').color }">
                    {{ App.t(finance.categoryById.get(t.categoryId ?? '').name) }}
                  </span>
                  <span v-else class="text-ink-300">—</span>
                </span>
                <span class="text-sm text-ink-500 truncate">{{ App.t(finance.accountById.get(t.accountId)?.name) }}</span>
                <span class="min-w-0 flex items-center gap-1.5 flex-wrap">
                  <span v-for="tag in tagsFor(t)" :key="tag.id"
                    class="inline-flex items-center px-1.5 py-0.5 rounded-full text-[11px] font-medium shrink-0"
                    :style="{ background: tag.color + '1a', color: tag.color }">{{ App.t(tag.name) }}</span>
                  <span v-if="t.memo" class="text-sm text-ink-400 truncate">{{ t.memo }}</span>
                </span>
                <span class="text-sm font-semibold text-right" :class="t.amount < 0 ? 'text-money-neg' : 'text-money-pos'">
                  {{ t.amount < 0 ? '−' : '+' }}{{ App.formatMoney(Math.abs(t.amount), finance.accountById.get(t.accountId)?.currency) }}
                </span>
                <span class="flex justify-end text-ink-300">
                  <app-icon name="edit" :size="15" />
                </span>
              </button>

              <button class="lg:hidden w-full text-left flex items-center gap-3 px-4 py-3 active:bg-ink-50 transition-colors cursor-pointer" @click="ui.openEditTransaction(t)">
                <span class="w-9 h-9 rounded-full flex items-center justify-center shrink-0 text-xs font-semibold"
                  :style="{ background: (finance.categoryById.get(t.categoryId ?? '')?.color ?? '#94a3b8') + '1a', color: finance.categoryById.get(t.categoryId ?? '')?.color ?? '#94a3b8' }">
                  {{ (t.type === App.TransactionType.TRANSFER ? App.t('Перевод') : (finance.categoryById.get(t.categoryId ?? '')?.name ?? '?')).slice(0, 1).toUpperCase() }}
                </span>
                <span class="min-w-0 flex-1">
                  <span class="flex items-center justify-between gap-2">
                    <span class="text-sm font-medium text-ink-900 truncate">{{ t.type === App.TransactionType.TRANSFER ? App.t('Перевод') : App.t(finance.categoryById.get(t.categoryId ?? '')?.name ?? 'Без категории') }}</span>
                    <span class="text-sm font-semibold shrink-0" :class="t.amount < 0 ? 'text-money-neg' : 'text-money-pos'">
                      {{ t.amount < 0 ? '−' : '+' }}{{ App.formatMoney(Math.abs(t.amount), finance.accountById.get(t.accountId)?.currency) }}
                    </span>
                  </span>
                  <span class="flex items-center justify-between gap-2 mt-0.5">
                    <span class="text-xs text-ink-400 truncate">
                      {{ t.memo || App.t(finance.accountById.get(t.accountId)?.name) }}
                    </span>
                  </span>
                </span>
              </button>
            </li>
          </ul>
        </div>

        <div v-if="totalPages > 1" class="flex items-center justify-between px-4 md:px-5 py-3 border-t border-ink-200">
          <button type="button" :disabled="page <= 1"
            class="px-3 py-1.5 rounded-lg border border-ink-200 text-sm font-medium text-ink-700 hover:bg-ink-50 disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
            @click="page -= 1">{{ App.t('Назад') }}</button>
          <span class="text-xs text-ink-500">{{ App.t('Страница {page} из {total}', { page, total: totalPages }) }}</span>
          <button type="button" :disabled="page >= totalPages"
            class="px-3 py-1.5 rounded-lg border border-ink-200 text-sm font-medium text-ink-700 hover:bg-ink-50 disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
            @click="page += 1">{{ App.t('Далее') }}</button>
        </div>
      </div>
    </div>
  `,
  data() {
    return {
      App,
      finance: App.financeStore,
      ui: App.uiStore,
      filters: {
        search: '',
        accountId: App.router.state.query.account ?? '',
        categoryId: '',
        tagId: '',
        direction: 'all',
        // Defaults to the current month — with potentially thousands of transactions, the backend
        // only ever returns what's actually asked for (see App.api.getTransactions/periodToDateRange).
        period: 'month',
        dateFrom: '',
        dateTo: '',
      },
      page: 1,
      pageSize: 100,
      rows: [],
      totalCount: 0,
      totalPages: 1,
      sumsByCurrency: {},
      loading: true,
      searchDebounce: null,
      requestToken: 0,
    }
  },
  computed: {
    groups() {
      const map = new Map()
      for (const t of this.rows) {
        if (!map.has(t.date)) map.set(t.date, [])
        map.get(t.date).push(t)
      }
      const useBase = !this.filters.accountId
      return [...map.entries()].map(([date, txs]) => ({
        date,
        txs,
        total: txs.reduce((s, t) => s + (useBase ? this.finance.amountInBase(t) : t.amount), 0),
      }))
    },
    resultCurrency() {
      return this.filters.accountId ? this.finance.accountById.get(this.filters.accountId)?.currency : this.finance.state.baseCurrency
    },
    // resultSum totals every matching transaction across every page (via the backend's
    // sumsByCurrency aggregate), not just the currently displayed page.
    resultSum() {
      if (this.filters.accountId) {
        return Object.values(this.sumsByCurrency).reduce((s, v) => s + v, 0)
      }
      return Object.entries(this.sumsByCurrency).reduce((s, [code, v]) => s + this.finance.toBase(v, code), 0)
    },
    hasActiveFilters() {
      return !!this.filters.search || !!this.filters.accountId || !!this.filters.categoryId || !!this.filters.tagId || this.filters.direction !== 'all' || this.filters.period !== 'month'
    },
    directionType() {
      if (this.filters.direction === 'income') return App.TransactionType.INCOME
      if (this.filters.direction === 'expense') return App.TransactionType.EXPENSE
      if (this.filters.direction === 'transfer') return App.TransactionType.TRANSFER
      return undefined
    },
    // A single string combining every filter except search — watched as one source so setting
    // several fields at once (e.g. resetFilters) triggers exactly one refetch, not one per field.
    nonSearchFiltersKey() {
      const f = this.filters
      return [f.accountId, f.categoryId, f.tagId, f.direction, f.period, f.dateFrom, f.dateTo].join('|')
    },
  },
  watch: {
    nonSearchFiltersKey() { this.onFiltersChanged() },
    'filters.search'() {
      clearTimeout(this.searchDebounce)
      this.searchDebounce = setTimeout(() => this.onFiltersChanged(), 350)
    },
    page() { this.refresh() },
    'finance.state.transactionsVersion'() { this.refresh() },
  },
  mounted() {
    this.refresh()
  },
  methods: {
    tagsFor(t) {
      return (t.tagIds || []).map((id) => this.finance.tagById.get(id)).filter(Boolean)
    },
    onFiltersChanged() {
      if (this.page !== 1) this.page = 1
      else this.refresh()
    },
    async refresh() {
      const token = ++this.requestToken
      this.loading = true
      const { dateFrom, dateTo } = App.periodToDateRange(this.filters.period, this.filters.dateFrom, this.filters.dateTo)
      try {
        const res = await App.api.getTransactions({
          dateFrom,
          dateTo,
          accountId: this.filters.accountId || undefined,
          categoryId: this.filters.categoryId || undefined,
          tagId: this.filters.tagId || undefined,
          type: this.directionType,
          search: this.filters.search.trim() || undefined,
          page: this.page,
          pageSize: this.pageSize,
        })
        if (token !== this.requestToken) return
        this.rows = res.transactions
        this.totalCount = res.totalCount
        this.totalPages = res.totalPages
        this.sumsByCurrency = res.sumsByCurrency
      } finally {
        if (token === this.requestToken) this.loading = false
      }
    },
    resetFilters() {
      this.filters.search = ''
      this.filters.accountId = ''
      this.filters.categoryId = ''
      this.filters.tagId = ''
      this.filters.direction = 'all'
      this.filters.period = 'month'
      this.filters.dateFrom = ''
      this.filters.dateTo = ''
    },
  },
}
