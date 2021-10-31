// +build windows

/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package server

import (
	"os"
)

func addPlatformSignals(sigs map[os.Signal]func()) map[os.Signal]func() {
	fmt.Println("==addPlatformSignals==")
	return sigs
}
