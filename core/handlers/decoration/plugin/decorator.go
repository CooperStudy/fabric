/*
Copyright IBM Corp, SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"fmt"
	"github.com/hyperledger/fabric/core/handlers/decoration"
	"github.com/hyperledger/fabric/protos/peer"
)

// NewDecorator creates a new decorator
func NewDecorator() decoration.Decorator {
	logger.Info("==NewDecorator=")
	return &decorator{}
}

type decorator struct {
}

// Decorate decorates a chaincode input by changing it
func (d *decorator) Decorate(proposal *peer.Proposal, input *peer.ChaincodeInput) *peer.ChaincodeInput {
	logger.Info("==decorator=Decorate==")
	return input
}

func main() {
}
