window.App = window.App || {};

App.TransactionModal = {
  components: { 'app-icon': App.AppIcon },
  template: `
    <Transition name="fade">
      <div v-if="ui.transactionModalOpen" class="fixed inset-0 z-50 bg-ink-950/40 flex items-end sm:items-center justify-center" @click.self="close">
        <Transition name="sheet" appear>
          <div v-if="ui.transactionModalOpen" class="bg-white w-full sm:max-w-md sm:rounded-2xl rounded-t-2xl shadow-xl max-h-[92vh] overflow-y-auto">
            <div class="flex items-center justify-between px-5 h-14 border-b border-ink-200 sticky top-0 bg-white">
              <h2 class="font-semibold text-ink-950">{{ App.t(isEditing ? 'Изменить операцию' : 'Новая операция') }}</h2>
              <button class="p-1.5 rounded-lg text-ink-500 hover:bg-ink-100 cursor-pointer" @click="close">
                <app-icon name="close" :size="18" />
              </button>
            </div>

            <form class="p-5 space-y-4" @submit.prevent="submit">
              <div class="flex rounded-lg bg-ink-100 p-1">
                <button type="button" class="flex-1 py-2 rounded-md text-sm font-medium transition-colors cursor-pointer"
                  :class="form.direction === 'expense' ? 'bg-white shadow-sm text-money-neg' : 'text-ink-500'"
                  @click="form.direction = 'expense'; form.categoryId = ''">{{ App.t('Расход') }}</button>
                <button type="button" class="flex-1 py-2 rounded-md text-sm font-medium transition-colors cursor-pointer"
                  :class="form.direction === 'income' ? 'bg-white shadow-sm text-money-pos' : 'text-ink-500'"
                  @click="form.direction = 'income'; form.categoryId = ''">{{ App.t('Доход') }}</button>
                <button type="button" class="flex-1 py-2 rounded-md text-sm font-medium transition-colors cursor-pointer"
                  :class="form.direction === 'transfer' ? 'bg-white shadow-sm text-ink-900' : 'text-ink-500'"
                  @click="form.direction = 'transfer'; form.categoryId = ''">{{ App.t('Перевод') }}</button>
              </div>

              <div>
                <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Сумма') }}</label>
                <input v-model="form.amount" type="number" min="0" step="1" required placeholder="0"
                  class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-lg font-semibold focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
              </div>

              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Дата') }}</label>
                  <input v-model="form.date" type="date" required
                    class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t(form.direction === 'transfer' ? 'Со счёта' : 'Счёт') }}</label>
                  <select v-model="form.accountId" required
                    class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500">
                    <option v-for="a in finance.state.accounts" :key="a.id" :value="a.id">{{ App.t(a.name) }}</option>
                  </select>
                </div>
              </div>

              <div v-if="form.direction === 'transfer'">
                <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('На счёт') }}</label>
                <select v-model="form.toAccountId" required
                  class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500">
                  <option v-for="a in transferTargetOptions" :key="a.id" :value="a.id">{{ App.t(a.name) }}</option>
                </select>
                <p v-if="transferTargetOptions.length === 0" class="text-xs text-money-neg mt-1">{{ App.t('Нужен ещё хотя бы один счёт для перевода') }}</p>
              </div>

              <div v-if="isCrossCurrency">
                <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Курс: 1 {from} = ? {to}', { from: fromAccount.currency, to: toAccount.currency }) }}</label>
                <input v-model="form.rate" type="number" min="0" step="0.0001" required placeholder="Например, 90"
                  class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
                <p v-if="suggestedRate && Number(form.rate) !== suggestedRate" class="text-xs text-brand-600 mt-1 cursor-pointer hover:underline" @click="form.rate = suggestedRate">
                  {{ App.t('Курс из настроек: 1 {from} = {rate} {to} — применить', { from: fromAccount.currency, rate: suggestedRate, to: toAccount.currency }) }}
                </p>
                <p class="text-xs text-ink-400 mt-1">
                  {{ App.t('Зачислится на «{name}»:', { name: App.t(toAccount.name) }) }} <span class="font-medium text-ink-700">{{ formatMoney(convertedAmount, toAccount.currency) }}</span>
                </p>
              </div>

              <div v-if="form.direction !== 'transfer'">
                <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Категория') }}</label>
                <select v-model="form.categoryId"
                  class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500">
                  <option value="">{{ App.t('Без категории') }}</option>
                  <option v-for="c in categoryOptions" :key="c.id" :value="c.id">{{ App.t(c.name) }}</option>
                </select>
              </div>

              <div>
                <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Заметка') }}</label>
                <input v-model="form.memo" type="text" :placeholder="App.t('Необязательно')"
                  class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
              </div>

              <div class="flex gap-2 pt-2">
                <button v-if="isEditing" type="button" :disabled="saving"
                  class="px-4 py-2.5 rounded-lg border border-ink-200 text-money-neg text-sm font-medium hover:bg-red-50 cursor-pointer disabled:opacity-50"
                  @click="remove">{{ App.t('Удалить') }}</button>
                <button type="submit" :disabled="saving"
                  class="flex-1 rounded-lg bg-brand-600 text-white text-sm font-medium py-2.5 hover:bg-brand-500 transition-colors cursor-pointer disabled:opacity-50">
                  {{ saving ? App.t('Сохранение…') : App.t(isEditing ? 'Сохранить' : 'Добавить') }}
                </button>
              </div>
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
      form: {
        date: new Date().toISOString().slice(0, 10),
        accountId: '',
        toAccountId: '',
        categoryId: '',
        memo: '',
        amount: '',
        rate: '',
        direction: 'expense',
      },
    }
  },
  computed: {
    isEditing() {
      return !!this.ui.editingTransaction
    },
    categoryOptions() {
      const type = this.form.direction === 'expense' ? App.CategoryType.EXPENSE : App.CategoryType.INCOME
      return this.finance.state.categories.filter((c) => c.type === type && (!c.archived || c.id === this.form.categoryId))
    },
    transferTargetOptions() {
      return this.finance.state.accounts.filter((a) => a.id !== this.form.accountId)
    },
    fromAccount() {
      return this.finance.accountById.get(this.form.accountId)
    },
    toAccount() {
      return this.finance.accountById.get(this.form.toAccountId)
    },
    isCrossCurrency() {
      return this.form.direction === 'transfer' && !!this.fromAccount && !!this.toAccount && this.fromAccount.currency !== this.toAccount.currency
    },
    convertedAmount() {
      return (Number(this.form.amount) || 0) * (Number(this.form.rate) || 0)
    },
    suggestedRate() {
      if (!this.isCrossCurrency) return null
      const fromCur = this.finance.currencyByCode.get(this.fromAccount.currency)
      const toCur = this.finance.currencyByCode.get(this.toAccount.currency)
      if (!fromCur?.rate || !toCur?.rate) return null
      return Math.round((fromCur.rate / toCur.rate) * 10000) / 10000
    },
  },
  watch: {
    'form.accountId'(val) {
      if (this.form.direction === 'transfer' && val === this.form.toAccountId) {
        this.form.toAccountId = ''
      } else if (this.form.direction === 'transfer' && !this.isEditing && this.suggestedRate) {
        this.form.rate = this.suggestedRate
      }
    },
    'form.toAccountId'(val) {
      if (!val || this.form.direction !== 'transfer' || this.isEditing) return
      if (this.suggestedRate) this.form.rate = this.suggestedRate
    },
    isCrossCurrency(val) {
      if (!val) this.form.rate = ''
    },
    'ui.transactionModalOpen'(open) {
      if (!open) return
      const editing = this.ui.editingTransaction
      if (editing && editing.type === 'transfer') {
        const isOutgoing = editing.amount < 0
        this.form = {
          date: editing.date,
          accountId: isOutgoing ? editing.accountId : editing.transferAccountId,
          toAccountId: isOutgoing ? editing.transferAccountId : editing.accountId,
          categoryId: '',
          memo: editing.memo,
          amount: Math.abs(editing.transferAmount ?? editing.amount),
          rate: editing.transferRate ?? 1,
          direction: 'transfer',
        }
      } else if (editing) {
        this.form = {
          date: editing.date,
          accountId: editing.accountId,
          toAccountId: '',
          categoryId: editing.categoryId ?? '',
          memo: editing.memo,
          amount: Math.abs(editing.amount),
          rate: '',
          direction: editing.amount < 0 ? 'expense' : 'income',
        }
      } else {
        this.form = {
          date: new Date().toISOString().slice(0, 10),
          accountId: this.finance.state.accounts[0]?.id ?? '',
          toAccountId: '',
          categoryId: '',
          memo: '',
          amount: '',
          rate: '',
          direction: 'expense',
        }
      }
    },
  },
  methods: {
    formatMoney: App.formatMoney,
    close() {
      App.uiStore.closeTransactionModal()
    },
    async submit() {
      const amountNum = Math.round(Number(this.form.amount))
      if (!amountNum || !this.form.accountId) return
      this.saving = true
      try {
        if (this.form.direction === 'transfer') {
          if (!this.form.toAccountId || this.form.toAccountId === this.form.accountId) return
          let rate = 1
          if (this.isCrossCurrency) {
            rate = Number(this.form.rate)
            if (!rate || rate <= 0) return
          }
          const payload = {
            fromAccountId: this.form.accountId,
            toAccountId: this.form.toAccountId,
            amount: Math.abs(amountNum),
            toAmount: Math.round(Math.abs(amountNum) * rate),
            rate,
            date: this.form.date,
            memo: this.form.memo.trim(),
          }
          if (this.isEditing && this.ui.editingTransaction.type === 'transfer') {
            await this.finance.updateTransfer(this.ui.editingTransaction.id, payload)
          } else {
            await this.finance.addTransfer(payload)
          }
        } else {
          const signedAmount = this.form.direction === 'expense' ? -Math.abs(amountNum) : Math.abs(amountNum)
          if (this.isEditing && this.ui.editingTransaction.type !== 'transfer') {
            await this.finance.updateTransaction({
              ...this.ui.editingTransaction,
              date: this.form.date,
              accountId: this.form.accountId,
              categoryId: this.form.categoryId || null,
              memo: this.form.memo.trim(),
              amount: signedAmount,
            })
          } else if (this.isEditing) {
            await this.finance.deleteTransfer(this.ui.editingTransaction.id)
            await this.finance.addTransaction({
              date: this.form.date,
              accountId: this.form.accountId,
              categoryId: this.form.categoryId || null,
              memo: this.form.memo.trim(),
              amount: signedAmount,
            })
          } else {
            await this.finance.addTransaction({
              date: this.form.date,
              accountId: this.form.accountId,
              categoryId: this.form.categoryId || null,
              memo: this.form.memo.trim(),
              amount: signedAmount,
            })
          }
        }
        this.close()
      } finally {
        this.saving = false
      }
    },
    async remove() {
      if (!this.ui.editingTransaction) return
      this.saving = true
      if (this.ui.editingTransaction.type === 'transfer') {
        try {
          await this.finance.deleteTransfer(this.ui.editingTransaction.id)
          this.close()
        } finally {
          this.saving = false
        }
        return
      }
      try {
        await this.finance.deleteTransaction(this.ui.editingTransaction.id)
        this.close()
      } finally {
        this.saving = false
      }
    },
  },
}
