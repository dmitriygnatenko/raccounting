window.App = window.App || {};

App.CurrencyModal = {
  components: { 'app-icon': App.AppIcon },
  template: `
    <Transition name="fade">
      <div v-if="ui.currencyModalOpen" class="fixed inset-0 z-50 bg-ink-950/40 flex items-end sm:items-center justify-center" @click.self="close">
        <Transition name="sheet" appear>
          <div v-if="ui.currencyModalOpen" class="bg-white w-full sm:max-w-sm sm:rounded-2xl rounded-t-2xl shadow-xl max-h-[92vh] overflow-y-auto">
            <div class="flex items-center justify-between px-5 h-14 border-b border-ink-200 sticky top-0 bg-white">
              <h2 class="font-semibold text-ink-950">{{ App.t(isEditing ? 'Изменить валюту' : 'Новая валюта') }}</h2>
              <button class="p-1.5 rounded-lg text-ink-500 hover:bg-ink-100 cursor-pointer" @click="close">
                <app-icon name="close" :size="18" />
              </button>
            </div>

            <form class="p-5 space-y-4" @submit.prevent="submit">
              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Код') }}</label>
                  <input v-model="form.code" type="text" required maxlength="3" minlength="3" :placeholder="App.t('Например, GBP')" :disabled="isEditing"
                    class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-sm uppercase focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500 disabled:bg-ink-50 disabled:text-ink-400" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Символ') }}</label>
                  <input v-model="form.symbol" type="text" required maxlength="4" :placeholder="App.t('Например, £')"
                    class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
                </div>
              </div>

              <div>
                <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Название') }}</label>
                <input v-model="form.name" type="text" required :placeholder="App.t('Например, Фунт стерлингов')"
                  class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
              </div>

              <div v-if="isBase">
                <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Курс к {base}', { base: baseCode }) }}</label>
                <div class="w-full rounded-lg border border-ink-200 bg-ink-50 px-3 py-2.5 text-sm text-ink-400">{{ App.t('Базовая валюта, курс 1') }}</div>
              </div>
              <div v-else>
                <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Курс к {base}: 1 {code} = ? {symbol}', { base: baseCode, code: form.code || '...', symbol: baseSymbol }) }}</label>
                <input v-model="form.rate" type="number" min="0" step="0.0001" required placeholder="Например, 90"
                  class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
              </div>

              <p v-if="error" class="text-sm text-money-neg">{{ error }}</p>

              <div v-if="isEditing" class="flex gap-2">
                <button v-if="!ui.editingCurrency.archived" type="button" :disabled="saving" :title="App.t('Скрыть из выбора при создании счетов')"
                  class="flex-1 px-3 py-2.5 rounded-lg border border-ink-200 text-ink-600 text-sm font-medium hover:bg-ink-50 cursor-pointer disabled:opacity-50"
                  @click="archive">{{ App.t('Деактивировать') }}</button>
                <button v-else type="button" :disabled="saving" :title="App.t('Вернуть в выбор при создании счетов')"
                  class="flex-1 px-3 py-2.5 rounded-lg border border-ink-200 text-ink-600 text-sm font-medium hover:bg-ink-50 cursor-pointer disabled:opacity-50"
                  @click="unarchive">{{ App.t('Активировать') }}</button>
                <button v-if="!inUse" type="button" :disabled="saving" :title="App.t('Удалить валюту безвозвратно')"
                  class="flex-1 px-3 py-2.5 rounded-lg border border-ink-200 text-money-neg text-sm font-medium hover:bg-red-50 cursor-pointer disabled:opacity-50"
                  @click="remove">{{ App.t('Удалить') }}</button>
              </div>
              <p v-if="isEditing && inUse" class="text-xs text-ink-400 text-center">{{ App.t('Валюта используется в счетах — удалить нельзя, можно деактивировать') }}</p>

              <button type="submit" :disabled="saving"
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
      error: '',
      form: this.emptyForm(),
    }
  },
  computed: {
    isEditing() {
      return !!this.ui.editingCurrency
    },
    inUse() {
      if (!this.isEditing) return false
      return this.finance.isCurrencyInUse(this.ui.editingCurrency.code)
    },
    baseCode() {
      return this.finance.state.baseCurrency
    },
    baseSymbol() {
      return this.finance.currencyByCode.get(this.baseCode)?.symbol ?? this.baseCode
    },
    isBase() {
      return this.form.code.trim().toUpperCase() === this.baseCode
    },
  },
  watch: {
    'ui.currencyModalOpen'(open) {
      if (!open) return
      this.error = ''
      const editing = this.ui.editingCurrency
      this.form = editing
        ? { code: editing.code, symbol: editing.symbol, name: editing.name, rate: editing.rate ?? 1 }
        : this.emptyForm()
    },
  },
  methods: {
    emptyForm() {
      return { code: '', symbol: '', name: '', rate: '' }
    },
    close() {
      App.uiStore.closeCurrencyModal()
    },
    async submit() {
      const code = this.form.code.trim().toUpperCase()
      const symbol = this.form.symbol.trim()
      const name = this.form.name.trim()
      if (!code || !symbol || !name) return
      if (!this.isEditing && this.finance.state.currencies.some((c) => c.code === code)) {
        this.error = App.t('Такой код валюты уже есть')
        return
      }
      let rate = 1
      if (code !== this.baseCode) {
        rate = Number(this.form.rate)
        if (!rate || rate <= 0) {
          this.error = App.t('Укажите курс к {base}', { base: this.baseCode })
          return
        }
      }
      this.error = ''
      this.saving = true
      try {
        if (this.isEditing) {
          await this.finance.updateCurrency({ code, symbol, name, rate, default: this.ui.editingCurrency.is_default, archived: this.ui.editingCurrency.archived })
        } else {
          await this.finance.addCurrency({ code, symbol, name, rate })
        }
        this.close()
      } finally {
        this.saving = false
      }
    },
    async archive() {
      if (!this.ui.editingCurrency) return
      this.saving = true
      try {
        await this.finance.archiveCurrency(this.ui.editingCurrency.code)
        this.close()
      } finally {
        this.saving = false
      }
    },
    async unarchive() {
      if (!this.ui.editingCurrency) return
      this.saving = true
      try {
        await this.finance.unarchiveCurrency(this.ui.editingCurrency.code)
        this.close()
      } finally {
        this.saving = false
      }
    },
    async remove() {
      if (!this.ui.editingCurrency) return
      this.saving = true
      try {
        await this.finance.deleteCurrency(this.ui.editingCurrency.code)
        this.close()
      } finally {
        this.saving = false
      }
    },
  },
}
