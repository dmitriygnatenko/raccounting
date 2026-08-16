package entity

import (
	"encoding/json"
	"time"
)

const MaxAccountNameLength = 255

type AccountType uint8

const (
	AccountTypeCash AccountType = iota + 1
	AccountTypeCard
	AccountTypeAccount
	AccountTypeSavings
	AccountTypeCreditCard
	AccountTypeDebt
	AccountTypeVirtual
)

// accountTypeNames maps each AccountType to its wire representation (see AccountType.String).
// AccountTypeAccount is deliberately serialized as "checking" — that's what the frontend
// (web/js/data/accounts.js) calls a regular bank account; the Go constant name predates that.
var accountTypeNames = map[AccountType]string{
	AccountTypeCash:       "cash",
	AccountTypeCard:       "card",
	AccountTypeAccount:    "checking",
	AccountTypeSavings:    "savings",
	AccountTypeCreditCard: "credit_card",
	AccountTypeDebt:       "debt",
	AccountTypeVirtual:    "virtual",
}

// String returns the wire-format name for t, or "" if t is not a known account type.
func (t AccountType) String() string {
	return accountTypeNames[t]
}

// ParseAccountType parses a wire-format account type string. ok is false for an unrecognized value.
func ParseAccountType(s string) (AccountType, bool) {
	for t, name := range accountTypeNames {
		if name == s {
			return t, true
		}
	}

	return 0, false
}

// AccountTypeNames returns every known account type's wire-format name, in a stable order —
// used to build the "must be one of these" validation rule.
func AccountTypeNames() []string {
	names := make([]string, 0, len(accountTypeNames))
	for _, t := range []AccountType{
		AccountTypeCash,
		AccountTypeCard,
		AccountTypeAccount,
		AccountTypeSavings,
		AccountTypeCreditCard,
		AccountTypeDebt,
		AccountTypeVirtual,
	} {
		names = append(names, t.String())
	}

	return names
}

type AccountStatus uint8

const (
	AccountStatusActive AccountStatus = iota + 1
	AccountStatusArchived
)

// Account is a wallet, bank account, or other place money is held.
type Account struct {
	ID           uint64
	Name         string
	Type         AccountType
	CurrencyCode string
	Balance      int64
	Status       AccountStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// accountJSON is Account's wire shape: Type as its string name, Status collapsed to a boolean
// (the frontend only ever asks "is this archived?"), Balance converted from minor units to a
// decimal major-unit amount.
type accountJSON struct {
	ID        uint64    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Currency  string    `json:"currency"`
	Balance   int64     `json:"balance"`
	Archived  bool      `json:"archived"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (a Account) MarshalJSON() ([]byte, error) {
	return json.Marshal(accountJSON{
		ID:        a.ID,
		Name:      a.Name,
		Type:      a.Type.String(),
		Currency:  a.CurrencyCode,
		Balance:   a.Balance,
		Archived:  a.Status == AccountStatusArchived,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	})
}
