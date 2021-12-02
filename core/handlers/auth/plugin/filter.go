/*
Copyright IBM Corp, SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"context"
	"github.com/hyperledger/fabric/common/flogging"

	"github.com/hyperledger/fabric/core/handlers/auth"
	"github.com/hyperledger/fabric/protos/peer"
)

var logger = flogging.MustGetLogger("core.handlers.auth.plugin")

// NewFilter creates a new Filter
func NewFilter() auth.Filter {
	logger.Info("====NewFilter=")
	return &filter{}
}

type filter struct {
	next peer.EndorserServer
}

// Init initializes the Filter with the next EndorserServer
func (f *filter) Init(next peer.EndorserServer) {
	logger.Info("====filter=Init==")
	f.next = next
}

// ProcessProposal processes a signed proposal
func (f *filter) ProcessProposal(ctx context.Context, signedProp *peer.SignedProposal) (*peer.ProposalResponse, error) {
	logger.Info("====filter=ProcessProposal==")
	return f.next.ProcessProposal(ctx, signedProp)
}

func main() {
}
