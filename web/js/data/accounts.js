window.App = window.App || {};

// Account type is a bare number on the wire (matches Go's entity.AccountType), not a string — these
// maps translate it to an icon name / label for display. Keys must stay in sync with the backend's
// entity.AccountType constants (1 = cash, 2 = card, ...).
App.AccountType = {
  CASH: 1,
  CARD: 2,
  ACCOUNT: 3,
  SAVINGS: 4,
  CREDIT_CARD: 5,
  DEBT: 6,
  VIRTUAL: 7,
}

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
