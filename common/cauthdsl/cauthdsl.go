/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cauthdsl

import (
	"fmt"
	"time"

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/msp"
	cb "github.com/hyperledger/fabric/protos/common"
	mb "github.com/hyperledger/fabric/protos/msp"
	"go.uber.org/zap/zapcore"
)

var cauthdslLogger = flogging.MustGetLogger("cauthdsl")

// deduplicate removes any duplicated identities while otherwise preserving identity order
func deduplicate(sds []IdentityAndSignature) []IdentityAndSignature {
	logger.Info("==deduplicate===")
	ids := make(map[string]struct{})
	result := make([]IdentityAndSignature, 0, len(sds))
	for i, sd := range sds {
		logger.Info("=========i",i)//0
		logger.Info("=========sd",sd)
		/*
		&{0xc0002169b0 0xc00060b440 <nil>}
		*/
		identity, err := sd.Identity()
		logger.Info("=======identity========",identity)//&{0xc0001bb500 0xc0003a8f60}
		if err != nil {
			cauthdslLogger.Errorf("Principal deserialization failure (%s) for identity %d", err, i)
			continue
		}
		key := identity.GetIdentifier().Mspid + identity.GetIdentifier().Id

		logger.Info("=======================",identity.GetIdentifier().Mspid) //Org1MSP
		logger.Info("=======================",identity.GetIdentifier().Id)    //a2a4c9bcba1dc3e3f8f11e4bbad67f564f0045eaf91005836304bbb23f5f7329
		if _, ok := ids[key]; ok {
			logger.Info("--------ok",ok)
			fmt.Printf("De-duplicating identity [%s] at index %d in signature set\n", key, i)
		} else {
			result = append(result, sd)
			ids[key] = struct{}{}
		}
	}
	logger.Info("========result======",result)//[0xc0001bb110]
	return result
}

// compile recursively builds a go evaluatable function corresponding to the policy specified, remember to call deduplicate on identities before
// passing them to this function for evaluation
func compile(policy *cb.SignaturePolicy, identities []*mb.MSPPrincipal, deserializer msp.IdentityDeserializer) (func([]IdentityAndSignature, []bool) bool, error) {
	logger.Info("==compile===")
	if policy == nil {
		return nil, fmt.Errorf("Empty policy element")
	}

	switch t := policy.Type.(type) {
	case *cb.SignaturePolicy_NOutOf_:
		policies := make([]func([]IdentityAndSignature, []bool) bool, len(t.NOutOf.Rules))
		for i, policy := range t.NOutOf.Rules {
			compiledPolicy, err := compile(policy, identities, deserializer)
			if err != nil {
				return nil, err
			}
			policies[i] = compiledPolicy

		}
		return func(signedData []IdentityAndSignature, used []bool) bool {
			grepKey := time.Now().UnixNano()
			cauthdslLogger.Debugf("%p gate %d evaluation starts", signedData, grepKey)
			verified := int32(0)
			_used := make([]bool, len(used))
			for _, policy := range policies {
				copy(_used, used)
				if policy(signedData, _used) {
					verified++
					copy(used, _used)
				}
			}

			if verified >= t.NOutOf.N {
				cauthdslLogger.Debugf("%p gate %d evaluation succeeds", signedData, grepKey)
			} else {
				cauthdslLogger.Debugf("%p gate %d evaluation fails", signedData, grepKey)
			}

			return verified >= t.NOutOf.N
		}, nil
	case *cb.SignaturePolicy_SignedBy:
		if t.SignedBy < 0 || t.SignedBy >= int32(len(identities)) {
			return nil, fmt.Errorf("identity index out of range, requested %v, but identies length is %d", t.SignedBy, len(identities))
		}
		signedByID := identities[t.SignedBy]
		return func(signedData []IdentityAndSignature, used []bool) bool {
			cauthdslLogger.Debugf("%p signed by %d principal evaluation starts (used %v)", signedData, t.SignedBy, used)
			for i, sd := range signedData {
				if used[i] {
					cauthdslLogger.Debugf("%p skipping identity %d because it has already been used", signedData, i)
					continue
				}
				if cauthdslLogger.IsEnabledFor(zapcore.DebugLevel) {
					// Unlike most places, this is a huge print statement, and worth checking log level before create garbage
					cauthdslLogger.Debugf("%p processing identity %d with bytes of %x", signedData, i, sd.Identity)
				}
				identity, err := sd.Identity()
				if err != nil {
					cauthdslLogger.Errorf("Principal deserialization failure (%s) for identity %d", err, i)
					continue
				}
				err = identity.SatisfiesPrincipal(signedByID)
				if err != nil {
					cauthdslLogger.Debugf("%p identity %d does not satisfy principal: %s", signedData, i, err)
					continue
				}
				cauthdslLogger.Debugf("%p principal matched by identity %d", signedData, i)
				err = sd.Verify()
				if err != nil {
					cauthdslLogger.Debugf("%p signature for identity %d is invalid: %s", signedData, i, err)
					continue
				}
				cauthdslLogger.Debugf("%p principal evaluation succeeds for identity %d", signedData, i)
				used[i] = true
				return true
			}
			cauthdslLogger.Debugf("%p principal evaluation fails", signedData)
			return false
		}, nil
	default:
		return nil, fmt.Errorf("Unknown type: %T:%v", t, t)
	}
}
