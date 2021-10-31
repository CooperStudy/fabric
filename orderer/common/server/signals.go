// +build !windows

/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package server

import (
	"fmt"
	"os"
	"syscall"

	"github.com/hyperledger/fabric/common/diag"
)

func addPlatformSignals(sigs map[os.Signal]func()) map[os.Signal]func() {
	fmt.Println("==addPlatformSignals==")
	sigs[syscall.SIGUSR1] = func() { diag.LogGoRoutines(logger.Named("diag")) }
	return sigs
}
