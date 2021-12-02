/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package pvtdatastorage

import (
	"math"

	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/pvtdatapolicy"
	"github.com/hyperledger/fabric/protos/ledger/rwset"
	"github.com/willf/bitset"
)

func prepareStoreEntries(blockNum uint64, pvtData []*ledger.TxPvtData, btlPolicy pvtdatapolicy.BTLPolicy,

	missingPvtData ledger.TxMissingPvtDataMap) (*storeEntries, error) {
	logger.Info("===prepareStoreEntries=====")
	dataEntries := prepareDataEntries(blockNum, pvtData)

	missingDataEntries := prepareMissingDataEntries(blockNum, missingPvtData)

	expiryEntries, err := prepareExpiryEntries(blockNum, dataEntries, missingDataEntries, btlPolicy)
	if err != nil {
		return nil, err
	}

	return &storeEntries{
		dataEntries:        dataEntries,
		expiryEntries:      expiryEntries,
		missingDataEntries: missingDataEntries}, nil
}

func prepareDataEntries(blockNum uint64, pvtData []*ledger.TxPvtData) []*dataEntry {
	logger.Info("===prepareDataEntries=====")
	var dataEntries []*dataEntry
	for _, txPvtdata := range pvtData {
		for _, nsPvtdata := range txPvtdata.WriteSet.NsPvtRwset {
			for _, collPvtdata := range nsPvtdata.CollectionPvtRwset {
				txnum := txPvtdata.SeqInBlock
				ns := nsPvtdata.Namespace
				coll := collPvtdata.CollectionName
				dataKey := &dataKey{nsCollBlk{ns, coll, blockNum}, txnum}
				dataEntries = append(dataEntries, &dataEntry{key: dataKey, value: collPvtdata})
			}
		}
	}
	return dataEntries
}

func prepareMissingDataEntries(committingBlk uint64, missingPvtData ledger.TxMissingPvtDataMap) map[missingDataKey]*bitset.BitSet {

	logger.Info("===prepareMissingDataEntries=====")

	missingDataEntries := make(map[missingDataKey]*bitset.BitSet)
	for txNum, missingData := range missingPvtData {
		for _, nsColl := range missingData {
			key := missingDataKey{nsCollBlk{nsColl.Namespace, nsColl.Collection, committingBlk},
				nsColl.IsEligible}

			if _, ok := missingDataEntries[key]; !ok {
				missingDataEntries[key] = &bitset.BitSet{}
			}
			bitmap := missingDataEntries[key]

			bitmap.Set(uint(txNum))
		}
	}

	return missingDataEntries
}

// prepareExpiryEntries returns expiry entries for both private data which is present in the committingBlk
// and missing private.
func prepareExpiryEntries(committingBlk uint64, dataEntries []*dataEntry, missingDataEntries map[missingDataKey]*bitset.BitSet,

	btlPolicy pvtdatapolicy.BTLPolicy) ([]*expiryEntry, error) {

	logger.Info("===prepareExpiryEntries=====")
	var expiryEntries []*expiryEntry
	mapByExpiringBlk := make(map[uint64]*ExpiryData)

	// 1. prepare expiryData for non-missing data
	for _, dataEntry := range dataEntries {
		prepareExpiryEntriesForPresentData(mapByExpiringBlk, dataEntry.key, btlPolicy)
	}

	// 2. prepare expiryData for missing data
	for missingDataKey := range missingDataEntries {
		prepareExpiryEntriesForMissingData(mapByExpiringBlk, &missingDataKey, btlPolicy)
	}

	for expiryBlk, expiryData := range mapByExpiringBlk {
		expiryKey := &expiryKey{expiringBlk: expiryBlk, committingBlk: committingBlk}
		expiryEntries = append(expiryEntries, &expiryEntry{key: expiryKey, value: expiryData})
	}

	return expiryEntries, nil
}

// prepareExpiryDataForPresentData creates expiryData for non-missing pvt data
func prepareExpiryEntriesForPresentData(mapByExpiringBlk map[uint64]*ExpiryData, dataKey *dataKey, btlPolicy pvtdatapolicy.BTLPolicy) error {

	logger.Info("===prepareExpiryEntriesForPresentData=====")
	expiringBlk, err := btlPolicy.GetExpiringBlock(dataKey.ns, dataKey.coll, dataKey.blkNum)
	if err != nil {
		return err
	}
	if neverExpires(expiringBlk) {
		return nil
	}

	expiryData := getOrCreateExpiryData(mapByExpiringBlk, expiringBlk)

	expiryData.addPresentData(dataKey.ns, dataKey.coll, dataKey.txNum)
	return nil
}

// prepareExpiryDataForMissingData creates expiryData for missing pvt data
func prepareExpiryEntriesForMissingData(mapByExpiringBlk map[uint64]*ExpiryData, missingKey *missingDataKey, btlPolicy pvtdatapolicy.BTLPolicy) error {
	logger.Info("===prepareExpiryEntriesForMissingData=====")
	expiringBlk, err := btlPolicy.GetExpiringBlock(missingKey.ns, missingKey.coll, missingKey.blkNum)
	if err != nil {
		return err
	}
	if neverExpires(expiringBlk) {
		return nil
	}

	expiryData := getOrCreateExpiryData(mapByExpiringBlk, expiringBlk)

	expiryData.addMissingData(missingKey.ns, missingKey.coll)
	return nil
}

func getOrCreateExpiryData(mapByExpiringBlk map[uint64]*ExpiryData, expiringBlk uint64) *ExpiryData {
	logger.Info("===getOrCreateExpiryData=====")
	expiryData, ok := mapByExpiringBlk[expiringBlk]
	if !ok {
		expiryData = newExpiryData()
		mapByExpiringBlk[expiringBlk] = expiryData
	}
	return expiryData
}

// deriveKeys constructs dataKeys and missingDataKey from an expiryEntry
func deriveKeys(expiryEntry *expiryEntry) (dataKeys []*dataKey, missingDataKeys []*missingDataKey) {
	logger.Info("===deriveKeys=====")
	for ns, colls := range expiryEntry.value.Map {
		// 1. constructs dataKeys of expired existing pvt data
		for coll, txNums := range colls.Map {
			for _, txNum := range txNums.List {
				dataKeys = append(dataKeys,
					&dataKey{nsCollBlk{ns, coll, expiryEntry.key.committingBlk}, txNum})
			}
		}
		// 2. constructs missingDataKeys of expired missing pvt data
		for coll := range colls.MissingDataMap {
			// one key for eligible entries and another for ieligible entries
			missingDataKeys = append(missingDataKeys,
				&missingDataKey{nsCollBlk{ns, coll, expiryEntry.key.committingBlk}, true})
			missingDataKeys = append(missingDataKeys,
				&missingDataKey{nsCollBlk{ns, coll, expiryEntry.key.committingBlk}, false})

		}
	}
	return
}

func passesFilter(dataKey *dataKey, filter ledger.PvtNsCollFilter) bool {
	logger.Info("===passesFilter=====")
	return filter == nil || filter.Has(dataKey.ns, dataKey.coll)
}

func isExpired(key nsCollBlk, btl pvtdatapolicy.BTLPolicy, latestBlkNum uint64) (bool, error) {
	logger.Info("===isExpired=====")
	expiringBlk, err := btl.GetExpiringBlock(key.ns, key.coll, key.blkNum)
	if err != nil {
		return false, err
	}

	return latestBlkNum >= expiringBlk, nil
}

func neverExpires(expiringBlkNum uint64) bool {
	logger.Info("===neverExpires=====")
	return expiringBlkNum == math.MaxUint64
}

type txPvtdataAssembler struct {
	blockNum, txNum uint64
	txWset          *rwset.TxPvtReadWriteSet
	currentNsWSet   *rwset.NsPvtReadWriteSet
	firstCall       bool
}

func newTxPvtdataAssembler(blockNum, txNum uint64) *txPvtdataAssembler {
	logger.Info("===newTxPvtdataAssembler=====")
	return &txPvtdataAssembler{blockNum, txNum, &rwset.TxPvtReadWriteSet{}, nil, true}
}

func (a *txPvtdataAssembler) add(ns string, collPvtWset *rwset.CollectionPvtReadWriteSet) {


	logger.Info("===txPvtdataAssembler===add==")
	// start a NsWset
	if a.firstCall {
		a.currentNsWSet = &rwset.NsPvtReadWriteSet{Namespace: ns}
		a.firstCall = false
	}

	// if a new ns started, add the existing NsWset to TxWset and start a new one
	if a.currentNsWSet.Namespace != ns {
		a.txWset.NsPvtRwset = append(a.txWset.NsPvtRwset, a.currentNsWSet)
		a.currentNsWSet = &rwset.NsPvtReadWriteSet{Namespace: ns}
	}
	// add the collWset to the current NsWset
	a.currentNsWSet.CollectionPvtRwset = append(a.currentNsWSet.CollectionPvtRwset, collPvtWset)
}

func (a *txPvtdataAssembler) done() {
	logger.Info("===txPvtdataAssembler===done==")
	if a.currentNsWSet != nil {
		a.txWset.NsPvtRwset = append(a.txWset.NsPvtRwset, a.currentNsWSet)
	}
	a.currentNsWSet = nil
}

func (a *txPvtdataAssembler) getTxPvtdata() *ledger.TxPvtData {
	logger.Info("===txPvtdataAssembler===getTxPvtdata==")
	a.done()
	return &ledger.TxPvtData{SeqInBlock: a.txNum, WriteSet: a.txWset}
}
