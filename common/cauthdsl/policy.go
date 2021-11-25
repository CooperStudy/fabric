/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cauthdsl

import (
	"errors"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/policies"
	"github.com/hyperledger/fabric/msp"
	cb "github.com/hyperledger/fabric/protos/common"
	mspp "github.com/hyperledger/fabric/protos/msp"
)

type Identity interface {
	// SatisfiesPrincipal checks whether this instance matches
	// the description supplied in MSPPrincipal. The check may
	// involve a byte-by-byte comparison (if the principal is
	// a serialized identity) or may require MSP validation
	SatisfiesPrincipal(principal *mspp.MSPPrincipal) error

	// GetIdentifier returns the identifier of that identity
	GetIdentifier() *msp.IdentityIdentifier
}

type IdentityAndSignature interface {
	// Identity returns the identity associated to this instance
	Identity() (Identity, error)

	// Verify returns the validity status of this identity's signature over the message
	Verify() error
}

type deserializeAndVerify struct {
	signedData           *cb.SignedData
	deserializer         msp.IdentityDeserializer
	deserializedIdentity msp.Identity
}

func (d *deserializeAndVerify) Identity() (Identity, error) {
	deserializedIdentity, err := d.deserializer.DeserializeIdentity(d.signedData.Identity)
	if err != nil {
		return nil, err
	}

	d.deserializedIdentity = deserializedIdentity
	return deserializedIdentity, nil
}

func (d *deserializeAndVerify) Verify() error {
	if d.deserializedIdentity == nil {
		cauthdslLogger.Panicf("programming error, Identity must be called prior to Verify")
	}
	return d.deserializedIdentity.Verify(d.signedData.Data, d.signedData.Signature)
}

type provider struct {
	deserializer msp.IdentityDeserializer
}

// NewProviderImpl provides a policy generator for cauthdsl type policies
func NewPolicyProvider(deserializer msp.IdentityDeserializer) policies.Provider {
	cauthdslLogger.Info("==================func NewPolicyProvider(deserializer msp.IdentityDeserializer) policies.Provider==============================")
	return &provider{
		deserializer: deserializer,
	}
}

// NewPolicy creates a new policy based on the policy bytes
func (pr *provider) NewPolicy(data []byte) (policies.Policy, proto.Message, error) {
	cauthdslLogger.Info("======================func (pr *provider) NewPolicy(data []byte) (policies.Policy, proto.Message, error)================================================")
	sigPolicy := &cb.SignaturePolicyEnvelope{}
	if err := proto.Unmarshal(data, sigPolicy); err != nil {
		return nil, nil, fmt.Errorf("Error unmarshaling to SignaturePolicy: %s", err)
	}

	cauthdslLogger.Info("=================sigPolicy=============================",sigPolicy)
	//rule:<n_out_of:<n:2 rules:<signed_by:0 > rules:<signed_by:1 > > > identities:<principal:"\n\007Org1MSP\020\003" > identities:<principal:"\n\007Org2MSP\020\003" >
	/*
	=================sigPolicy=============================
	rule:<n_out_of:<n:2 rules:<signed_by:0 > rules:<signed_by:1 > > > identities:<principal:"\n\007Org1MSP\020\003" > identities:<principal:"\n\007Org2MSP\020\003" >
	*/
	cauthdslLogger.Info("=================sigPolicy.version=============================",sigPolicy.Version)//0
	if sigPolicy.Version != 0 {
		return nil, nil, fmt.Errorf("This evaluator only understands messages of version 0, but version was %d", sigPolicy.Version)
	}

	cauthdslLogger.Infof("======================compiled, err := compile(sigPolicy.Rule:%v, sigPolicy.Identities:%v, pr.deserializer:%v)==\n",sigPolicy.Rule,sigPolicy.Identities,pr.deserializer)
	//======================compiled, err := compile(sigPolicy.Rule:n_out_of:<n:2 rules:<signed_by:0 > rules:<signed_by:1 > > , sigPolicy.Identities:[principal:"\n\007Org1MSP\020\003"  principal:"\n\007Org2MSP\020\003" ], pr.deserializer:&{{0xc00290a980 0xc0005cf630}})==
	compiled, err := compile(sigPolicy.Rule, sigPolicy.Identities, pr.deserializer)
	cauthdslLogger.Info("===========compiled============",compiled)
	/*
	compile
	return func(signedData []IdentityAndSignature, used []bool) bool {
				cauthdslLogger.Info("====================compiledPolicy return   case *cb.SignaturePolicy_NOutOf_ ===func(signedData []IdentityAndSignature, used []bool) bool============================")
				grepKey := time.Now().UnixNano()
				cauthdslLogger.Infof("%p gate %d evaluation starts", signedData, grepKey)
				//0xc0039755a0 gate 1637736265597327441 evaluation starts
				verified := int32(0)
				cauthdslLogger.Info("====================verified============================",verified)//0

				_used := make([]bool, len(used))
				for k, policy := range policies {
					cauthdslLogger.Info("====================k============================",k)//0 1
					cauthdslLogger.Info("====================policies============================",policies)
					// [0xa9ab40 0xa9ab40] [0xa9ab40 0xa9ab40]


					copy(_used, used)
					cauthdslLogger.Info("====================_used============================",_used)//true
					cauthdslLogger.Info("====================a:= policy(signedData, _used) ============================")
					//
					a:= policy(signedData, _used)
					cauthdslLogger.Info("===================a ============================",a)//false

					//true
					//false
					if a {
						cauthdslLogger.Info("===================verified ============================",verified)//0
						verified++
						cauthdslLogger.Info("==================verified++============================")//1
						cauthdslLogger.Info("===================verified ============================",verified)
						cauthdslLogger.Info("===================used ============================",used)//false
						cauthdslLogger.Info("===================_used ============================",_used)//true
						copy(used, _used)
						cauthdslLogger.Info("=================copy(used, _used)==========================")
						cauthdslLogger.Info("===================used ============================",used)//true
						cauthdslLogger.Info("===================_used ============================",_used)//true
					}
				}

				if verified >= t.NOutOf.N {
					cauthdslLogger.Infof("%p gate %d evaluation succeeds", signedData, grepKey)
				} else {
					cauthdslLogger.Infof("%p gate %d evaluation fails", signedData, grepKey)
				}

				return verified >= t.NOutOf.N
			}, nil
	 */

	//policies[i] = compiledPolicy
	cauthdslLogger.Info("===========err============",err)
	if err != nil {
		return nil, nil, err
	}


	return &policy{
		evaluator:    compiled,
		deserializer: pr.deserializer,
	}, sigPolicy, nil

}

type policy struct {
	evaluator    func([]IdentityAndSignature, []bool) bool
	deserializer msp.IdentityDeserializer
}

// Evaluate takes a set of SignedData and evaluates whether this set of signatures satisfies the policy
func (p *policy) Evaluate(signatureSet []*cb.SignedData) error {
	cauthdslLogger.Info("==============func (p *policy) Evaluate(signatureSet []*cb.SignedData) error=============================")
	if p == nil {
		return fmt.Errorf("No such policy")
	}
	idAndS := make([]IdentityAndSignature, len(signatureSet))
	for i, sd := range signatureSet {
		idAndS[i] = &deserializeAndVerify{
			signedData:   sd,
			deserializer: p.deserializer,
		}
	}
	cauthdslLogger.Info("==============ok := p.evaluator(deduplicate(idAndS), make([]bool, len(signatureSet)))=============================")

	de := deduplicate(idAndS)
	cauthdslLogger.Info("==============%v := deduplicate(%v)============================",de,idAndS)

	ok := p.evaluator(de, make([]bool, len(signatureSet)))
	/*
	====================compiledPolicy return   case *cb.SignaturePolicy_NOutOf_ ===func(signedData []IdentityAndSignature, used []bool) bool============================

	*/

	cauthdslLogger.Info("==============ok============================",ok)
	if !ok {
		return errors.New("signature set did not satisfy policy")
	}
	return nil
}
