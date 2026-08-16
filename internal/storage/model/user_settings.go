package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"raccounting/internal/domain/entity"
)

// UserSettings is the shape of the users.settings JSON column.
type UserSettings struct {
	Language string `json:"language,omitempty"`
}

// ToEntity converts the stored column into a domain entity.UserSettings.
func (m UserSettings) ToEntity() entity.UserSettings {
	return entity.UserSettings{
		Language: m.Language,
	}
}

// Scan implements sql.Scanner, decoding the users.settings JSON column.
func (m *UserSettings) Scan(src any) error {
	if src == nil {
		*m = UserSettings{}
		return nil
	}

	var data []byte

	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("model: cannot scan %T into UserSettings", src)
	}

	if len(data) == 0 {
		*m = UserSettings{}
		return nil
	}

	return json.Unmarshal(data, m)
}

// Value implements driver.Valuer, encoding m for the users.settings JSON column.
func (m UserSettings) Value() (driver.Value, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	return string(data), nil
}
