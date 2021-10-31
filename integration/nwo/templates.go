/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package nwo

import "fmt"

// Templates can be used to provide custom templates to GenerateConfigTree.
type Templates struct {
	ConfigTx string `yaml:"configtx,omitempty"`
	Core     string `yaml:"core,omitempty"`
	Crypto   string `yaml:"crypto,omitempty"`
	Orderer  string `yaml:"orderer,omitempty"`
}

func (t *Templates) ConfigTxTemplate() string {
	fmt.Println("=Templates====ConfigTxTemplate====")
	if t.ConfigTx != "" {
		return t.ConfigTx
	}
	return DefaultConfigTxTemplate
}

func (t *Templates) CoreTemplate() string {
	fmt.Println("=Templates====CoreTemplate====")
	if t.Core != "" {
		return t.Core
	}
	return DefaultCoreTemplate
}

func (t *Templates) CryptoTemplate() string {
	fmt.Println("=Templates====CryptoTemplate====")
	if t.Crypto != "" {
		return t.Crypto
	}
	return DefaultCryptoTemplate
}

func (t *Templates) OrdererTemplate() string {
	fmt.Println("=Templates====OrdererTemplate====")
	if t.Orderer != "" {
		return t.Orderer
	}
	return DefaultOrdererTemplate
}
