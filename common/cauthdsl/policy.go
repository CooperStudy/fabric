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
	fmt.Println("==deserializeAndVerify==Identity=")
	deserializedIdentity, err := d.deserializer.DeserializeIdentity(d.signedData.Identity)
	if err != nil {
		return nil, err
	}

	d.deserializedIdentity = deserializedIdentity
	return deserializedIdentity, nil
}

func (d *deserializeAndVerify) Verify() error {
	fmt.Println("==deserializeAndVerify==Verify=")
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
	fmt.Println("==NewPolicyProvider=")
	return &provider{
		deserializer: deserializer,
	}
}

// NewPolicy creates a new policy based on the policy bytes
func (pr *provider) NewPolicy(data []byte) (policies.Policy, proto.Message, error) {
	fmt.Println("==NewPolicy=")
	sigPolicy := &cb.SignaturePolicyEnvelope{}
	if err := proto.Unmarshal(data, sigPolicy); err != nil {
		return nil, nil, fmt.Errorf("Error unmarshaling to SignaturePolicy: %s", err)
	}

	if sigPolicy.Version != 0 {
		return nil, nil, fmt.Errorf("This evaluator only understands messages of version 0, but version was %d", sigPolicy.Version)
	}

	compiled, err := compile(sigPolicy.Rule, sigPolicy.Identities, pr.deserializer)
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
	fmt.Println("===policy==Evaluate=")
	if p == nil {
		return fmt.Errorf("No such policy")
	}
	idAndS := make([]IdentityAndSignature, len(signatureSet))
	for i, sd := range signatureSet {
		fmt.Println("===========i",i)//0
		fmt.Println("===========sd",sd)
		/*
		&{[24 5 122 94 18 20 8 245 155 142 143 243 167 144 220 22 16 159 195 178 150 243 167 144 220 22 26 32 80 15 118 229 112 194 46 145 52 58 201 194 141 181 93 105 72 250 164 77 167 16 219 111 167 151 22 65 212 234 40 91 34 32 4 59 188 47 229 180 124 126 24 37 33 69 136 3 20 79 26 33 87 166 41 252 227 244 219 36 52 236 147 79 42 38 42 2 8 1] [10 7 79 114 103 49 77 83 80 18 166 6 45 45 45 45 45 66 69 71 73 78 32 67 69 82 84 73 70 73 67 65 8]
		 */
		idAndS[i] = &deserializeAndVerify{
			signedData:   sd,
			deserializer: p.deserializer,
		}
	}

	ok := p.evaluator(deduplicate(idAndS), make([]bool, len(signatureSet)))
	if !ok {
		return errors.New("signature set did not satisfy policy")
	}
	return nil
}
