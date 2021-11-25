/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"fmt"
	"os"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/integration/chaincode/keylevelep"
)

func main() {
	fmt.Println("=========fabric/examples/chaincode/keylevelep/cmd/main.go func main()=========================")

	err := shim.Start(&keylevelep.EndorsementCC{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Exiting SBE chaincode: %s", err)
		os.Exit(2)
	}
}
