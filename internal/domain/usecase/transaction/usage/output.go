package usage

// Output is the set of account/category ids referenced by at least one transaction.
type Output struct {
	AccountIDs  []uint64
	CategoryIDs []uint64
}
