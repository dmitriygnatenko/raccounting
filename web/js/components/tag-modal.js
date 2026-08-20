window.App = window.App || {};

;(function () {
const PALETTE = [
  '#f59e0b', '#3b82f6', '#8b5cf6', '#f43f5e', '#14b8a6', '#06b6d4', '#ec4899',
  '#fb923c', '#6366f1', '#64748b', '#22c55e', '#84cc16', '#a3e635', '#4ade80',
]

App.TagModal = {
  components: { 'app-icon': App.AppIcon },
  template: `
    <Transition name="fade">
      <div v-if="ui.tagModalOpen" class="fixed inset-0 z-50 bg-ink-950/40 flex items-end sm:items-center justify-center" @click.self="close">
        <Transition name="sheet" appear>
          <div v-if="ui.tagModalOpen" class="bg-white w-full sm:max-w-sm sm:rounded-2xl rounded-t-2xl shadow-xl max-h-[92vh] overflow-y-auto">
            <div class="flex items-center justify-between px-5 h-14 border-b border-ink-200 sticky top-0 bg-white">
              <h2 class="font-semibold text-ink-950">{{ App.t(isEditing ? 'Изменить тег' : 'Новый тег') }}</h2>
              <button class="p-1.5 rounded-lg text-ink-500 hover:bg-ink-100 cursor-pointer" @click="close">
                <app-icon name="close" :size="18" />
              </button>
            </div>

            <form class="p-5 space-y-4" @submit.prevent="submit">
              <div>
                <label class="block text-xs font-medium text-ink-500 mb-1">{{ App.t('Название') }}</label>
                <input v-model="form.name" type="text" required :placeholder="App.t('Например, Отпуск')"
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
                <button type="button" :disabled="saving" :title="App.t('Удалить тег безвозвратно')"
                  class="flex-1 px-3 py-2.5 rounded-lg border border-ink-200 text-money-neg text-sm font-medium hover:bg-red-50 cursor-pointer disabled:opacity-50"
                  @click="remove">{{ App.t('Удалить') }}</button>
              </div>

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
      form: { name: '', color: PALETTE[0] },
    }
  },
  computed: {
    isEditing() {
      return !!this.ui.editingTag
    },
  },
  watch: {
    'ui.tagModalOpen'(open) {
      if (!open) return
      const editing = this.ui.editingTag
      this.form = editing
        ? { name: editing.name, color: editing.color }
        : { name: '', color: PALETTE[Math.floor(Math.random() * PALETTE.length)] }
    },
  },
  methods: {
    close() {
      App.uiStore.closeTagModal()
    },
    async submit() {
      const name = this.form.name.trim()
      if (!name) return
      this.saving = true
      try {
        if (this.isEditing) {
          await this.finance.updateTag({ ...this.ui.editingTag, name, color: this.form.color })
        } else {
          await this.finance.addTag({ name, color: this.form.color })
        }
        this.close()
      } finally {
        this.saving = false
      }
    },
    async remove() {
      if (!this.ui.editingTag) return
      this.saving = true
      try {
        await this.finance.deleteTag(this.ui.editingTag.id)
        this.close()
      } finally {
        this.saving = false
      }
    },
  },
}
})();
