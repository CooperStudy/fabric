/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package filter

import (
	"context"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/handlers/auth"
	"github.com/hyperledger/fabric/protos/peer"
)
var filterLogger = flogging.MustGetLogger("core.handlers.auth.filter")
// NewFilter creates a new Filter
func NewFilter() auth.Filter {
	//filterLogger.Info("==NewFilter===")
	return &filter{}
}

type filter struct {
	next peer.EndorserServer
}

//Init initializes the Filter with the next EndorserServer
func (f *filter) Init(next peer.EndorserServer) {
	filterLogger.Info("==filter==Init=")
	filterLogger.Infof("==next type:%T==",next)
	f.next = next
}

// ProcessProposal processes a signed proposal
func (f *filter) ProcessProposal(ctx context.Context, signedProp *peer.SignedProposal) (*peer.ProposalResponse, error) {
	filterLogger.Info("==filter==ProcessProposal=")
	return f.next.ProcessProposal(ctx, signedProp)
}
