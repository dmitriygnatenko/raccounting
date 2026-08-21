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

// monthKey is 'YYYY-MM', as used for budgetMonth throughout the app.
App.formatMonthYear = function (monthKey) {
  const [year, month] = monthKey.split('-').map(Number)
  const label = new Intl.DateTimeFormat(intlLocale(), { month: 'long', year: 'numeric' }).format(new Date(year, month - 1, 1))
  return label.charAt(0).toUpperCase() + label.slice(1)
}

// dateStr formats a Date as 'YYYY-MM-DD' in local time (unlike Date#toISOString, which is UTC and
// can land on the wrong day near midnight) — the wire format entity.DateLayout expects.
App.dateStr = function (d) {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

// periodToDateRange resolves one of the period selectors used by the Операции/Отчёты views
// ('all' | 'month' | 'lastMonth' | 'year' | 'custom') into concrete {dateFrom, dateTo} bounds
// (inclusive, 'YYYY-MM-DD') to send to the backend — 'all' and an empty custom bound come back as
// undefined, meaning "no bound on that side".
App.periodToDateRange = function (period, customFrom, customTo) {
  const now = new Date()

  switch (period) {
    case 'custom':
      return { dateFrom: customFrom || undefined, dateTo: customTo || undefined }
    case 'month':
      return {
        dateFrom: App.dateStr(new Date(now.getFullYear(), now.getMonth(), 1)),
        dateTo: App.dateStr(new Date(now.getFullYear(), now.getMonth() + 1, 0)),
      }
    case 'lastMonth':
      return {
        dateFrom: App.dateStr(new Date(now.getFullYear(), now.getMonth() - 1, 1)),
        dateTo: App.dateStr(new Date(now.getFullYear(), now.getMonth(), 0)),
      }
    case 'year':
      return {
        dateFrom: App.dateStr(new Date(now.getFullYear(), 0, 1)),
        dateTo: App.dateStr(new Date(now.getFullYear(), 11, 31)),
      }
    default:
      return { dateFrom: undefined, dateTo: undefined }
  }
}
})();
