window.App = window.App || {};

;(function () {
// Real API layer — talks to the Go backend under /api/... over same-origin fetch(). Cookies (the
// session) are sent automatically for same-origin requests, so no credentials:'include' is needed.

async function request(method, path, body) {
  const init = { method, headers: {} }
  if (body !== undefined) {
    init.headers['Content-Type'] = 'application/json'
    init.body = JSON.stringify(body)
  }

  const res = await fetch(path, init)

  let data
  if (res.status !== 204) {
    const text = await res.text()
    if (text) {
      try {
        data = JSON.parse(text)
      } catch {
        data = undefined
      }
    }
  }

  if (!res.ok) {
    const message = (data && data.error) || 'Request failed'
    throw new Error(message)
  }

  return data
}

App.api = {
  async login({ username, password, language }) {
    return request('POST', '/api/auth/login', { username, password, language })
  },
  async logout() {
    return request('POST', '/api/auth/logout')
  },
  async me() {
    return request('GET', '/api/auth/me')
  },
  async changeCredentials({ currentPassword, newUsername, newPassword }) {
    return request('PATCH', '/api/auth/credentials', {
      currentPassword,
      newUsername,
      newPassword,
    })
  },
  getAccounts() {
    return request('GET', '/api/accounts')
  },
  getCategories() {
    return request('GET', '/api/categories')
  },
  getTransactions() {
    return request('GET', '/api/transactions')
  },
  getCurrencies() {
    return request('GET', '/api/currencies')
  },
  async updateSettings(patch) {
    return request('PATCH', '/api/settings', patch)
  },
  getCategoryBudgets() {
    return request('GET', '/api/category-budgets')
  },
  async setCategoryBudget(categoryId, monthKey, amount) {
    return request('PUT', `/api/category-budgets/${categoryId}/${monthKey}`, { amount })
  },
  async createTransaction(tx) {
    return request('POST', '/api/transactions', tx)
  },
  async updateTransaction(tx) {
    return request('PUT', `/api/transactions/${tx.id}`, tx)
  },
  async deleteTransaction(id) {
    return request('DELETE', `/api/transactions/${id}`)
  },
  async createTransfer({ fromAccountId, toAccountId, amount, toAmount, rate, date, memo }) {
    return request('POST', '/api/transfers', { fromAccountId, toAccountId, amount, toAmount, rate, date, memo })
  },
  async deleteTransfer(id) {
    return request('DELETE', `/api/transfers/${id}`)
  },
  async createAccount(account) {
    return request('POST', '/api/accounts', account)
  },
  async updateAccount(account) {
    return request('PUT', `/api/accounts/${account.id}`, account)
  },
  async deleteAccount(id) {
    return request('DELETE', `/api/accounts/${id}`)
  },
  async createCurrency(currency) {
    return request('POST', '/api/currencies', currency)
  },
  async updateCurrency(currency) {
    return request('PUT', `/api/currencies/${currency.code}`, currency)
  },
  async deleteCurrency(code) {
    return request('DELETE', `/api/currencies/${code}`)
  },
  async createCategory(category) {
    return request('POST', '/api/categories', category)
  },
  async updateCategory(category) {
    return request('PUT', `/api/categories/${category.id}`, category)
  },
  async deleteCategory(id) {
    return request('DELETE', `/api/categories/${id}`)
  },
  getTags() {
    return request('GET', '/api/tags')
  },
  async createTag(tag) {
    return request('POST', '/api/tags', tag)
  },
  async updateTag(tag) {
    return request('PUT', `/api/tags/${tag.id}`, tag)
  },
  async deleteTag(id) {
    return request('DELETE', `/api/tags/${id}`)
  },
}
})();
