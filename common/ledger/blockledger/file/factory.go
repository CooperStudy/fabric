/*
Copyright IBM Corp. 2017 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

                 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package fileledger

import (
	"sync"

	"github.com/hyperledger/fabric/common/ledger/blkstorage"
	"github.com/hyperledger/fabric/common/ledger/blkstorage/fsblkstorage"
	"github.com/hyperledger/fabric/common/ledger/blockledger"
)

type fileLedgerFactory struct {
	blkstorageProvider blkstorage.BlockStoreProvider
	ledgers            map[string]blockledger.ReadWriter
	mutex              sync.Mutex
}

// GetOrCreate gets an existing ledger (if it exists) or creates it if it does not
func (flf *fileLedgerFactory) GetOrCreate(chainID string) (blockledger.ReadWriter, error) {
	//logger.Info("=========fileLedgerFactory=======GetOrCreate=======")
	flf.mutex.Lock()
	defer flf.mutex.Unlock()

	key := chainID
	// check cache
	ledger, ok := flf.ledgers[key]
	//logger.Info("==========ok=======",ok)
	if ok {
		//logger.Info("======if ledge exist,return ledger========")
		return ledger, nil
	}
	// open fresh
	blockStore, err := flf.blkstorageProvider.OpenBlockStore(key)
	if err != nil {
		return nil, err
	}
	ledger = NewFileLedger(blockStore)
	//logger.Info("=====flf.ledgers[key] = ledger==============",key)
	flf.ledgers[key] = ledger
	return ledger, nil
}

// ChainIDs returns the chain IDs the factory is aware of
func (flf *fileLedgerFactory) ChainIDs() []string {
	//logger.Info("=========fileLedgerFactory=======ChainIDs=======")
	chainIDs, err := flf.blkstorageProvider.List()
	//logger.Info("=====chainIDs========",chainIDs)
	if err != nil {
		logger.Panic(err)
	}
	return chainIDs
}

// Close releases all resources acquired by the factory
func (flf *fileLedgerFactory) Close() {
	//logger.Info("=========fileLedgerFactory=======Close=======")
	flf.blkstorageProvider.Close()
}

// New creates a new ledger factory
func New(directory string) blockledger.Factory {
	//logger.Info("=========New======")
	return &fileLedgerFactory{
		blkstorageProvider: fsblkstorage.NewProvider(
			fsblkstorage.NewConf(directory, -1),
			&blkstorage.IndexConfig{
				AttrsToIndex: []blkstorage.IndexableAttr{blkstorage.IndexableAttrBlockNum}},
		),
		ledgers: make(map[string]blockledger.ReadWriter),
	}
}
