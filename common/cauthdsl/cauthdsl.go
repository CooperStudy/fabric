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
	cauthdslLogger.Info("===============func deduplicate(sds []IdentityAndSignature) []IdentityAndSignature=====================================")
	ids := make(map[string]struct{})
	result := make([]IdentityAndSignature, 0, len(sds))
	for i, sd := range sds {
		identity, err := sd.Identity()
		if err != nil {
			cauthdslLogger.Errorf("Principal deserialization failure (%s) for identity %d", err, i)
			continue
		}
		key := identity.GetIdentifier().Mspid + identity.GetIdentifier().Id

		if _, ok := ids[key]; ok {
			cauthdslLogger.Warningf("De-duplicating identity [%s] at index %d in signature set", key, i)
		} else {
			result = append(result, sd)
			ids[key] = struct{}{}
		}
	}
	return result
}

// compile recursively builds a go evaluatable function corresponding to the policy specified, remember to call deduplicate on identities before
// passing them to this function for evaluation
/*
	compiled, err := compile(sigPolicy.Rule:n_out_of:<n:2 rules:<signed_by:0 > rules:<signed_by:1 > > , sigPolicy.Identities:[principal:"\n\007Org1MSP\020\003"  principal:"\n\007Org2MSP\020\003" ], pr.deserializer:&{{0xc00290a980 0xc0005cf630}})==

 */
func compile(policy *cb.SignaturePolicy, identities []*mb.MSPPrincipal, deserializer msp.IdentityDeserializer) (func([]IdentityAndSignature, []bool) bool, error) {
	cauthdslLogger.Info("===============func compile(policy *cb.SignaturePolicy, identities []*mb.MSPPrincipal, deserializer msp.IdentityDeserializer) (func([]IdentityAndSignature, []bool) bool, error) ===================")
	if policy == nil {
		return nil, fmt.Errorf("Empty policy element")
	}


	switch t := policy.Type.(type) {
	case *cb.SignaturePolicy_NOutOf_:
		//invoke: 1 go here
		cauthdslLogger.Info("==============case *cb.SignaturePolicy_NOutOf_:===================")


		policies := make([]func([]IdentityAndSignature, []bool) bool, len(t.NOutOf.Rules))
		for i, policy := range t.NOutOf.Rules {
			cauthdslLogger.Info("==============i:===================",i)//0
			cauthdslLogger.Info("==============policy===================",policy)// signed_by:0
			cauthdslLogger.Infof("=====2   compiledPolicy, err := compile(%v, %v, %v)==================",policy,identities,deserializer)// signed_by:0
			compiledPolicy, err := compile(policy, identities, deserializer)
			//=====compiledPolicy==================
			cauthdslLogger.Infof("=====compiledPolicy:%T==================",compiledPolicy)
			//=====compiledPolicy:func([]cauthdsl.IdentityAndSignature, []bool) bool==================
			cauthdslLogger.Info("=====err==================",err) //nil
			if err != nil {
				return nil, err
			}
			policies[i] = compiledPolicy
		}
		//返回的是函數不執行
		return func(signedData []IdentityAndSignature, used []bool) bool {
			cauthdslLogger.Info("====================compiledPolicy return   case *cb.SignaturePolicy_NOutOf_ ===func(signedData []IdentityAndSignature, used []bool) bool============================")
			grepKey := time.Now().UnixNano()
			cauthdslLogger.Infof("%p gate %d evaluation starts", signedData, grepKey)
			//
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
	case *cb.SignaturePolicy_SignedBy:
		//invoke 2 go here
		//signed_by 0
		//signed_by 1
		cauthdslLogger.Info("=============case *cb.SignaturePolicy_SignedBy:===================")
		if t.SignedBy < 0 || t.SignedBy >= int32(len(identities)) {
			return nil, fmt.Errorf("identity index out of range, requested %v, but identies length is %d", t.SignedBy, len(identities))
		}
		cauthdslLogger.Info("=============signedByID := identities[t.SignedBy]===================")
		cauthdslLogger.Info("=============t.SignedBy===================",t.SignedBy) //0 //1
		cauthdslLogger.Info("============identities===================",identities)
		//[principal:"\n\007Org1MSP\020\003"  principal:"\n\007Org2MSP\020\003" ]
		//[principal:"\n\007Org1MSP\020\003"  principal:"\n\007Org2MSP\020\003" ]

		signedByID := identities[t.SignedBy]
		cauthdslLogger.Info("============signedByID===================",signedByID)
		//principal:"\n\007Org1MSP\020\003" = identities[0]
		//principal:"\n\007Org1MSP\020\003" = identities[1]
		return func(signedData []IdentityAndSignature, used []bool) bool {
			cauthdslLogger.Info("====================compiledPolicy return   case *cb.SignaturePolicy_SignedBy: ===func(signedData []IdentityAndSignature, used []bool) bool============================")
			cauthdslLogger.Infof("%p signed by %d principal evaluation starts (used %v)", signedData, t.SignedBy, used)
			//0xc0039755a0 signed by 0 principal evaluation starts (used [false])
			// 0xc0039755a0 signed by 1 principal evaluation starts (used [true])
			for i, sd := range signedData {
				cauthdslLogger.Info("================i=============",i)//0
				cauthdslLogger.Info("================sd=============",sd)
				//&{0xc0042d99a0 0xc0029fc900 0xc00404b660}
				//&{0xc0042d99a0 0xc0029fc900 0xc00404b8e0}
				cauthdslLogger.Infof("================used[%v]:%v=============",i,used[i])//used[0]:false
				//================used[0]:true=============
				//================used[0]:true=============
				if used[i] {
					cauthdslLogger.Infof("%p skipping identity %d because it has already been used", signedData, i)
					//INFO 1186d 0xc0039755a0 skipping identity 0 because it has already been used
					//0xc0039755a0 skipping identity 0 because it has already been used
					continue
				}
				if cauthdslLogger.IsEnabledFor(zapcore.DebugLevel) {
					// Unlike most places, this is a huge print statement, and worth checking log level before create garbage
					cauthdslLogger.Debugf("%p processing identity %d with bytes of %x", signedData, i, sd.Identity)
				}
				identity, err := sd.Identity()
				cauthdslLogger.Info("=============identity, err := sd.Identity()=============")
				cauthdslLogger.Info("===============identity=============",identity)// &{0xc0022ba960 0xc00367a4e0}
				cauthdslLogger.Info("==============1 =err=============",err)// nil
				if err != nil {
					cauthdslLogger.Errorf("Principal deserialization failure (%s) for identity %d", err, i)
					continue
				}
				cauthdslLogger.Info("===============identity=============",identity)// &{0xc0022ba960 0xc00367a4e0}
				cauthdslLogger.Info("============err = identity.SatisfiesPrincipal(signedByID)============")
				err = identity.SatisfiesPrincipal(signedByID)
				cauthdslLogger.Info("===========2 err============",err)
				//nil

				//the identity is a member of a different MSP (expected Org1MSP, got Org2MSP)

				if err != nil {
					cauthdslLogger.Infof("%p identity %d does not satisfy principal: %s", signedData, i, err)
					continue
				}
				cauthdslLogger.Infof("%p principal matched by identity %d", signedData, i)//0xc0039755a0 principal matched by identity 0
				cauthdslLogger.Info("========err = sd.Verify()=========")
				err = sd.Verify()
				cauthdslLogger.Info("========err =========",err)
				if err != nil {
					cauthdslLogger.Infof("%p signature for identity %d is invalid: %s", signedData, i, err)
					continue
				}
				cauthdslLogger.Infof("%p principal evaluation succeeds for identity %d", signedData, i)
				used[i] = true
				cauthdslLogger.Infof("======used[%v] = true=========",i)
				//======tused[0] = true=========
				return true
			}
			cauthdslLogger.Infof("%p principal evaluation fails", signedData)//0xc0039755a0
			return false
		}, nil
	default:
		return nil, fmt.Errorf("Unknown type: %T:%v", t, t)
	}
}
