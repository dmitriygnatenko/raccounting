window.App = window.App || {};

App.AccountsView = {
  components: { 'app-icon': App.AppIcon, 'skeleton-block': App.SkeletonBlock },
  template: `
    <div v-if="finance.state.loading" class="space-y-4">
      <skeleton-block class="h-16" />
      <skeleton-block class="h-48" />
      <skeleton-block class="h-48" />
    </div>

    <div v-else class="space-y-5">
      <div class="rounded-xl bg-white border border-ink-200 p-4 md:p-5">
        <p class="text-xs font-medium text-ink-500 mb-1">{{ App.t('Итого по счетам, {currency}', { currency: finance.state.baseCurrency }) }}</p>
        <p class="text-2xl font-semibold text-ink-950">{{ App.formatMoney(finance.totalBalanceBase, finance.state.baseCurrency) }}</p>
        <p class="text-sm text-ink-400 mt-0.5">{{ App.tCount(finance.activeAccounts.length, 'accounts') }}</p>
      </div>

      <div v-for="[groupName, groupAccounts] in groups" :key="groupName" class="rounded-xl bg-white border border-ink-200 overflow-hidden">
        <div class="px-4 md:px-5 py-3 border-b border-ink-200 bg-ink-50/60">
          <h2 class="text-sm font-semibold text-ink-700">{{ App.t(groupName) }}</h2>
        </div>
        <ul class="divide-y divide-ink-100">
          <li v-for="a in groupAccounts" :key="a.id" class="flex items-center justify-between gap-3 px-4 md:px-5 py-3.5 hover:bg-ink-50/60 transition-colors">
            <a :href="'#/transactions?account=' + a.id" class="flex items-center gap-3 min-w-0 flex-1">
              <span class="w-10 h-10 rounded-xl bg-brand-100 text-brand-600 flex items-center justify-center shrink-0">
                <app-icon :name="App.accountIcon[a.type]" :size="18" />
              </span>
              <span class="min-w-0">
                <span class="block text-sm font-medium text-ink-900 truncate">{{ App.t(a.name) }}</span>
                <span class="block text-xs text-ink-400 truncate">{{ App.t(App.accountTypeLabel[a.type]) }}</span>
              </span>
            </a>
            <span class="text-sm md:text-base font-semibold shrink-0" :class="a.balance < 0 ? 'text-money-neg' : 'text-ink-950'">
              {{ App.formatMoney(a.balance, a.currency) }}
            </span>
          </li>
        </ul>
      </div>
    </div>
  `,
  data() {
    return { App, finance: App.financeStore }
  },
  computed: {
    groups() {
      const map = new Map()
      for (const a of this.finance.activeAccounts) {
        const label = App.accountTypeLabel[a.type] ?? 'Другое'
        if (!map.has(label)) map.set(label, [])
        map.get(label).push(a)
      }
      return [...map.entries()]
    },
  },
}
