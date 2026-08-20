window.App = window.App || {};

// Account type is a bare number on the wire (matches Go's entity.AccountType), not a string — these
// maps translate it to an icon name / label for display. Keys must stay in sync with the backend's
// entity.AccountType constants (1 = cash, 2 = card, ...).
App.accountIcon = {
  1: 'cash',
  2: 'card',
  3: 'bank',
  4: 'savings',
  5: 'card',
  6: 'debt',
  7: 'virtual',
}

App.accountTypeLabel = {
  1: 'Наличные',
  2: 'Банковская карта',
  3: 'Текущий счёт',
  4: 'Сберегательный счёт',
  5: 'Кредитная карта',
  6: 'Долговой счёт',
  7: 'Виртуальный счёт',
}
