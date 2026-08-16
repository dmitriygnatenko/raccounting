package list

import "raccounting/internal/domain/entity"

// Output is every category belonging to the signed-in user.
type Output struct {
	Categories []entity.Category
}
