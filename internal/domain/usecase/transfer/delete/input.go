package delete

// Input is what DeleteTransfer needs to remove a transfer. ID is the id of either one of its two
// legs — the other is found via its transfer_transaction_id pointer.
type Input struct {
	ID uint64
}
