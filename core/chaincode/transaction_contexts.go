/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package chaincode

import (
	"context"
	"sync"

	commonledger "github.com/hyperledger/fabric/common/ledger"
	"github.com/hyperledger/fabric/core/common/ccprovider"
	"github.com/hyperledger/fabric/core/ledger"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/pkg/errors"
)

type key string

const (
	// TXSimulatorKey is the context key used to provide a ledger.TxSimulator
	// from the endorser to the chaincode.
	TXSimulatorKey key = "txsimulatorkey"

	// HistoryQueryExecutorKey is the context key used to provide a
	// ledger.HistoryQueryExecutor from the endorser to the chaincode.
	HistoryQueryExecutorKey key = "historyqueryexecutorkey"
)

// TransactionContexts maintains active transaction contexts for a Handler.
type TransactionContexts struct {
	mutex    sync.Mutex
	contexts map[string]*TransactionContext
}

// NewTransactionContexts creates a registry for active transaction contexts.
func NewTransactionContexts() *TransactionContexts {
	logger.Info("======NewTransactionContexts=======")
	return &TransactionContexts{
		contexts: map[string]*TransactionContext{},
	}
}

// contextID creates a transaction identifier that is scoped to a chain.
func contextID(chainID, txID string) string {
	logger.Info("======contextID=======")
	return chainID + txID
}

// Create creates a new TransactionContext for the specified chain and
// transaction ID. An error is returned when a transaction context has already
// been created for the specified chain and transaction ID.
func (c *TransactionContexts) Create(txParams *ccprovider.TransactionParams) (*TransactionContext, error) {
	logger.Info("=========TransactionContexts==Create==============")
	c.mutex.Lock()
	defer c.mutex.Unlock()

	ctxID := contextID(txParams.ChannelID, txParams.TxID)
	if c.contexts[ctxID] != nil {
		return nil, errors.Errorf("txid: %s(%s) exists", txParams.TxID, txParams.ChannelID)
	}

	txctx := &TransactionContext{
		ChainID:              txParams.ChannelID,
		SignedProp:           txParams.SignedProp,
		Proposal:             txParams.Proposal,
		ResponseNotifier:     make(chan *pb.ChaincodeMessage, 1),
		TXSimulator:          txParams.TXSimulator,
		HistoryQueryExecutor: txParams.HistoryQueryExecutor,
		CollectionStore:      txParams.CollectionStore,
		IsInitTransaction:    txParams.IsInitTransaction,

		queryIteratorMap:    map[string]commonledger.ResultsIterator{},
		pendingQueryResults: map[string]*PendingQueryResult{},

		AllowedCollectionAccess: make(map[string]bool),
	}
	c.contexts[ctxID] = txctx

	return txctx, nil
}

func getTxSimulator(ctx context.Context) ledger.TxSimulator {
	logger.Info("======getTxSimulator=======")
	if txsim, ok := ctx.Value(TXSimulatorKey).(ledger.TxSimulator); ok {
		return txsim
	}
	return nil
}

func getHistoryQueryExecutor(ctx context.Context) ledger.HistoryQueryExecutor {
	logger.Info("======getHistoryQueryExecutor=======")
	if historyQueryExecutor, ok := ctx.Value(HistoryQueryExecutorKey).(ledger.HistoryQueryExecutor); ok {
		return historyQueryExecutor
	}
	return nil
}

// Get retrieves the transaction context associated with the chain and
// transaction ID.
func (c *TransactionContexts) Get(chainID, txID string) *TransactionContext {
	logger.Info("======TransactionContexts====Get===")
	ctxID := contextID(chainID, txID)
	c.mutex.Lock()
	tc := c.contexts[ctxID]
	c.mutex.Unlock()
	return tc
}

// Delete removes the transaction context associated with the specified chain
// and transaction ID.
func (c *TransactionContexts) Delete(chainID, txID string) {
	logger.Info("======TransactionContexts====Delete===")
	ctxID := contextID(chainID, txID)
	c.mutex.Lock()
	delete(c.contexts, ctxID)
	c.mutex.Unlock()
}

// Close closes all query iterators assocated with the context.
func (c *TransactionContexts) Close() {
	logger.Info("======TransactionContexts====Close===")
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, txctx := range c.contexts {
		txctx.CloseQueryIterators()
	}
}
