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

// AccountTypes returns every known account type, in a stable order — used to build the "must be
// one of these" validation rule. The wire format is the bare numeric value (see accountJSON); the
// frontend (web/js/data/accounts.js) keeps its own number -> icon/label maps in this same order.
func AccountTypes() []AccountType {
	return []AccountType{
		AccountTypeCash,
		AccountTypeCard,
		AccountTypeAccount,
		AccountTypeSavings,
		AccountTypeCreditCard,
		AccountTypeDebt,
		AccountTypeVirtual,
	}
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

// accountJSON is Account's wire shape: Type as its bare numeric value (the frontend keeps its own
// number -> icon/label maps, see AccountTypes), Status collapsed to a boolean (the frontend only
// ever asks "is this archived?"), Balance converted from minor units to a decimal major-unit
// amount.
type accountJSON struct {
	ID        uint64    `json:"id"`
	Name      string    `json:"name"`
	Type      uint8     `json:"type"`
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
		Type:      uint8(a.Type),
		Currency:  a.CurrencyCode,
		Balance:   a.Balance,
		Archived:  a.Status == AccountStatusArchived,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	})
}

// UnmarshalJSON is accountJSON's inverse — used to read an Account back out of a Backup file (see
// entity.Backup).
func (a *Account) UnmarshalJSON(data []byte) error {
	var j accountJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}

	status := AccountStatusActive
	if j.Archived {
		status = AccountStatusArchived
	}

	*a = Account{
		ID:           j.ID,
		Name:         j.Name,
		Type:         AccountType(j.Type),
		CurrencyCode: j.Currency,
		Balance:      j.Balance,
		Status:       status,
		CreatedAt:    j.CreatedAt,
		UpdatedAt:    j.UpdatedAt,
	}

	return nil
}
