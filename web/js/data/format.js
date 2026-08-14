window.App = window.App || {};

;(function () {
function intlLocale() {
  return App.LOCALE_INTL[App.i18nStore.locale] ?? 'ru-RU'
}

App.formatMoney = function (amount, currency = 'RUB') {
  const known = App.financeStore.state.currencies.find((c) => c.code === currency)
  const symbol = known ? known.symbol : currency
  const formatted = new Intl.NumberFormat(intlLocale(), {
    minimumFractionDigits: currency === 'USDT' ? 2 : 0,
    maximumFractionDigits: 2,
  }).format(Math.abs(amount))
  const sign = amount < 0 ? '−' : ''
  return `${sign}${formatted} ${symbol}`
}

App.formatDate = function (iso) {
  return new Intl.DateTimeFormat(intlLocale(), { day: '2-digit', month: 'short' }).format(new Date(iso))
}

App.formatDateLong = function (iso) {
  return new Intl.DateTimeFormat(intlLocale(), { day: '2-digit', month: 'long', year: 'numeric' }).format(new Date(iso))
}

App.formatMonthLabel = function (year, month) {
  return new Intl.DateTimeFormat(intlLocale(), { month: 'short' }).format(new Date(year, month, 1))
}
})();
