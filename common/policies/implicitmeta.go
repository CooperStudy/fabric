/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package policies

import (
	"bytes"
	"fmt"

	"github.com/golang/protobuf/proto"
	cb "github.com/hyperledger/fabric/protos/common"
	"go.uber.org/zap/zapcore"
)

type implicitMetaPolicy struct {
	threshold   int
	subPolicies []Policy

	// Only used for logging
	managers      map[string]*ManagerImpl
	subPolicyName string
}

// NewPolicy creates a new policy based on the policy bytes
func newImplicitMetaPolicy(data []byte, managers map[string]*ManagerImpl) (*implicitMetaPolicy, error) {
	//logger.Info("===newImplicitMetaPolicy===")
	definition := &cb.ImplicitMetaPolicy{}
	if err := proto.Unmarshal(data, definition); err != nil {
		return nil, fmt.Errorf("Error unmarshaling to ImplicitMetaPolicy: %s", err)
	}

	subPolicies := make([]Policy, len(managers))

	i := 0
	for _, manager := range managers {
		subPolicies[i], _ = manager.GetPolicy(definition.SubPolicy)
		i++
	}

	var threshold int

	switch definition.Rule {
	case cb.ImplicitMetaPolicy_ANY:
		//logger.Info("===============cb.ImplicitMetaPolicy_ANY:threshold = 1===========================")
		threshold = 1
	case cb.ImplicitMetaPolicy_ALL:
		threshold = len(subPolicies)
		//logger.Info("===============cb.ImplicitMetaPolicy_ALL:threshold = ===========================",threshold)
	case cb.ImplicitMetaPolicy_MAJORITY:
		threshold = len(subPolicies)/2 + 1
		//logger.Info("============len(subPolicies)==========",len(subPolicies))//1
		//logger.Info("===============cb.ImplicitMetaPolicy_MAJORITY:threshold = ===========================",threshold)
	}

	// In the special case that there are no policies, consider 0 to be a majority or any
	if len(subPolicies) == 0 {
		threshold = 0
	}

	return &implicitMetaPolicy{
		subPolicies:   subPolicies,
		threshold:     threshold,
		managers:      managers,
		subPolicyName: definition.SubPolicy,
	}, nil
}

// Evaluate takes a set of SignedData and evaluates whether this set of signatures satisfies the policy
func (imp *implicitMetaPolicy) Evaluate(signatureSet []*cb.SignedData) error {
	//logger.Info("===implicitMetaPolicy==Evaluate===")
	//logger.Info("This is an implicit meta policy, it will trigger other policy evaluations, whose failures may be benign")
	remaining := imp.threshold

	//logger.Info("=============remaining==========",remaining)
	defer func() {
		if remaining != 0 {
			// This log message may be large and expensive to construct, so worth checking the log level
			if logger.IsEnabledFor(zapcore.DebugLevel) {
				var b bytes.Buffer
				//logger.Info("==================",fmt.Sprintf("Evaluation Failed: Only %d policies were satisfied, but needed %d of [ ", imp.threshold-remaining, imp.threshold))
				b.WriteString(fmt.Sprintf("Evaluation Failed: Only %d policies were satisfied, but needed %d of [ ", imp.threshold-remaining, imp.threshold))
				for m := range imp.managers {
					//logger.Info("=======m=====",m)
					b.WriteString(m)
					b.WriteString(".")
					//logger.Info("=======imp.subPolicyName=====",imp.subPolicyName)
					b.WriteString(imp.subPolicyName)
					b.WriteString(" ")
				}
				b.WriteString("]")
				logger.Debugf(b.String())
				////logger.Info("=================b.String()=======================",b.String())
			}
		}
	}()

	for _, policy := range imp.subPolicies {
		//logger.Info("========policy==============",policy)
		if policy.Evaluate(signatureSet) == nil {
			//logger.Info("==========policy.Evaluate(signatureSet) == nil=======")
			remaining--
			//logger.Info("===========remaining===========",remaining)//0
			if remaining == 0 {
				//============ remaining == 0 （1）权限检测ok=====
				//logger.Info("============ remaining == 0 （1）权限检测ok==============")
				return nil
			}
		}
	}
	if remaining == 0 {
		//logger.Info("============ remaining == 0 （2）权限检测ok==============")
		return nil
	}
	return fmt.Errorf("Failed to reach implicit threshold of %d sub-policies, required %d remaining", imp.threshold, remaining)
}
