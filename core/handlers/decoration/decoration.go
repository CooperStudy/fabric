/*
Copyright IBM Corp, SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package decoration

import (
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/protos/peer"
)

// Decorator decorates a chaincode input
type Decorator interface {
	// Decorate decorates a chaincode input by changing it
	Decorate(proposal *peer.Proposal, input *peer.ChaincodeInput) *peer.ChaincodeInput
}
var logger = flogging.MustGetLogger("core.handlers.decoration.decorator")
// Apply decorators in the order provided
func Apply(proposal *peer.Proposal, input *peer.ChaincodeInput, decorators ...Decorator) *peer.ChaincodeInput {
	logger.Info("=======Apply==================")
	for _, decorator := range decorators {
		input = decorator.Decorate(proposal, input)
	}

	return input
}
