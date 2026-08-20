// Category lookup — categories themselves are loaded from the backend into App.financeStore.
window.App = window.App || {};

// Category type is a bare number on the wire (matches Go's entity.CategoryType), not a string —
// these constants must stay in sync with the backend's entity.CategoryType values.
App.CategoryType = {
  INCOME: 1,
  EXPENSE: 2,
}

App.getCategory = function (id) {
  return App.financeStore.state.categories.find((c) => c.id === id) ?? null
}
