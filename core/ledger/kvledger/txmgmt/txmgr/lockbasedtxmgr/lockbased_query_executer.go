/*
Copyright IBM Corp. All Rights Reserved.
SPDX-License-Identifier: Apache-2.0
*/

package lockbasedtxmgr

import (
	commonledger "github.com/hyperledger/fabric/common/ledger"
	"github.com/hyperledger/fabric/core/ledger"
)

// LockBasedQueryExecutor is a query executor used in `LockBasedTxMgr`
type lockBasedQueryExecutor struct {
	helper *queryHelper
	txid   string
}

func newQueryExecutor(txmgr *LockBasedTxMgr, txid string) *lockBasedQueryExecutor {
	logger.Info("==newQueryExecutor==")
	helper := newQueryHelper(txmgr, nil)
	logger.Infof("constructing new query executor txid = [%s]", txid)
	// constructing new query executor txid = [24fe3b28-cf46-41dc-8c03-b0e72c04fb1a]
	return &lockBasedQueryExecutor{helper, txid}
}

// GetState implements method in interface `ledger.QueryExecutor`
func (q *lockBasedQueryExecutor) GetState(ns string, key string) (val []byte, err error) {
	logger.Info("==lockBasedQueryExecutor==GetState==")
	val, _, err = q.helper.getState(ns, key)
	return
}

// GetStateMetadata implements method in interface `ledger.QueryExecutor`
func (q *lockBasedQueryExecutor) GetStateMetadata(namespace, key string) (map[string][]byte, error) {
	logger.Info("==lockBasedQueryExecutor==GetStateMetadata==")
	return q.helper.getStateMetadata(namespace, key)
}

// GetStateMultipleKeys implements method in interface `ledger.QueryExecutor`
func (q *lockBasedQueryExecutor) GetStateMultipleKeys(namespace string, keys []string) ([][]byte, error) {
	logger.Info("==lockBasedQueryExecutor==GetStateMultipleKeys==")
	return q.helper.getStateMultipleKeys(namespace, keys)
}

// GetStateRangeScanIterator implements method in interface `ledger.QueryExecutor`
// startKey is included in the results and endKey is excluded. An empty startKey refers to the first available key
// and an empty endKey refers to the last available key. For scanning all the keys, both the startKey and the endKey
// can be supplied as empty strings. However, a full scan shuold be used judiciously for performance reasons.
func (q *lockBasedQueryExecutor) GetStateRangeScanIterator(namespace string, startKey string, endKey string) (commonledger.ResultsIterator, error) {
	logger.Info("==lockBasedQueryExecutor==GetStateRangeScanIterator==")
	return q.helper.getStateRangeScanIterator(namespace, startKey, endKey)
}

// GetStateRangeScanIteratorWithMetadata implements method in interface `ledger.QueryExecutor`
// startKey is included in the results and endKey is excluded. An empty startKey refers to the first available key
// and an empty endKey refers to the last available key. For scanning all the keys, both the startKey and the endKey
// can be supplied as empty strings. However, a full scan shuold be used judiciously for performance reasons.
// metadata is a map of additional query parameters
func (q *lockBasedQueryExecutor) GetStateRangeScanIteratorWithMetadata(namespace string, startKey string, endKey string, metadata map[string]interface{}) (ledger.QueryResultsIterator, error) {
	logger.Info("==lockBasedQueryExecutor==GetStateRangeScanIteratorWithMetadata==")
	return q.helper.getStateRangeScanIteratorWithMetadata(namespace, startKey, endKey, metadata)
}

// ExecuteQuery implements method in interface `ledger.QueryExecutor`
func (q *lockBasedQueryExecutor) ExecuteQuery(namespace, query string) (commonledger.ResultsIterator, error) {
	logger.Info("==lockBasedQueryExecutor==ExecuteQuery==")
	return q.helper.executeQuery(namespace, query)
}

// ExecuteQueryWithMetadata implements method in interface `ledger.QueryExecutor`
func (q *lockBasedQueryExecutor) ExecuteQueryWithMetadata(namespace, query string, metadata map[string]interface{}) (ledger.QueryResultsIterator, error) {
	logger.Info("==lockBasedQueryExecutor==ExecuteQueryWithMetadata==")
	return q.helper.executeQueryWithMetadata(namespace, query, metadata)
}

// GetPrivateData implements method in interface `ledger.QueryExecutor`
func (q *lockBasedQueryExecutor) GetPrivateData(namespace, collection, key string) ([]byte, error) {
	logger.Info("==lockBasedQueryExecutor==GetPrivateData==")
	return q.helper.getPrivateData(namespace, collection, key)
}

// GetPrivateDataMetadata implements method in interface `ledger.QueryExecutor`
func (q *lockBasedQueryExecutor) GetPrivateDataMetadata(namespace, collection, key string) (map[string][]byte, error) {
	logger.Info("==lockBasedQueryExecutor==GetPrivateDataMetadata==")
	return q.helper.getPrivateDataMetadata(namespace, collection, key)
}

// GetPrivateDataMetadataByHash implements method in interface `ledger.QueryExecutor`
func (q *lockBasedQueryExecutor) GetPrivateDataMetadataByHash(namespace, collection string, keyhash []byte) (map[string][]byte, error) {
	logger.Info("==lockBasedQueryExecutor==GetPrivateDataMetadataByHash==")
	return q.helper.getPrivateDataMetadataByHash(namespace, collection, keyhash)
}

// GetPrivateDataMultipleKeys implements method in interface `ledger.QueryExecutor`
func (q *lockBasedQueryExecutor) GetPrivateDataMultipleKeys(namespace, collection string, keys []string) ([][]byte, error) {
	logger.Info("==lockBasedQueryExecutor==GetPrivateDataMultipleKeys==")
	return q.helper.getPrivateDataMultipleKeys(namespace, collection, keys)
}

// GetPrivateDataRangeScanIterator implements method in interface `ledger.QueryExecutor`
func (q *lockBasedQueryExecutor) GetPrivateDataRangeScanIterator(namespace, collection, startKey, endKey string) (commonledger.ResultsIterator, error) {
	logger.Info("==lockBasedQueryExecutor==GetPrivateDataRangeScanIterator==")
	return q.helper.getPrivateDataRangeScanIterator(namespace, collection, startKey, endKey)
}

// ExecuteQueryOnPrivateData implements method in interface `ledger.QueryExecutor`
func (q *lockBasedQueryExecutor) ExecuteQueryOnPrivateData(namespace, collection, query string) (commonledger.ResultsIterator, error) {
	logger.Info("==lockBasedQueryExecutor==ExecuteQueryOnPrivateData==")
	return q.helper.executeQueryOnPrivateData(namespace, collection, query)
}

// Done implements method in interface `ledger.QueryExecutor`
func (q *lockBasedQueryExecutor) Done() {
	logger.Info("==lockBasedQueryExecutor==Done==")
	logger.Infof("Done with transaction simulation / query execution [%s]", q.txid)
	q.helper.done()
}
