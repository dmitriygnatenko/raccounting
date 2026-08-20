package entity

import (
	"encoding/json"
	"time"
)

// CurrencyCodeLength is the required length of a currency code — ISO 4217 (e.g. "RUB", "USD").
const CurrencyCodeLength = 3

// MaxCurrencySymbolLength is the maximum accepted length for a currency symbol (e.g. "₽", "USDT").
const MaxCurrencySymbolLength = 10

// MaxCurrencyNameLength is the maximum accepted length for a currency's display name.
const MaxCurrencyNameLength = 255

type CurrencyStatus uint8

const (
	CurrencyStatusActive CurrencyStatus = iota + 1
	CurrencyStatusArchived
)

// Currency is a unit of money accounts can be denominated in. Rate is how the frontend converts
// between currencies: everything is relative to whichever currency has Default set. The default
// currency's own Rate is conventionally 1.
type Currency struct {
	Code      string
	Symbol    string
	Name      string
	Rate      float64
	Default   bool
	Status    CurrencyStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

// currencyJSON is Currency's wire shape: Status collapsed to a boolean (the frontend only ever
// asks "is this archived?").
type currencyJSON struct {
	Code      string    `json:"code"`
	Symbol    string    `json:"symbol"`
	Name      string    `json:"name"`
	Rate      float64   `json:"rate"`
	Default   bool      `json:"is_default"`
	Archived  bool      `json:"archived"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (c Currency) MarshalJSON() ([]byte, error) {
	return json.Marshal(currencyJSON{
		Code:      c.Code,
		Symbol:    c.Symbol,
		Name:      c.Name,
		Rate:      c.Rate,
		Default:   c.Default,
		Archived:  c.Status == CurrencyStatusArchived,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	})
}

// UnmarshalJSON is currencyJSON's inverse — used to read a Currency back out of a Backup file (see
// entity.Backup).
func (c *Currency) UnmarshalJSON(data []byte) error {
	var j currencyJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}

	status := CurrencyStatusActive
	if j.Archived {
		status = CurrencyStatusArchived
	}

	*c = Currency{
		Code:      j.Code,
		Symbol:    j.Symbol,
		Name:      j.Name,
		Rate:      j.Rate,
		Default:   j.Default,
		Status:    status,
		CreatedAt: j.CreatedAt,
		UpdatedAt: j.UpdatedAt,
	}

	return nil
}
