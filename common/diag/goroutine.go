/*
Copyright IBM Corp All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package diag

import (
	"bytes"
	"fmt"
	"runtime/pprof"
)

type Logger interface {
	Infof(template string, args ...interface{})
	Errorf(template string, args ...interface{})
}

func CaptureGoRoutines() (string, error) {
	logger.Info("======CaptureGoRoutines===")
	var buf bytes.Buffer
	err := pprof.Lookup("goroutine").WriteTo(&buf, 2)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func LogGoRoutines(logger Logger) {
	logger.Info("======SLogGoRoutines===")
	output, err := CaptureGoRoutines()
	if err != nil {
		logger.Errorf("failed to capture go routines: %s", err)
		return
	}

	logger.Infof("Go routines report:\n%s", output)
}
