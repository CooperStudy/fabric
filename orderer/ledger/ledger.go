package ledger

import (
	cb "github.com/hyperledger/fabric/protos/common"
	ab "github.com/hyperledger/fabric/protos/orderer"
)

// Factory retrieves or creates new ledgers by chainID
type Factory interface {
	// GetOrCreate gets an existing ledger (if it exists)
	// or creates it if it does not
	GetOrCreate(chainID string) (ReadWriter, error)

	// ChainIDs returns the chain IDs the Factory is aware of
	ChainIDs() []string

	// Close releases all resources acquired by the factory
	Close()
}

// Iterator is useful for a chain Reader to stream blocks as they are created
type Iterator interface {
	// Next blocks until there is a new block available, or returns an error if
	// the next block is no longer retrievable
	Next() (*cb.Block, cb.Status)
	// ReadyChan supplies a channel which will block until Next will not block
	ReadyChan() <-chan struct{}
}

// Reader allows the caller to inspect the ledger
type Reader interface {
	// Iterator returns an Iterator, as specified by a cb.SeekInfo message, and
	// its starting block number
	Iterator(startType *ab.SeekPosition) (Iterator, uint64)
	// Height returns the number of blocks on the ledger
	Height() uint64
}

// Writer allows the caller to modify the ledger
type Writer interface {
	// Append a new block to the ledger
	Append(block *cb.Block) error
}

// ReadWriter encapsulates the read/write functions of the ledger
//ReadWriter是Reader和Writer组成，orederer中可能实现只读账本和只写账本，读写账本的分类
type ReadWriter interface {
	Reader
	Writer
}
