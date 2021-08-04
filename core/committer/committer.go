package committer

import "github.com/hyperledger/fabric/protos/common"

// Committer is the interface supported by committers
// The only committer is noopssinglechain committer.
// The interface is intentionally sparse with the sole
// aim of "leave-everything-to-the-committer-for-now".
// As we solidify the bootstrap process and as we add
// more support (such as Gossip) this interface will
// change
type Committer interface {

	// Commit block to the ledger
	Commit(block *common.Block) error

	// Get recent block sequence number
	LedgerHeight() (uint64, error)

	// Gets blocks with sequence numbers provided in the slice
	GetBlocks(blockSeqs []uint64) []*common.Block

	// Closes committing service
	Close()
}
