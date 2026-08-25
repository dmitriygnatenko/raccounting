window.App = window.App || {};

const LAST_ACCOUNT_KEY = 'raccounting.lastAccountId'

function getStoredAccountId() {
  try {
    return localStorage.getItem(LAST_ACCOUNT_KEY) || null
  } catch {
    return null
  }
}

function setStoredAccountId(id) {
  try {
    if (id) localStorage.setItem(LAST_ACCOUNT_KEY, id)
  } catch {}
}

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
                <input v-model="form.amount" type="number" min="0" :max="App.MAX_AMOUNT" step="1" required placeholder="0"
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
                    <option v-for="a in accountOptions" :key="a.id" :value="a.id">{{ App.t(a.name) }}</option>
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

              <div v-if="form.direction !== 'transfer'" class="relative">
                <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Теги') }}</label>
                <div class="w-full rounded-lg border border-ink-200 px-2 py-1.5 flex flex-wrap items-center gap-1.5 focus-within:ring-2 focus-within:ring-brand-500/40 focus-within:border-brand-500">
                  <span v-for="tag in selectedTags" :key="tag.id"
                    class="inline-flex items-center gap-1 pl-2 pr-1 py-0.5 rounded-full text-xs font-medium"
                    :style="{ background: tag.color + '1a', color: tag.color }">
                    {{ App.t(tag.name) }}
                    <button type="button" class="hover:opacity-70 cursor-pointer" @click="removeTag(tag.id)">
                      <app-icon name="close" :size="11" />
                    </button>
                  </span>
                  <input v-model="tagInput" type="text" :placeholder="selectedTags.length ? '' : App.t('Добавить тег…')"
                    class="flex-1 min-w-[6rem] py-1 text-sm focus:outline-none"
                    @keydown.enter.prevent="onTagEnter" @keydown.backspace="onTagBackspace" />
                </div>
                <ul v-if="tagInput.trim() && tagSuggestions.length" class="absolute z-10 mt-1 w-full max-h-40 overflow-y-auto rounded-lg border border-ink-200 bg-white shadow-lg py-1">
                  <li v-for="tag in tagSuggestions" :key="tag.id">
                    <button type="button" class="w-full text-left px-3 py-1.5 text-sm hover:bg-ink-50 cursor-pointer flex items-center gap-2"
                      @click="selectTag(tag)">
                      <span class="w-2 h-2 rounded-full shrink-0" :style="{ background: tag.color }"></span>
                      {{ App.t(tag.name) }}
                    </button>
                  </li>
                </ul>
                <p v-else-if="tagInput.trim()" class="absolute z-10 mt-1 w-full rounded-lg border border-ink-200 bg-white shadow-lg py-1.5 px-3 text-xs text-ink-400">
                  {{ App.t('Enter — создать тег «{name}»', { name: tagInput.trim() }) }}
                </p>
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
      tagInput: '',
      form: {
        date: new Date().toISOString().slice(0, 10),
        accountId: '',
        toAccountId: '',
        categoryId: '',
        memo: '',
        amount: '',
        rate: '',
        direction: 'expense',
        tagIds: [],
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
    accountOptions() {
      return this.finance.state.accounts.filter((a) => !a.archived || a.id === this.form.accountId)
    },
    selectedTags() {
      return this.form.tagIds.map((id) => this.finance.tagById.get(id)).filter(Boolean)
    },
    tagSuggestions() {
      const search = this.tagInput.trim().toLowerCase()
      if (!search) return []
      return this.finance.state.tags
        .filter((t) => !this.form.tagIds.includes(t.id))
        .filter((t) => t.name.toLowerCase().includes(search))
        .slice(0, 8)
    },
    transferTargetOptions() {
      return this.finance.state.accounts.filter((a) => a.id !== this.form.accountId && (!a.archived || a.id === this.form.toAccountId))
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
      this.tagInput = ''
      const editing = this.ui.editingTransaction
      if (editing && editing.type === App.TransactionType.TRANSFER) {
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
          tagIds: [],
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
          tagIds: [...(editing.tagIds ?? [])],
        }
      } else {
        const storedAccountId = getStoredAccountId()
        const activeAccounts = this.finance.state.accounts.filter((a) => !a.archived)
        const storedAccount = activeAccounts.find((a) => String(a.id) === storedAccountId)
        const defaultAccountId = storedAccount ? storedAccount.id : activeAccounts[0]?.id ?? ''
        this.form = {
          date: new Date().toISOString().slice(0, 10),
          accountId: defaultAccountId,
          toAccountId: '',
          categoryId: '',
          memo: '',
          amount: '',
          rate: '',
          direction: 'expense',
          tagIds: [],
        }
      }
    },
  },
  methods: {
    formatMoney: App.formatMoney,
    close() {
      App.uiStore.closeTransactionModal()
    },
    removeTag(id) {
      this.form.tagIds = this.form.tagIds.filter((tagId) => tagId !== id)
    },
    selectTag(tag) {
      if (!this.form.tagIds.includes(tag.id)) this.form.tagIds.push(tag.id)
      this.tagInput = ''
    },
    async onTagEnter() {
      const name = this.tagInput.trim()
      if (!name) return
      const existing = this.finance.state.tags.find((t) => t.name.toLowerCase() === name.toLowerCase())
      if (existing) {
        this.selectTag(existing)
        return
      }
      const palette = ['#f59e0b', '#3b82f6', '#8b5cf6', '#f43f5e', '#14b8a6', '#06b6d4', '#ec4899', '#fb923c', '#6366f1', '#64748b', '#22c55e', '#84cc16']
      const color = palette[this.finance.state.tags.length % palette.length]
      const created = await this.finance.addTag({ name, color })
      this.selectTag(created)
    },
    onTagBackspace() {
      if (this.tagInput || !this.form.tagIds.length) return
      this.form.tagIds = this.form.tagIds.slice(0, -1)
    },
    async submit() {
      const amountNum = Math.round(Number(this.form.amount))
      if (!amountNum || !this.form.accountId) return
      const wasEditing = this.isEditing
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
          if (this.isEditing && this.ui.editingTransaction.type === App.TransactionType.TRANSFER) {
            await this.finance.updateTransfer(this.ui.editingTransaction.id, payload)
          } else {
            await this.finance.addTransfer(payload)
          }
        } else {
          const signedAmount = this.form.direction === 'expense' ? -Math.abs(amountNum) : Math.abs(amountNum)
          if (this.isEditing && this.ui.editingTransaction.type !== App.TransactionType.TRANSFER) {
            await this.finance.updateTransaction({
              ...this.ui.editingTransaction,
              date: this.form.date,
              accountId: this.form.accountId,
              categoryId: this.form.categoryId || null,
              memo: this.form.memo.trim(),
              amount: signedAmount,
              tagIds: this.form.tagIds,
            })
          } else if (this.isEditing) {
            await this.finance.deleteTransfer(this.ui.editingTransaction.id)
            await this.finance.addTransaction({
              date: this.form.date,
              accountId: this.form.accountId,
              categoryId: this.form.categoryId || null,
              memo: this.form.memo.trim(),
              amount: signedAmount,
              tagIds: this.form.tagIds,
            })
          } else {
            await this.finance.addTransaction({
              date: this.form.date,
              accountId: this.form.accountId,
              categoryId: this.form.categoryId || null,
              memo: this.form.memo.trim(),
              amount: signedAmount,
              tagIds: this.form.tagIds,
            })
          }
        }
        if (!wasEditing) setStoredAccountId(this.form.accountId)
        this.close()
      } finally {
        this.saving = false
      }
    },
    async remove() {
      if (!this.ui.editingTransaction) return
      this.saving = true
      if (this.ui.editingTransaction.type === App.TransactionType.TRANSFER) {
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
