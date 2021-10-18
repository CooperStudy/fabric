/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package metadata

import (
	"fmt"
	"github.com/hyperledger/fabric/common/flogging"
	"runtime"

	common "github.com/hyperledger/fabric/common/metadata"
)

// package-scoped variables

// Package version
var Version string

// package-scoped constants

// Program name
const ProgramName = "orderer"
var logger = flogging.MustGetLogger("orderer.common.metadata.metadata.go")
func GetVersionInfo() string {
	logger.Info("====GetVersionInfo===")
	Version = common.Version
	if Version == "" {
		Version = "development build"
	}

	return fmt.Sprintf(
		"%s:\n Version: %s\n Commit SHA: %s\n Go version: %s\n OS/Arch: %s\n",
		ProgramName,
		Version,
		common.CommitSHA,
		runtime.Version(),
		fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	)
}
