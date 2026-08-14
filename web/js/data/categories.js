// Category lookup — categories themselves are loaded from the backend into App.financeStore.
window.App = window.App || {};

App.getCategory = function (id) {
  return App.financeStore.state.categories.find((c) => c.id === id) ?? null
}
