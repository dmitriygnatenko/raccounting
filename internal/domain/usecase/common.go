// Package usecase holds the small pieces every use case package shares: username normalization,
// validation rule sets, and default values — kept in one place so the rules stay consistent across
// auth, account, category, currency, transaction, transfer, category budget and settings use cases.
package usecase

import (
	"fmt"
	"regexp"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"raccounting/internal/domain/entity"
)

// NormalizeUsername lower-cases and trims a username, so "Admin " and "admin" are the same account.
func NormalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

// UsernameRules is the ozzo-validation rule set for a username field: required, and between
// entity.MinUsernameLength and entity.MaxUsernameLength characters (the latter matches the
// users.username column width — see entity.MaxUsernameLength).
func UsernameRules() []validation.Rule {
	return []validation.Rule{
		validation.Required.Error("Please enter a username"),
		validation.Length(entity.MinUsernameLength, entity.MaxUsernameLength).
			Error(fmt.Sprintf(
				"Username must be between %d and %d characters",
				entity.MinUsernameLength, entity.MaxUsernameLength,
			)),
	}
}

// PasswordRules is the ozzo-validation rule set for a password field: required, and at least
// entity.MinPasswordLength characters. Required is chained in front of Length with the same
// message: Length alone treats an empty value as valid, which would let a blank password slip
// through silently.
func PasswordRules() []validation.Rule {
	return []validation.Rule{
		validation.Required.Error("Please enter a password"),
		validation.Length(entity.MinPasswordLength, 0).
			Error(fmt.Sprintf("Password must be at least %d characters", entity.MinPasswordLength)),
	}
}

// AccountNameRules is the ozzo-validation rule set for an account name field.
func AccountNameRules() []validation.Rule {
	return []validation.Rule{
		validation.Required.Error("Please enter an account name"),
		validation.Length(0, entity.MaxAccountNameLength).
			Error(fmt.Sprintf("Account name must be at most %d characters", entity.MaxAccountNameLength)),
	}
}

// AccountTypeRules is the ozzo-validation rule set for a numeric account type field: required (the
// zero value means "not set" — every real entity.AccountType starts at 1), and one of the known
// account types (see entity.AccountTypes).
func AccountTypeRules() []validation.Rule {
	types := entity.AccountTypes()
	values := make([]any, len(types))

	for i, t := range types {
		values[i] = uint8(t)
	}

	return []validation.Rule{
		validation.Required.Error("Please choose an account type"),
		validation.In(values...).Error("Unknown account type"),
	}
}

// currencyCodeRE matches an ISO 4217 currency code: exactly three uppercase letters (e.g. "RUB",
// "USD"). Input is normalized (trimmed, upper-cased) before this runs — see currency create/update
// Input.Validate.
var currencyCodeRE = regexp.MustCompile(`^[A-Z]{3}$`)

// CurrencyCodeRules is the ozzo-validation rule set for a currency code field: required, and
// exactly entity.CurrencyCodeLength uppercase letters (ISO 4217).
func CurrencyCodeRules() []validation.Rule {
	return []validation.Rule{
		validation.Required.Error("Please choose a currency"),
		validation.Match(currencyCodeRE).
			Error(fmt.Sprintf("Currency code must be %d letters (ISO 4217), e.g. \"USD\"", entity.CurrencyCodeLength)),
	}
}

// CategoryNameRules is the ozzo-validation rule set for a category name field.
func CategoryNameRules() []validation.Rule {
	return []validation.Rule{
		validation.Required.Error("Please enter a category name"),
		validation.Length(0, entity.MaxCategoryNameLength).
			Error(fmt.Sprintf("Category name must be at most %d characters", entity.MaxCategoryNameLength)),
	}
}

// CategoryTypeRules is the ozzo-validation rule set for a numeric category type field: required
// (the zero value means "not set" — every real entity.CategoryType starts at 1), and one of the
// known category types (see entity.CategoryTypes).
func CategoryTypeRules() []validation.Rule {
	const msg = "Category type must be \"expense\" or \"income\""

	types := entity.CategoryTypes()
	values := make([]any, len(types))

	for i, t := range types {
		values[i] = uint8(t)
	}

	return []validation.Rule{
		validation.Required.Error(msg),
		validation.In(values...).Error(msg),
	}
}

// CategoryColorRules is the ozzo-validation rule set for a category color field. Not Required —
// usecase.ResolveColor falls back to a default when it's blank.
func CategoryColorRules() []validation.Rule {
	return []validation.Rule{
		validation.Length(0, entity.MaxCategoryColorLength).
			Error(fmt.Sprintf("Category color must be at most %d characters", entity.MaxCategoryColorLength)),
	}
}

// TagNameRules is the ozzo-validation rule set for a tag name field.
func TagNameRules() []validation.Rule {
	return []validation.Rule{
		validation.Required.Error("Please enter a tag name"),
		validation.Length(0, entity.MaxTagNameLength).
			Error(fmt.Sprintf("Tag name must be at most %d characters", entity.MaxTagNameLength)),
	}
}

// TagColorRules is the ozzo-validation rule set for a tag color field. Not Required —
// usecase.ResolveColor falls back to a default when it's blank.
func TagColorRules() []validation.Rule {
	return []validation.Rule{
		validation.Length(0, entity.MaxTagColorLength).
			Error(fmt.Sprintf("Tag color must be at most %d characters", entity.MaxTagColorLength)),
	}
}

// CurrencySymbolRules is the ozzo-validation rule set for a currency symbol field.
func CurrencySymbolRules() []validation.Rule {
	return []validation.Rule{
		validation.Required.Error("Please enter a currency symbol"),
		validation.Length(0, entity.MaxCurrencySymbolLength).
			Error(fmt.Sprintf("Currency symbol must be at most %d characters", entity.MaxCurrencySymbolLength)),
	}
}

// CurrencyNameRules is the ozzo-validation rule set for a currency display-name field.
func CurrencyNameRules() []validation.Rule {
	return []validation.Rule{
		validation.Required.Error("Please enter a currency name"),
		validation.Length(0, entity.MaxCurrencyNameLength).
			Error(fmt.Sprintf("Currency name must be at most %d characters", entity.MaxCurrencyNameLength)),
	}
}

// RateRules is the ozzo-validation rule set for a currency's rate field: must be positive.
func RateRules() []validation.Rule {
	return []validation.Rule{
		validation.Min(0.000001).Error("Exchange rate must be greater than zero"),
	}
}

// AccountIDRules is the ozzo-validation rule set for a field referencing the account a transaction
// belongs to: it must be a positive id.
func AccountIDRules() []validation.Rule {
	return []validation.Rule{
		validation.Min(uint64(1)).Error("Choose an account"),
	}
}

// AmountRules is the ozzo-validation rule set for a signed transaction Amount field: nonzero, and
// bounded to entity.MaxAmount in either direction.
func AmountRules() []validation.Rule {
	limitErr := fmt.Sprintf("Amount must be at most %d", entity.MaxAmount)

	return []validation.Rule{
		validation.Required.Error("Amount must not be zero"),
		validation.Min(int64(-entity.MaxAmount)).Error(limitErr),
		validation.Max(int64(entity.MaxAmount)).Error(limitErr),
	}
}

// TransferAmountRules is the ozzo-validation rule set for an unsigned transfer Amount/ToAmount
// field: must be positive, and bounded to entity.MaxAmount.
func TransferAmountRules() []validation.Rule {
	return []validation.Rule{
		validation.Min(int64(1)).Error("Transfer amount must be greater than zero"),
		validation.Max(int64(entity.MaxAmount)).Error(fmt.Sprintf("Transfer amount must be at most %d", entity.MaxAmount)),
	}
}

// CategoryIDRules is the ozzo-validation rule set for a field referencing a category: it must be a
// positive id.
func CategoryIDRules() []validation.Rule {
	return []validation.Rule{
		validation.Min(uint64(1)).Error("Choose a category"),
	}
}

// dateRE matches entity.DateLayout ("YYYY-MM-DD").
var dateRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// DateRules is the ozzo-validation rule set for a transaction/transfer date field.
func DateRules() []validation.Rule {
	return []validation.Rule{
		validation.Required.Error("Please choose a date"),
		validation.Match(dateRE).Error("Date must be in YYYY-MM-DD format"),
	}
}

// monthKeyRE matches a category budget's MonthKey ("YYYY-MM").
var monthKeyRE = regexp.MustCompile(`^\d{4}-\d{2}$`)

// MonthKeyRules is the ozzo-validation rule set for a category budget's month key field.
func MonthKeyRules() []validation.Rule {
	return []validation.Rule{
		validation.Required.Error("Please choose a month"),
		validation.Match(monthKeyRE).Error("Month must be in YYYY-MM format"),
	}
}

// languageRE matches a BCP 47-ish UI language tag: 2-3 lowercase letters, optionally followed by a
// dash and an uppercase region (e.g. "en", "ru", "pt-BR") — covers App.SUPPORTED_LOCALES in the
// frontend without hard-coding that specific list here.
var languageRE = regexp.MustCompile(`^[a-z]{2,3}(-[A-Z]{2})?$`)

// LanguageRules is the ozzo-validation rule set for a UI language field.
func LanguageRules() []validation.Rule {
	return []validation.Rule{
		validation.Required.Error("Please choose a language"),
		validation.Match(languageRE).Error("Language must be a valid language code, e.g. \"en\""),
	}
}

// ThemeRules is the ozzo-validation rule set for a UI theme field: required, and either "light" or
// "dark".
func ThemeRules() []validation.Rule {
	return []validation.Rule{
		validation.Required.Error("Please choose a theme"),
		validation.In("light", "dark").Error("Theme must be \"light\" or \"dark\""),
	}
}

// DedupeIDs drops zero values and duplicates from ids, preserving first-seen order — used to
// normalize a transaction's tag id list before it's validated/persisted.
func DedupeIDs(ids []uint64) []uint64 {
	seen := make(map[uint64]struct{}, len(ids))
	out := make([]uint64, 0, len(ids))

	for _, id := range ids {
		if id == 0 {
			continue
		}

		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		out = append(out, id)
	}

	return out
}

// DefaultCategoryColor is used whenever the caller doesn't supply a category color.
const DefaultCategoryColor = "#64748b"

// ResolveColor trims the given color, falling back to DefaultCategoryColor when blank.
func ResolveColor(color string) string {
	color = strings.TrimSpace(color)
	if color == "" {
		return DefaultCategoryColor
	}

	return color
}
