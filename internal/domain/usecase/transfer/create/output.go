package create

import "raccounting/internal/domain/entity"

// Output is the pair of transaction legs the transfer created.
type Output struct {
	LegFrom entity.Transaction
	LegTo   entity.Transaction
}
