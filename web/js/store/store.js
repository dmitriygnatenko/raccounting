window.App = window.App || {};

;(function () {
const { reactive, computed } = Vue

function createFinanceStore() {
  const state = reactive({
    accounts: [],
    categories: [],
    transactions: [],
    currencies: [],
    tags: [],
    baseCurrency: 'RUB',
    categoryBudgets: {},
    loading: true,
    loaded: false,
  })

  async function load() {
    if (state.loaded) return
    state.loading = true
    const [a, c, t, cur, budgets, tags] = await Promise.all([
      App.api.getAccounts(),
      App.api.getCategories(),
      App.api.getTransactions(),
      App.api.getCurrencies(),
      App.api.getCategoryBudgets(),
      App.api.getTags(),
    ])
    state.accounts = a
    state.categories = c
    state.transactions = t
    state.currencies = cur
    state.baseCurrency = cur.find((c) => c.is_default)?.code ?? cur[0]?.code ?? 'RUB'
    state.categoryBudgets = budgets
    state.tags = tags
    state.loading = false
    state.loaded = true
  }

  function budgetFor(categoryId, monthKey) {
    return state.categoryBudgets[categoryId]?.[monthKey] ?? 0
  }

  async function setCategoryBudget(categoryId, monthKey, amount) {
    await App.api.setCategoryBudget(categoryId, monthKey, amount)
    if (!state.categoryBudgets[categoryId]) state.categoryBudgets[categoryId] = {}
    if (amount > 0) state.categoryBudgets[categoryId][monthKey] = amount
    else delete state.categoryBudgets[categoryId][monthKey]
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

  async function addTransaction(tx) {
    const created = await App.api.createTransaction(tx)
    state.transactions.unshift(created)
    await refreshAccounts()
    return created
  }

  async function updateTransaction(tx) {
    await App.api.updateTransaction(tx)
    const idx = state.transactions.findIndex((t) => t.id === tx.id)
    if (idx !== -1) state.transactions[idx] = tx
    await refreshAccounts()
  }

  async function deleteTransaction(id) {
    await App.api.deleteTransaction(id)
    state.transactions = state.transactions.filter((t) => t.id !== id)
    await refreshAccounts()
  }

  async function addTransfer({ fromAccountId, toAccountId, amount, toAmount, rate, date, memo }) {
    const { legFrom, legTo } = await App.api.createTransfer({ fromAccountId, toAccountId, amount, toAmount, rate, date, memo })
    state.transactions.unshift(legTo, legFrom)
    await refreshAccounts()
    return { legFrom, legTo }
  }

  async function deleteTransfer(id) {
    const leg = state.transactions.find((t) => t.id === id)
    const pairedId = leg?.transferTransactionId
    await App.api.deleteTransfer(id)
    state.transactions = state.transactions.filter((t) => t.id !== id && t.id !== pairedId)
    await refreshAccounts()
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
    return state.transactions.some((t) => t.accountId === id)
  }

  async function archiveAccount(id) {
    const acc = accountById.value.get(id)
    if (!acc) return
    await updateAccount({ ...acc, archived: true })
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

  function isCategoryInUse(id) {
    const hasBudget = state.categoryBudgets[id] && Object.keys(state.categoryBudgets[id]).length > 0
    return state.transactions.some((t) => t.categoryId === id) || !!hasBudget
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
    state.transactions.forEach((t) => {
      if (t.tagIds?.includes(id)) t.tagIds = t.tagIds.filter((tagId) => tagId !== id)
    })
  }

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
