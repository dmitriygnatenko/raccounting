// Tag lookup — tags themselves are loaded from the backend into App.financeStore.
window.App = window.App || {};

App.getTag = function (id) {
  return App.financeStore.state.tags.find((t) => t.id === id) ?? null
}
