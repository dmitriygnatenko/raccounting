package update

import "raccounting/internal/domain/entity"

// Output is the signed-in user's settings after the requested change was applied.
type Output struct {
	Settings entity.UserSettings
}
