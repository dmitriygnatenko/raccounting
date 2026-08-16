window.App = window.App || {};

App.AccountModal = {
  components: { 'app-icon': App.AppIcon },
  template: `
    <Transition name="fade">
      <div v-if="ui.accountModalOpen" class="fixed inset-0 z-50 bg-ink-950/40 flex items-end sm:items-center justify-center" @click.self="close">
        <Transition name="sheet" appear>
          <div v-if="ui.accountModalOpen" class="bg-white w-full sm:max-w-md sm:rounded-2xl rounded-t-2xl shadow-xl max-h-[92vh] overflow-y-auto">
            <div class="flex items-center justify-between px-5 h-14 border-b border-ink-200 sticky top-0 bg-white">
              <h2 class="font-semibold text-ink-950">{{ App.t(isEditing ? 'Изменить счёт' : 'Новый счёт') }}</h2>
              <button class="p-1.5 rounded-lg text-ink-500 hover:bg-ink-100 cursor-pointer" @click="close">
                <app-icon name="close" :size="18" />
              </button>
            </div>

            <form class="p-5 space-y-4" @submit.prevent="submit">
              <div>
                <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Название') }}</label>
                <input v-model="form.name" type="text" required :placeholder="App.t('Например, Дебетовая карта')"
                  class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
              </div>

              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Тип') }}</label>
                  <select v-model="form.type" required
                    class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500">
                    <option v-for="t in typeOptions" :key="t.value" :value="t.value">{{ App.t(t.label) }}</option>
                  </select>
                </div>
                <div>
                  <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Валюта') }}</label>
                  <select v-model="form.currency" required :disabled="noCurrencies"
                    class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500 disabled:opacity-50">
                    <option v-for="c in currencyOptions" :key="c" :value="c">{{ c }}</option>
                  </select>
                </div>
              </div>

              <p v-if="noCurrencies" class="text-xs text-money-neg">{{ App.t('Сначала добавьте валюту в настройках') }}</p>

              <div v-if="!isEditing">
                <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Начальный баланс') }}</label>
                <input v-model="form.balance" type="number" step="1" placeholder="0"
                  class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-lg font-semibold focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
              </div>
              <div v-else>
                <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Баланс') }}</label>
                <p class="w-full rounded-lg border border-ink-200 bg-ink-50 px-3 py-2.5 text-lg font-semibold text-ink-900">
                  {{ App.formatMoney(ui.editingAccount ? ui.editingAccount.balance : 0, form.currency) }}
                </p>
                <p class="text-xs text-ink-400 mt-1">{{ App.t('Баланс меняется через операции и переводы, а не редактируется напрямую') }}</p>
              </div>

              <div v-if="isEditing" class="flex gap-2">
                <button v-if="!ui.editingAccount.archived" type="button" :disabled="saving" :title="App.t('Скрыть из активных счетов, история операций сохранится')"
                  class="flex-1 px-3 py-2.5 rounded-lg border border-ink-200 text-ink-600 text-sm font-medium hover:bg-ink-50 cursor-pointer disabled:opacity-50"
                  @click="archive">{{ App.t('Деактивировать') }}</button>
                <button v-if="!inUse" type="button" :disabled="saving" :title="App.t('Удалить счёт безвозвратно')"
                  class="flex-1 px-3 py-2.5 rounded-lg border border-ink-200 text-money-neg text-sm font-medium hover:bg-red-50 cursor-pointer disabled:opacity-50"
                  @click="remove">{{ App.t('Удалить') }}</button>
              </div>
              <p v-if="isEditing && inUse" class="text-xs text-ink-400 text-center">{{ App.t('По счёту есть операции — удалить нельзя, можно деактивировать') }}</p>

              <button type="submit" :disabled="saving || noCurrencies"
                class="w-full rounded-lg bg-brand-600 text-white text-sm font-medium py-2.5 hover:bg-brand-500 transition-colors cursor-pointer disabled:opacity-50">
                {{ saving ? App.t('Сохранение…') : App.t(isEditing ? 'Сохранить' : 'Добавить') }}
              </button>
            </form>
          </div>
        </Transition>
      </div>
    </Transition>
  `,
  data() {
    return {
      App,
      finance: App.financeStore,
      ui: App.uiStore.state,
      saving: false,
      typeOptions: Object.keys(App.accountTypeLabel).map((value) => ({ value, label: App.accountTypeLabel[value] })),
      form: this.emptyForm(),
    }
  },
  computed: {
    isEditing() {
      return !!this.ui.editingAccount
    },
    inUse() {
      return this.isEditing && this.finance.isAccountInUse(this.ui.editingAccount.id)
    },
    currencyOptions() {
      const codes = this.finance.state.currencies.filter((c) => !c.archived).map((c) => c.code)
      if (this.form.currency && !codes.includes(this.form.currency)) codes.push(this.form.currency)
      return codes
    },
    noCurrencies() {
      return !this.isEditing && this.finance.state.currencies.filter((c) => !c.archived).length === 0
    },
  },
  watch: {
    'ui.accountModalOpen'(open) {
      if (!open) return
      const editing = this.ui.editingAccount
      this.form = editing
        ? {
            name: editing.name,
            type: editing.type,
            currency: editing.currency,
            balance: editing.balance,
          }
        : this.emptyForm()
    },
  },
  methods: {
    emptyForm() {
      return { name: '', type: 'card', currency: 'RUB', balance: '' }
    },
    close() {
      App.uiStore.closeAccountModal()
    },
    async submit() {
      if (!this.form.name.trim() || this.noCurrencies) return
      this.saving = true
      const payload = {
        name: this.form.name.trim(),
        type: this.form.type,
        currency: this.form.currency,
        balance: Math.round(Number(this.form.balance) || 0),
      }
      try {
        if (this.isEditing) {
          await this.finance.updateAccount({ ...this.ui.editingAccount, ...payload })
        } else {
          await this.finance.addAccount(payload)
        }
        this.close()
      } finally {
        this.saving = false
      }
    },
    async archive() {
      if (!this.ui.editingAccount) return
      this.saving = true
      try {
        await this.finance.archiveAccount(this.ui.editingAccount.id)
        this.close()
      } finally {
        this.saving = false
      }
    },
    async remove() {
      if (!this.ui.editingAccount) return
      this.saving = true
      try {
        await this.finance.deleteAccount(this.ui.editingAccount.id)
        this.close()
      } finally {
        this.saving = false
      }
    },
  },
}
