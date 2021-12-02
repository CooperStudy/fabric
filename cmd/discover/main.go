/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"github.com/hyperledger/fabric/common/flogging"
	"os"

	"github.com/hyperledger/fabric/bccsp/factory"
	"github.com/hyperledger/fabric/cmd/common"
	"github.com/hyperledger/fabric/discovery/cmd"
)
var logger = flogging.MustGetLogger("cmd.discover")
func main() {
	logger.Info("========main===========")
	factory.InitFactories(nil)
	cli := common.NewCLI("discover", "Command line client for fabric discovery service")
	discovery.AddCommands(cli)
	cli.Run(os.Args[1:])
}
