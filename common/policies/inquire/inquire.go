/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package inquire

import (
	"fmt"

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/common/graph"
	"github.com/hyperledger/fabric/common/policies"
	"github.com/hyperledger/fabric/protos/common"
)

var logger = flogging.MustGetLogger("policies.inquire")

type inquireableSignaturePolicy struct {
	sigPol *common.SignaturePolicyEnvelope
}

// NewInquireableSignaturePolicy creates a signature policy that can be inquired,
// from a policy and a signature policy.
func NewInquireableSignaturePolicy(sigPol *common.SignaturePolicyEnvelope) policies.InquireablePolicy {
	logger.Info("======NewInquireableSignaturePolicy=====")
	return &inquireableSignaturePolicy{
		sigPol: sigPol,
	}
}

// SatisfiedBy returns a slice of PrincipalSets that each of them
// satisfies the policy.
func (isp *inquireableSignaturePolicy) SatisfiedBy() []policies.PrincipalSet {
	logger.Info("===inquireableSignaturePolicy===SatisfiedBy=====")
	rootId := fmt.Sprintf("%d", 0)
	root := graph.NewTreeVertex(rootId, isp.sigPol.Rule)
	computePolicyTree(root)
	var res []policies.PrincipalSet
	for _, perm := range root.ToTree().Permute() {
		principalSet := principalsOfTree(perm, isp.sigPol.Identities)
		if len(principalSet) == 0 {
			return nil
		}
		res = append(res, principalSet)
	}
	return res
}

func principalsOfTree(tree *graph.Tree, principals policies.PrincipalSet) policies.PrincipalSet {
	logger.Info("===principalsOfTree=====")
	var principalSet policies.PrincipalSet
	i := tree.BFS()
	for {
		v := i.Next()
		if v == nil {
			break
		}
		if !v.IsLeaf() {
			continue
		}
		pol := v.Data.(*common.SignaturePolicy)
		switch principalIndex := pol.Type.(type) {
		case *common.SignaturePolicy_SignedBy:
			logger.Info("==============*common.SignaturePolicy_SignedBy===============")
			if len(principals) <= int(principalIndex.SignedBy) {
				logger.Warning("Failed computing principalsOfTree, index out of bounds")
				return nil
			}
			principal := principals[principalIndex.SignedBy]
			principalSet = append(principalSet, principal)
		default:
			logger.Info("=========default===================")
			// Leaf vertex is not of type SignedBy
			logger.Info("Leaf vertex", v.Id, "is of type", pol.GetType())
			return nil
		}
	}
	return principalSet
}

func computePolicyTree(v *graph.TreeVertex) {
	logger.Info("===computePolicyTree=====")
	sigPol := v.Data.(*common.SignaturePolicy)
	if p := sigPol.GetNOutOf(); p != nil {
		v.Threshold = int(p.N)
		for i, rule := range p.Rules {
			id := fmt.Sprintf("%s.%d", v.Id, i)
			u := v.AddDescendant(graph.NewTreeVertex(id, rule))
			computePolicyTree(u)
		}
	}
}
