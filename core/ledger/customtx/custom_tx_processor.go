/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package customtx

import (
	"github.com/hyperledger/fabric/common/flogging"
	"sync"

	"github.com/hyperledger/fabric/protos/common"
)

var processors Processors
var once sync.Once
var logger = flogging.MustGetLogger("core.ledger.customtx")
// Processors maintains the association between a custom transaction type to its corresponding tx processor
type Processors map[common.HeaderType]Processor

// Initialize sets the custom processors. This function is expected to be invoked only during ledgermgmt.Initialize() function.
func Initialize(customTxProcessors Processors) {
	logger.Info("==Initialize======")
	once.Do(func() {
		initialize(customTxProcessors)
	})
}

func initialize(customTxProcessors Processors) {
	logger.Info("==initialize======")
	processors = customTxProcessors
}

// GetProcessor returns a Processor associated with the txType
func GetProcessor(txType common.HeaderType) Processor {
	logger.Info("==GetProcessorf======")
	return processors[txType]
}
