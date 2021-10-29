/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/
package bridge

import (
	"fmt"
	"github.com/hyperledger/fabric-amcl/amcl"
	cryptolib "github.com/hyperledger/fabric/idemix"
)

// NewRandOrPanic return a new amcl PRG or panic
func NewRandOrPanic() *amcl.RAND {
	fmt.Println("====NewRandOrPanic=======")
	rng, err := cryptolib.GetRand()
	if err != nil {
		panic(err)
	}
	return rng
}
