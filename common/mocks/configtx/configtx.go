/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package configtx

import (
	"fmt"
	cb "github.com/hyperledger/fabric/protos/common"
)

// Validator is a mock implementation of configtx.Validator
type Validator struct {
	// ChainIDVal is returned as the result of ChainID()
	ChainIDVal string

	// SequenceVal is returned as the result of Sequence()
	SequenceVal uint64

	// ApplyVal is returned by Apply
	ApplyVal error

	// AppliedConfigUpdateEnvelope is set by Apply
	AppliedConfigUpdateEnvelope *cb.ConfigEnvelope

	// ValidateVal is returned by Validate
	ValidateVal error

	// ProposeConfigUpdateError is returned as the error value for ProposeConfigUpdate
	ProposeConfigUpdateError error

	// ProposeConfigUpdateVal is returns as the value for ProposeConfigUpdate
	ProposeConfigUpdateVal *cb.ConfigEnvelope

	// ConfigProtoVal is returned as the value for ConfigProtoVal()
	ConfigProtoVal *cb.Config
}

// ConfigProto returns the ConfigProtoVal
func (cm *Validator) ConfigProto() *cb.Config {
	fmt.Println("==Validator===ConfigProto:start=====")
	defer func() {
		fmt.Println("==Validator===ConfigProto:end=====")
	}()
	return cm.ConfigProtoVal
}

// ConsensusType returns the ConsensusTypeVal
func (cm *Validator) ChainID() string {
	fmt.Println("==Validator===ChainID:start=====")
	defer func() {
		fmt.Println("==Validator===ChainID:end=====")
	}()
	return cm.ChainIDVal
}

// BatchSize returns the BatchSizeVal
func (cm *Validator) Sequence() uint64 {
	fmt.Println("==Validator===Sequence:start=====")
	defer func() {
		fmt.Println("==Validator===Sequence:end=====")
	}()
	return cm.SequenceVal
}

// ProposeConfigUpdate
func (cm *Validator) ProposeConfigUpdate(update *cb.Envelope) (*cb.ConfigEnvelope, error) {
	fmt.Println("==Validator===ProposeConfigUpdate:start=====")
	defer func() {
		fmt.Println("==Validator===ProposeConfigUpdate:end=====")
	}()
	return cm.ProposeConfigUpdateVal, cm.ProposeConfigUpdateError
}

// Apply returns ApplyVal
func (cm *Validator) Apply(configEnv *cb.ConfigEnvelope) error {
	fmt.Println("==Validator===Apply:start=====")
	defer func() {
		fmt.Println("==Validator===Apply:end=====")
	}()
	cm.AppliedConfigUpdateEnvelope = configEnv
	return cm.ApplyVal
}

// Validate returns ValidateVal
func (cm *Validator) Validate(configEnv *cb.ConfigEnvelope) error {
	fmt.Println("==Validator===Validate:start=====")
	defer func() {
		fmt.Println("==Validator===Validate:end=====")
	}()
	return cm.ValidateVal
}
