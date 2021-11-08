/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package token

import (
	"fmt"
	"github.com/hyperledger/fabric/core/handlers/validation/api"
	"github.com/hyperledger/fabric/protos/common"
)

type ValidationFactory struct {
}

func (*ValidationFactory) New() validation.Plugin {
	fmt.Println("=========ValidationFactory====New=================")
	return &ValidationPlugin{}
}

type ValidationPlugin struct {
}

func (v *ValidationPlugin) Init(dependencies ...validation.Dependency) error {
	fmt.Println("=========ValidationPlugin====Init=================")
	return nil
}

func (v *ValidationPlugin) Validate(block *common.Block, namespace string, txPosition int, actionPosition int, contextData ...validation.ContextDatum) error {
	fmt.Println("=========ValidationPlugin====Validate=================")
	return nil
}
