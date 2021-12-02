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

package cauthdsl

import (
	"fmt"
	"sort"

	"github.com/golang/protobuf/proto"
	cb "github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/msp"
	"github.com/hyperledger/fabric/protos/utils"
)

// AcceptAllPolicy always evaluates to true
var AcceptAllPolicy *cb.SignaturePolicyEnvelope

// MarshaledAcceptAllPolicy is the Marshaled version of AcceptAllPolicy
var MarshaledAcceptAllPolicy []byte

// RejectAllPolicy always evaluates to false
var RejectAllPolicy *cb.SignaturePolicyEnvelope

// MarshaledRejectAllPolicy is the Marshaled version of RejectAllPolicy
var MarshaledRejectAllPolicy []byte

func init() {
	logger.Info("==cauthdsl_builder.go init===")
	var err error

	AcceptAllPolicy = Envelope(NOutOf(0, []*cb.SignaturePolicy{}), [][]byte{})
	MarshaledAcceptAllPolicy, err = proto.Marshal(AcceptAllPolicy)
	if err != nil {
		panic("Error marshaling trueEnvelope")
	}

	RejectAllPolicy = Envelope(NOutOf(1, []*cb.SignaturePolicy{}), [][]byte{})
	MarshaledRejectAllPolicy, err = proto.Marshal(RejectAllPolicy)
	if err != nil {
		panic("Error marshaling falseEnvelope")
	}
}

// Envelope builds an envelope message embedding a SignaturePolicy
func Envelope(policy *cb.SignaturePolicy, identities [][]byte) *cb.SignaturePolicyEnvelope {
	logger.Info("==Envelope===")
	ids := make([]*msp.MSPPrincipal, len(identities))
	for i := range ids {
		ids[i] = &msp.MSPPrincipal{PrincipalClassification: msp.MSPPrincipal_IDENTITY, Principal: identities[i]}
	}

	return &cb.SignaturePolicyEnvelope{
		Version:    0,
		Rule:       policy,
		Identities: ids,
	}
}

// SignedBy creates a SignaturePolicy requiring a given signer's signature
func SignedBy(index int32) *cb.SignaturePolicy {
	logger.Info("==SignedBy===")
	logger.Info("===index",index)
	return &cb.SignaturePolicy{
		Type: &cb.SignaturePolicy_SignedBy{
			SignedBy: index,
		},
	}
}

// SignedByMspMember creates a SignaturePolicyEnvelope
// requiring 1 signature from any member of the specified MSP
func SignedByMspMember(mspId string) *cb.SignaturePolicyEnvelope {
	logger.Info("==SignedByMspMember===")
	return signedByFabricEntity(mspId, msp.MSPRole_MEMBER)
}

// SignedByMspClient creates a SignaturePolicyEnvelope
// requiring 1 signature from any client of the specified MSP
func SignedByMspClient(mspId string) *cb.SignaturePolicyEnvelope {
	logger.Info("==SignedByMspClient===")
	return signedByFabricEntity(mspId, msp.MSPRole_CLIENT)
}

// SignedByMspPeer creates a SignaturePolicyEnvelope
// requiring 1 signature from any peer of the specified MSP
func SignedByMspPeer(mspId string) *cb.SignaturePolicyEnvelope {
	logger.Info("==SignedByMspPeer===")
	return signedByFabricEntity(mspId, msp.MSPRole_PEER)
}

// SignedByFabricEntity creates a SignaturePolicyEnvelope
// requiring 1 signature from any fabric entity, having the passed role, of the specified MSP
func signedByFabricEntity(mspId string, role msp.MSPRole_MSPRoleType) *cb.SignaturePolicyEnvelope {
	logger.Info("==signedByFabricEntity===")
	// specify the principal: it's a member of the msp we just found
	principal := &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               utils.MarshalOrPanic(&msp.MSPRole{Role: role, MspIdentifier: mspId})}

	// create the policy: it requires exactly 1 signature from the first (and only) principal
	p := &cb.SignaturePolicyEnvelope{
		Version:    0,
		Rule:       NOutOf(1, []*cb.SignaturePolicy{SignedBy(0)}),
		Identities: []*msp.MSPPrincipal{principal},
	}

	return p
}

// SignedByMspAdmin creates a SignaturePolicyEnvelope
// requiring 1 signature from any admin of the specified MSP
func SignedByMspAdmin(mspId string) *cb.SignaturePolicyEnvelope {
	logger.Info("==SignedByMspAdmin===")
	// specify the principal: it's a member of the msp we just found
	principal := &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               utils.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_ADMIN, MspIdentifier: mspId})}

	// create the policy: it requires exactly 1 signature from the first (and only) principal
	p := &cb.SignaturePolicyEnvelope{
		Version:    0,
		Rule:       NOutOf(1, []*cb.SignaturePolicy{SignedBy(0)}),
		Identities: []*msp.MSPPrincipal{principal},
	}

	return p
}

//wrapper for generating "any of a given role" type policies
func signedByAnyOfGivenRole(role msp.MSPRole_MSPRoleType, ids []string) *cb.SignaturePolicyEnvelope {
	logger.Info("==signedByAnyOfGivenRole===")
	// we create an array of principals, one principal
	// per application MSP defined on this chain
	sort.Strings(ids)
	principals := make([]*msp.MSPPrincipal, len(ids))
	sigspolicy := make([]*cb.SignaturePolicy, len(ids))
	for i, id := range ids {
		principals[i] = &msp.MSPPrincipal{
			PrincipalClassification: msp.MSPPrincipal_ROLE,
			Principal:               utils.MarshalOrPanic(&msp.MSPRole{Role: role, MspIdentifier: id})}
		sigspolicy[i] = SignedBy(int32(i))
	}

	// create the policy: it requires exactly 1 signature from any of the principals
	p := &cb.SignaturePolicyEnvelope{
		Version:    0,
		Rule:       NOutOf(1, sigspolicy),
		Identities: principals,
	}

	return p
}

// SignedByAnyMember returns a policy that requires one valid
// signature from a member of any of the orgs whose ids are
// listed in the supplied string array
func SignedByAnyMember(ids []string) *cb.SignaturePolicyEnvelope {
	logger.Info("==SignedByAnyMember===")
	return signedByAnyOfGivenRole(msp.MSPRole_MEMBER, ids)
}

// SignedByAnyClient returns a policy that requires one valid
// signature from a client of any of the orgs whose ids are
// listed in the supplied string array
func SignedByAnyClient(ids []string) *cb.SignaturePolicyEnvelope {
	logger.Info("==SignedByAnyClient===")
	return signedByAnyOfGivenRole(msp.MSPRole_CLIENT, ids)
}

// SignedByAnyPeer returns a policy that requires one valid
// signature from an orderer of any of the orgs whose ids are
// listed in the supplied string array
func SignedByAnyPeer(ids []string) *cb.SignaturePolicyEnvelope {
	logger.Info("==SignedByAnyPeer===")
	return signedByAnyOfGivenRole(msp.MSPRole_PEER, ids)
}

// SignedByAnyAdmin returns a policy that requires one valid
// signature from a admin of any of the orgs whose ids are
// listed in the supplied string array
func SignedByAnyAdmin(ids []string) *cb.SignaturePolicyEnvelope {
	logger.Info("==SignedByAnyAdmin===")
	/*
	初始化链码的时候，需要Admin权限
	 */
	return signedByAnyOfGivenRole(msp.MSPRole_ADMIN, ids)
}

// And is a convenience method which utilizes NOutOf to produce And equivalent behavior
func And(lhs, rhs *cb.SignaturePolicy) *cb.SignaturePolicy {
	logger.Info("==And===")
	return NOutOf(2, []*cb.SignaturePolicy{lhs, rhs})
}

// Or is a convenience method which utilizes NOutOf to produce Or equivalent behavior
func Or(lhs, rhs *cb.SignaturePolicy) *cb.SignaturePolicy {
	logger.Info("==Or===")
	logger.Info("===========lhs",lhs)//signed_by:0
	logger.Info("===========rhs.Type",(*rhs).Type)//1
	return NOutOf(1, []*cb.SignaturePolicy{lhs, rhs})
}

// NOutOf creates a policy which requires N out of the slice of policies to evaluate to true
func NOutOf(n int32, policies []*cb.SignaturePolicy) *cb.SignaturePolicy {
	logger.Info("==NOutOf===")
	logger.Info("==============n=====",n)//0
	//1
	logger.Info("=============policies []*cb.SignaturePolicy=====",policies) // [signed_by:0  signed_by:1 ]  [signed_by:2  signed_by:3 ]
	//[signed_by:0 ]
	return &cb.SignaturePolicy{
		Type: &cb.SignaturePolicy_NOutOf_{
			NOutOf: &cb.SignaturePolicy_NOutOf{
				N:     n,
				Rules: policies,
			},
		},
	}
}
