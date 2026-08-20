window.App = window.App || {};

;(function () {
const PALETTE = [
  '#f59e0b', '#3b82f6', '#8b5cf6', '#f43f5e', '#14b8a6', '#06b6d4', '#ec4899',
  '#fb923c', '#6366f1', '#64748b', '#22c55e', '#84cc16', '#a3e635', '#4ade80',
]

App.CategoryModal = {
  components: { 'app-icon': App.AppIcon },
  template: `
    <Transition name="fade">
      <div v-if="ui.categoryModalOpen" class="fixed inset-0 z-50 bg-ink-950/40 flex items-end sm:items-center justify-center" @click.self="close">
        <Transition name="sheet" appear>
          <div v-if="ui.categoryModalOpen" class="bg-white w-full sm:max-w-sm sm:rounded-2xl rounded-t-2xl shadow-xl max-h-[92vh] overflow-y-auto">
            <div class="flex items-center justify-between px-5 h-14 border-b border-ink-200 sticky top-0 bg-white">
              <h2 class="font-semibold text-ink-950">{{ App.t(isEditing ? 'Изменить категорию' : 'Новая категория') }}</h2>
              <button class="p-1.5 rounded-lg text-ink-500 hover:bg-ink-100 cursor-pointer" @click="close">
                <app-icon name="close" :size="18" />
              </button>
            </div>

            <form class="p-5 space-y-4" @submit.prevent="submit">
              <div class="flex rounded-lg bg-ink-100 p-1">
                <button type="button" class="flex-1 py-2 rounded-md text-sm font-medium transition-colors cursor-pointer"
                  :class="form.type === App.CategoryType.EXPENSE ? 'bg-white shadow-sm text-money-neg' : 'text-ink-500'"
                  @click="form.type = App.CategoryType.EXPENSE">{{ App.t('Расход') }}</button>
                <button type="button" class="flex-1 py-2 rounded-md text-sm font-medium transition-colors cursor-pointer"
                  :class="form.type === App.CategoryType.INCOME ? 'bg-white shadow-sm text-money-pos' : 'text-ink-500'"
                  @click="form.type = App.CategoryType.INCOME">{{ App.t('Доход') }}</button>
              </div>

              <div>
                <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Название') }}</label>
                <input v-model="form.name" type="text" required :placeholder="App.t('Например, Хобби')"
                  class="w-full rounded-lg border border-ink-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/40 focus:border-brand-500" />
              </div>

              <div>
                <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Цвет') }}</label>
                <div class="flex flex-wrap gap-2">
                  <button v-for="c in palette" :key="c" type="button" @click="form.color = c"
                    class="w-7 h-7 rounded-full cursor-pointer transition-transform"
                    :class="form.color === c ? 'ring-2 ring-offset-2 ring-ink-900 scale-105' : ''"
                    :style="{ background: c }"></button>
                </div>
              </div>

              <div v-if="isEditing" class="flex gap-2">
                <button v-if="!ui.editingCategory.archived" type="button" :disabled="saving" :title="App.t('Скрыть из выбора категории при новых операциях')"
                  class="flex-1 px-3 py-2.5 rounded-lg border border-ink-200 text-ink-600 text-sm font-medium hover:bg-ink-50 cursor-pointer disabled:opacity-50"
                  @click="archive">{{ App.t('Деактивировать') }}</button>
                <button v-else type="button" :disabled="saving" :title="App.t('Вернуть в выбор категории при новых операциях')"
                  class="flex-1 px-3 py-2.5 rounded-lg border border-ink-200 text-ink-600 text-sm font-medium hover:bg-ink-50 cursor-pointer disabled:opacity-50"
                  @click="unarchive">{{ App.t('Активировать') }}</button>
                <button v-if="!inUse" type="button" :disabled="saving" :title="App.t('Удалить категорию безвозвратно')"
                  class="flex-1 px-3 py-2.5 rounded-lg border border-ink-200 text-money-neg text-sm font-medium hover:bg-red-50 cursor-pointer disabled:opacity-50"
                  @click="remove">{{ App.t('Удалить') }}</button>
              </div>
              <p v-if="isEditing && inUse" class="text-xs text-ink-400 text-center">{{ App.t('Используется в операциях — удалить нельзя, можно деактивировать') }}</p>

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
      palette: PALETTE,
      form: { name: '', type: App.CategoryType.EXPENSE, color: PALETTE[0] },
    }
  },
  computed: {
    isEditing() {
      return !!this.ui.editingCategory
    },
    inUse() {
      return this.isEditing && this.finance.isCategoryInUse(this.ui.editingCategory.id)
    },
  },
  watch: {
    'ui.categoryModalOpen'(open) {
      if (!open) return
      const editing = this.ui.editingCategory
      this.form = editing
        ? { name: editing.name, type: editing.type, color: editing.color }
        : { name: '', type: this.ui.newCategoryType || App.CategoryType.EXPENSE, color: PALETTE[0] }
    },
  },
  methods: {
    close() {
      App.uiStore.closeCategoryModal()
    },
    async submit() {
      const name = this.form.name.trim()
      if (!name) return
      this.saving = true
      try {
        if (this.isEditing) {
          await this.finance.updateCategory({ ...this.ui.editingCategory, name, type: this.form.type, color: this.form.color })
        } else {
          await this.finance.addCategory({ name, type: this.form.type, color: this.form.color })
        }
        this.close()
      } finally {
        this.saving = false
      }
    },
    async archive() {
      if (!this.ui.editingCategory) return
      this.saving = true
      try {
        await this.finance.archiveCategory(this.ui.editingCategory.id)
        this.close()
      } finally {
        this.saving = false
      }
    },
    async unarchive() {
      if (!this.ui.editingCategory) return
      this.saving = true
      try {
        await this.finance.unarchiveCategory(this.ui.editingCategory.id)
        this.close()
      } finally {
        this.saving = false
      }
    },
    async remove() {
      if (!this.ui.editingCategory) return
      this.saving = true
      try {
        await this.finance.deleteCategory(this.ui.editingCategory.id)
        this.close()
      } finally {
        this.saving = false
      }
    },
  },
}
})();
