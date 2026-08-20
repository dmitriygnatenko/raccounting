package entity

import "time"

// BackupFormatVersion is the current on-disk/wire version of Backup. Bump it whenever Backup's shape
// changes in a way older readers can't handle — the restore use case rejects any other version
// outright rather than guessing at a migration.
const BackupFormatVersion = 1

// Backup is the full export/import snapshot of every table raccounting stores, except the user's own
// login credentials (username/password) — see the auth/updatecredentials use case for those.
// raccounting is single-user (see port.TransactionRepository), so a Backup is a complete copy of the
// app's data: importing one replaces whatever is currently stored.
//
// IDs on Accounts/Categories/Tags/Transactions only need to be unique within the file itself — they
// let Transactions/Budgets reference the right account/category/tag/transfer-partner on export, and
// restore uses them to rebuild those links before assigning each row a fresh id. Currencies are keyed
// by Code instead, which is already stable and meaningful across export/import.
type Backup struct {
	Version      int           `json:"version"`
	ExportedAt   time.Time     `json:"exported_at"`
	Settings     UserSettings  `json:"settings"`
	Currencies   []Currency    `json:"currencies"`
	Accounts     []Account     `json:"accounts"`
	Categories   []Category    `json:"categories"`
	Tags         []Tag         `json:"tags"`
	Transactions []Transaction `json:"transactions"`
	Budgets      []Budget      `json:"budgets"`
}
