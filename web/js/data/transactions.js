window.App = window.App || {};

// Transaction type is a bare number on the wire (matches Go's entity.TransactionType), not a
// string — this constant must stay in sync with the backend's entity.TransactionType values.
// Only TRANSFER is ever compared against in the frontend: a plain expense/income row's direction is
// derived from the amount's sign instead (see form.direction in transaction-modal.js).
App.TransactionType = {
  INCOME: 1,
  EXPENSE: 2,
  TRANSFER: 3,
}
