/*
Copyright IBM Corp. 2016 All Rights Reserved.

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

package fsblkstorage

import (
	"sync"

	"github.com/hyperledger/fabric/common/ledger"
)

// blocksItr - an iterator for iterating over a sequence of blocks
type blocksItr struct {
	mgr                  *blockfileMgr
	maxBlockNumAvailable uint64
	blockNumToRetrieve   uint64
	stream               *blockStream
	closeMarker          bool
	closeMarkerLock      *sync.Mutex
}

func newBlockItr(mgr *blockfileMgr, startBlockNum uint64) *blocksItr {
	logger.Info("===newBlockItr==")
	mgr.cpInfoCond.L.Lock()
	defer mgr.cpInfoCond.L.Unlock()
	logger.Info("============mgr.cpInfo.lastBlockNumber==============",mgr.cpInfo.lastBlockNumber)
	logger.Info("============startBlockNum==============",startBlockNum)
	return &blocksItr{mgr, mgr.cpInfo.lastBlockNumber, startBlockNum, nil, false, &sync.Mutex{}}
}

func (itr *blocksItr) waitForBlock(blockNum uint64) uint64 {
	logger.Info("==blocksItr=waitForBlock==")
	itr.mgr.cpInfoCond.L.Lock()
	defer itr.mgr.cpInfoCond.L.Unlock()
	for itr.mgr.cpInfo.lastBlockNumber < blockNum && !itr.shouldClose() {
		logger.Info("Going to wait for newer blocks. maxAvailaBlockNumber=[%d], waitForBlockNum=[%d]",
			itr.mgr.cpInfo.lastBlockNumber, blockNum)
		itr.mgr.cpInfoCond.Wait()
		logger.Info("Came out of wait. maxAvailaBlockNumber=[%d]", itr.mgr.cpInfo.lastBlockNumber)
	}
	logger.Info("==========itr.mgr.cpInfo.lastBlockNumber===========",itr.mgr.cpInfo.lastBlockNumber)
	return itr.mgr.cpInfo.lastBlockNumber
}

func (itr *blocksItr) initStream() error {
	logger.Info("==blocksItr=initStream==")
	var lp *fileLocPointer
	var err error
	if lp, err = itr.mgr.index.getBlockLocByBlockNum(itr.blockNumToRetrieve); err != nil {
		return err
	}
	if itr.stream, err = newBlockStream(itr.mgr.rootDir, lp.fileSuffixNum, int64(lp.offset), -1); err != nil {
		return err
	}
	return nil
}

func (itr *blocksItr) shouldClose() bool {
	logger.Info("==blocksItr=shouldClose==")
	itr.closeMarkerLock.Lock()
	defer itr.closeMarkerLock.Unlock()
	return itr.closeMarker
}

// Next moves the cursor to next block and returns true iff the iterator is not exhausted
func (itr *blocksItr) Next() (ledger.QueryResult, error) {
	logger.Info("==blocksItr=Next==")
	logger.Info("===========itr.maxBlockNumAvailable==============",itr.maxBlockNumAvailable)
	logger.Info("===========itr.blockNumToRetrieve==============",itr.blockNumToRetrieve)
	if itr.maxBlockNumAvailable < itr.blockNumToRetrieve {
		logger.Info("=============wait block number===",itr.blockNumToRetrieve)
		itr.maxBlockNumAvailable = itr.waitForBlock(itr.blockNumToRetrieve)
	}
	itr.closeMarkerLock.Lock()
	defer itr.closeMarkerLock.Unlock()
	if itr.closeMarker {
		return nil, nil
	}
	if itr.stream == nil {
		logger.Info("======itr.stream == nil=======")
		/*
		create Channel
		 */
		logger.Debugf("Initializing block stream for iterator. itr.maxBlockNumAvailable=%d", itr.maxBlockNumAvailable)
		if err := itr.initStream(); err != nil {
			return nil, err
		}
	}
	nextBlockBytes, err := itr.stream.nextBlockBytes()
	if err != nil {
		return nil, err
	}
	itr.blockNumToRetrieve++
	return deserializeBlock(nextBlockBytes)
}

// Close releases any resources held by the iterator
func (itr *blocksItr) Close() {
	logger.Info("==blocksItr=Close==")
	itr.mgr.cpInfoCond.L.Lock()
	defer itr.mgr.cpInfoCond.L.Unlock()
	itr.closeMarkerLock.Lock()
	defer itr.closeMarkerLock.Unlock()
	itr.closeMarker = true
	itr.mgr.cpInfoCond.Broadcast()
	if itr.stream != nil {
		itr.stream.close()
	}
}
