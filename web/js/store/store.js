window.App = window.App || {};

;(function () {
const { reactive, computed, watch } = Vue

// currentMonthKey is 'YYYY-MM' for the current calendar month, as used for budgetMonth throughout
// the app (see App.formatMonthYear and the Отчёты/Бюджет views).
function currentMonthKey() {
  const now = new Date()
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
}

function createFinanceStore() {
  const state = reactive({
    accounts: [],
    categories: [],
    currencies: [],
    tags: [],
    baseCurrency: 'RUB',
    categoryBudgets: {},
    usedAccountIds: new Set(),
    usedCategoryIds: new Set(),
    // Bumped by every mutation that can add/remove/change a transaction — views that keep their own
    // bounded window of transactions (Операции/Отчёты/Обзор) watch this to know when to refetch,
    // since transactions are no longer cached in one global array here (see the module doc comment).
    transactionsVersion: 0,
    loading: true,
    loaded: false,
    // Categories over their current-month budget — drives the bell notification badge (see
    // App.notificationsStore, which tracks which of these the user has already seen).
    budgetAlerts: [],
  })

  async function refreshUsage() {
    const usage = await App.api.getTransactionsUsage()
    state.usedAccountIds = new Set(usage.accountIds)
    state.usedCategoryIds = new Set(usage.categoryIds)
  }

  async function load() {
    if (state.loaded) return
    state.loading = true
    const [a, c, cur, budgets, tags] = await Promise.all([
      App.api.getAccounts(),
      App.api.getCategories(),
      App.api.getCurrencies(),
      App.api.getCategoryBudgets(),
      App.api.getTags(),
      refreshUsage(),
    ])
    state.accounts = a
    state.categories = c
    state.currencies = cur
    state.baseCurrency = cur.find((c) => c.is_default)?.code ?? cur[0]?.code ?? 'RUB'
    state.categoryBudgets = budgets
    state.tags = tags
    state.loading = false
    state.loaded = true
    await refreshBudgetAlerts()
  }

  // Used after a full data import (see App.api.importData): every id in state is now stale, so the
  // simplest correct thing is to throw it all away and load() fresh, same as first startup.
  async function reload() {
    state.loaded = false
    await load()
    state.transactionsVersion += 1
  }

  function budgetFor(categoryId, monthKey) {
    return state.categoryBudgets[categoryId]?.[monthKey] ?? 0
  }

  async function setCategoryBudget(categoryId, monthKey, amount) {
    await App.api.setCategoryBudget(categoryId, monthKey, amount)
    if (!state.categoryBudgets[categoryId]) state.categoryBudgets[categoryId] = {}
    if (amount > 0) state.categoryBudgets[categoryId][monthKey] = amount
    else delete state.categoryBudgets[categoryId][monthKey]
    await refreshBudgetAlerts()
  }

  // refreshBudgetAlerts recomputes which categories are over their current-month budget. Called
  // after initial load, after every transaction mutation (via the transactionsVersion watch below),
  // and after a budget itself changes — anything that can push a category over or pull it back
  // under.
  async function refreshBudgetAlerts() {
    const { dateFrom, dateTo } = App.periodToDateRange('month')
    const transactions = await App.api.getAllTransactions({ dateFrom, dateTo, type: App.TransactionType.EXPENSE })

    const spentByCategory = new Map()
    for (const t of transactions) {
      if (t.amount >= 0) continue
      const key = t.categoryId ?? 'other-expense'
      spentByCategory.set(key, (spentByCategory.get(key) ?? 0) + Math.abs(amountInBase(t)))
    }

    const monthKey = currentMonthKey()
    const alerts = []
    for (const [categoryId, spent] of spentByCategory) {
      const budgeted = budgetFor(categoryId, monthKey)
      if (budgeted > 0 && spent > budgeted) {
        alerts.push({ categoryId, monthKey, category: App.getCategory(categoryId), spent, budgeted, overBy: spent - budgeted })
      }
    }
    state.budgetAlerts = alerts
  }

  const activeAccounts = computed(() => state.accounts.filter((a) => !a.archived))
  const categoryById = computed(() => new Map(state.categories.map((c) => [c.id, c])))
  const accountById = computed(() => new Map(state.accounts.map((a) => [a.id, a])))
  const currencyByCode = computed(() => new Map(state.currencies.map((c) => [c.code, c])))
  const tagById = computed(() => new Map(state.tags.map((t) => [t.id, t])))

  function currencyRate(code) {
    return currencyByCode.value.get(code)?.rate ?? 1
  }

  function toBase(amount, code) {
    if (code === state.baseCurrency) return amount
    return (amount * currencyRate(code)) / currencyRate(state.baseCurrency)
  }

  function amountInBase(tx) {
    const currency = accountById.value.get(tx.accountId)?.currency ?? state.baseCurrency
    return toBase(tx.amount, currency)
  }

  const totalBalanceBase = computed(() =>
    activeAccounts.value.reduce((sum, a) => sum + toBase(a.balance, a.currency), 0),
  )

  // Account balances are maintained authoritatively and transactionally by the backend (see
  // internal/adapter/mysql/transaction.go and transfer.go) — rather than hand-adjust acc.balance
  // client-side and risk drift, we just re-fetch the account list after any mutation that can
  // change a balance.
  async function refreshAccounts() {
    state.accounts = await App.api.getAccounts()
  }

  // afterTransactionMutation refreshes what a transaction add/edit/delete can invalidate — account
  // balances and the usage sets — and bumps transactionsVersion so any view holding its own bounded
  // window of transactions knows to refetch.
  async function afterTransactionMutation() {
    await Promise.all([refreshAccounts(), refreshUsage()])
    state.transactionsVersion += 1
  }

  async function addTransaction(tx) {
    const created = await App.api.createTransaction(tx)
    await afterTransactionMutation()
    return created
  }

  async function updateTransaction(tx) {
    const updated = await App.api.updateTransaction(tx)
    await afterTransactionMutation()
    return updated
  }

  async function deleteTransaction(id) {
    await App.api.deleteTransaction(id)
    await afterTransactionMutation()
  }

  async function addTransfer({ fromAccountId, toAccountId, amount, toAmount, rate, date, memo }) {
    const { legFrom, legTo } = await App.api.createTransfer({ fromAccountId, toAccountId, amount, toAmount, rate, date, memo })
    await afterTransactionMutation()
    return { legFrom, legTo }
  }

  async function deleteTransfer(id) {
    await App.api.deleteTransfer(id)
    await afterTransactionMutation()
  }

  async function updateTransfer(id, payload) {
    await deleteTransfer(id)
    return addTransfer(payload)
  }

  async function addAccount(account) {
    const created = await App.api.createAccount(account)
    state.accounts.unshift(created)
    return created
  }

  async function updateAccount(account) {
    await App.api.updateAccount(account)
    const idx = state.accounts.findIndex((a) => a.id === account.id)
    if (idx !== -1) state.accounts[idx] = account
  }

  function isAccountInUse(id) {
    return state.usedAccountIds.has(id)
  }

  async function archiveAccount(id) {
    const acc = accountById.value.get(id)
    if (!acc) return
    await updateAccount({ ...acc, archived: true })
  }

  async function unarchiveAccount(id) {
    const acc = accountById.value.get(id)
    if (!acc) return
    await updateAccount({ ...acc, archived: false })
  }

  async function deleteAccount(id) {
    if (isAccountInUse(id)) return
    await App.api.deleteAccount(id)
    state.accounts = state.accounts.filter((a) => a.id !== id)
  }

  // Which currency is default is stored per-row on currencies.is_default, and the backend clears
  // every other row's flag atomically whenever one is set — so after any currency mutation we
  // re-fetch the whole list rather than patch it locally, the same reasoning as refreshAccounts().
  async function refreshCurrencies() {
    state.currencies = await App.api.getCurrencies()
    state.baseCurrency = state.currencies.find((c) => c.is_default)?.code ?? state.baseCurrency
  }

  async function addCurrency(currency) {
    // The very first currency has nothing to be relative to, so it becomes the default.
    await App.api.createCurrency({ ...currency, default: state.currencies.length === 0 })
    await refreshCurrencies()
  }

  async function updateCurrency(currency) {
    await App.api.updateCurrency(currency)
    await refreshCurrencies()
  }

  function isCurrencyInUse(code) {
    return state.accounts.some((a) => a.currency === code)
  }

  async function archiveCurrency(code) {
    const cur = state.currencies.find((c) => c.code === code)
    if (!cur) return
    await updateCurrency({ code: cur.code, symbol: cur.symbol, name: cur.name, rate: cur.rate, default: cur.is_default, archived: true })
  }

  async function unarchiveCurrency(code) {
    const cur = state.currencies.find((c) => c.code === code)
    if (!cur) return
    await updateCurrency({ code: cur.code, symbol: cur.symbol, name: cur.name, rate: cur.rate, default: cur.is_default, archived: false })
  }

  async function deleteCurrency(code) {
    if (isCurrencyInUse(code)) return
    await App.api.deleteCurrency(code)
    await refreshCurrencies()
  }

  async function setDefaultCurrency(code) {
    const cur = state.currencies.find((c) => c.code === code)
    if (!cur) return
    await updateCurrency({ code: cur.code, symbol: cur.symbol, name: cur.name, rate: cur.rate, default: true, archived: cur.archived })
  }

  async function addCategory(category) {
    const created = await App.api.createCategory(category)
    state.categories.push(created)
    return created
  }

  async function updateCategory(category) {
    await App.api.updateCategory(category)
    const idx = state.categories.findIndex((c) => c.id === category.id)
    if (idx !== -1) state.categories[idx] = category
  }

  // isCategoryInUse only checks transactions — a category's budget entries don't block deletion,
  // they're just removed along with it (see deleteCategory; the backend cascades budgets.category_id
  // ON DELETE CASCADE).
  function isCategoryInUse(id) {
    return state.usedCategoryIds.has(id)
  }

  async function archiveCategory(id) {
    const cat = state.categories.find((c) => c.id === id)
    if (!cat) return
    await updateCategory({ ...cat, archived: true })
  }

  async function unarchiveCategory(id) {
    const cat = state.categories.find((c) => c.id === id)
    if (!cat) return
    await updateCategory({ ...cat, archived: false })
  }

  async function deleteCategory(id) {
    if (isCategoryInUse(id)) return
    await App.api.deleteCategory(id)
    state.categories = state.categories.filter((c) => c.id !== id)
    delete state.categoryBudgets[id]
  }

  async function addTag(tag) {
    const created = await App.api.createTag(tag)
    state.tags.push(created)
    return created
  }

  async function updateTag(tag) {
    await App.api.updateTag(tag)
    const idx = state.tags.findIndex((t) => t.id === tag.id)
    if (idx !== -1) state.tags[idx] = tag
  }

  async function deleteTag(id) {
    await App.api.deleteTag(id)
    state.tags = state.tags.filter((t) => t.id !== id)
    // Any transaction referencing this tag is updated in the DB by the backend's cascade delete —
    // bump transactionsVersion so views holding their own bounded window refetch and pick that up.
    state.transactionsVersion += 1
  }

  // A transaction add/edit/delete can push a category over budget or pull it back under, so
  // refresh alerts whenever transactionsVersion moves — the same signal every bounded transaction
  // list in the app already watches for this reason.
  watch(() => state.transactionsVersion, refreshBudgetAlerts)

  return {
    state,
    activeAccounts,
    totalBalanceBase,
    categoryById,
    accountById,
    currencyByCode,
    tagById,
    toBase,
    amountInBase,
    load,
    reload,
    addTransaction,
    updateTransaction,
    deleteTransaction,
    addTransfer,
    updateTransfer,
    deleteTransfer,
    addAccount,
    updateAccount,
    isAccountInUse,
    archiveAccount,
    unarchiveAccount,
    deleteAccount,
    addCurrency,
    updateCurrency,
    isCurrencyInUse,
    archiveCurrency,
    unarchiveCurrency,
    deleteCurrency,
    setDefaultCurrency,
    addCategory,
    updateCategory,
    isCategoryInUse,
    archiveCategory,
    unarchiveCategory,
    deleteCategory,
    budgetFor,
    setCategoryBudget,
    addTag,
    updateTag,
    deleteTag,
  }
}

function createUiStore() {
  const state = reactive({
    transactionModalOpen: false,
    editingTransaction: null,
    accountModalOpen: false,
    editingAccount: null,
    currencyModalOpen: false,
    editingCurrency: null,
    categoryModalOpen: false,
    editingCategory: null,
    newCategoryType: App.CategoryType.EXPENSE,
    tagModalOpen: false,
    editingTag: null,
    mobileMenuOpen: false,
  })

  function openNewTransaction() {
    state.editingTransaction = null
    state.transactionModalOpen = true
  }
  function openEditTransaction(tx) {
    state.editingTransaction = tx
    state.transactionModalOpen = true
  }
  function closeTransactionModal() {
    state.transactionModalOpen = false
    state.editingTransaction = null
  }
  function openNewAccount() {
    state.editingAccount = null
    state.accountModalOpen = true
  }
  function openEditAccount(account) {
    state.editingAccount = account
    state.accountModalOpen = true
  }
  function closeAccountModal() {
    state.accountModalOpen = false
    state.editingAccount = null
  }
  function openNewCurrency() {
    state.editingCurrency = null
    state.currencyModalOpen = true
  }
  function openEditCurrency(currency) {
    state.editingCurrency = currency
    state.currencyModalOpen = true
  }
  function closeCurrencyModal() {
    state.currencyModalOpen = false
    state.editingCurrency = null
  }
  function openNewCategory(type) {
    state.editingCategory = null
    state.newCategoryType = type || App.CategoryType.EXPENSE
    state.categoryModalOpen = true
  }
  function openEditCategory(category) {
    state.editingCategory = category
    state.categoryModalOpen = true
  }
  function closeCategoryModal() {
    state.categoryModalOpen = false
    state.editingCategory = null
  }
  function openNewTag() {
    state.editingTag = null
    state.tagModalOpen = true
  }
  function openEditTag(tag) {
    state.editingTag = tag
    state.tagModalOpen = true
  }
  function closeTagModal() {
    state.tagModalOpen = false
    state.editingTag = null
  }

  return {
    state,
    openNewTransaction,
    openEditTransaction,
    closeTransactionModal,
    openNewAccount,
    openEditAccount,
    closeAccountModal,
    openNewCurrency,
    openEditCurrency,
    closeCurrencyModal,
    openNewCategory,
    openEditCategory,
    closeCategoryModal,
    openNewTag,
    openEditTag,
    closeTagModal,
  }
}

App.financeStore = createFinanceStore()
App.uiStore = createUiStore()
})();
