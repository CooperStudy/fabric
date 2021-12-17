/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package pluggable

import (
	"fmt"
	"github.com/hyperledger/fabric/common/flogging"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	EndorsementPluginEnvVar = "ENDORSEMENT_PLUGIN_ENV_VAR"
	ValidationPluginEnvVar  = "VALIDATION_PLUGIN_ENV_VAR"
)

// EndorsementPluginActivationFolder returns the name of the folder that if
// the file of the peer's id in it exists - it indicates that the endorsement plugin was activated
// for that peer
var logger = flogging.MustGetLogger("integration.pluggable")
func EndorsementPluginActivationFolder() string {
	logger.Info("=====EndorsementPluginActivationFolder==========")
	return os.Getenv(EndorsementPluginEnvVar)
}

// SetEndorsementPluginActivationFolder sets the name of the folder
// that if the file of the peer's id in it exists - it indicates that the endorsement plugin was activated
// for that peer
func SetEndorsementPluginActivationFolder(path string) {
	logger.Info("=====SetEndorsementPluginActivationFolder==========")
	os.Setenv(EndorsementPluginEnvVar, path)
}

// ValidationPluginActivationFilePath returns the name of the folder that if
// the file of the peer's id in it exists - it indicates that the validation plugin was activated
// for that peer
func ValidationPluginActivationFolder() string {
	logger.Info("=====ValidationPluginActivationFolder==========")
	return os.Getenv(ValidationPluginEnvVar)
}

// SetValidationPluginActivationFolder sets the name of the folder
// that if the file of the peer's id in it exists - it indicates that the validation plugin was activated
// for that peer
func SetValidationPluginActivationFolder(path string) {
	logger.Info("=====SetValidationPluginActivationFolder==========")
	os.Setenv(ValidationPluginEnvVar, path)
}

func markPluginActivation(dir string) {
	logger.Info("=====markPluginActivation==========")
	fileName := filepath.Join(dir, viper.GetString("peer.id"))
	_, err := os.Create(fileName)
	if err != nil {
		panic(fmt.Sprintf("failed to create file %s: %v", fileName, err))
	}
}

// PublishEndorsementPluginActivation makes it known that the endorsement plugin
// was activated for the peer that is invoking this function
func PublishEndorsementPluginActivation() {
	logger.Info("=====PublishEndorsementPluginActivation==========")
	markPluginActivation(EndorsementPluginActivationFolder())
}

// PublishValidationPluginActivation makes it known that the validation plugin
// was activated for the peer that is invoking this function
func PublishValidationPluginActivation() {
	logger.Info("=====PublishValidationPluginActivation==========")
	markPluginActivation(ValidationPluginActivationFolder())
}

// CountEndorsementPluginActivations returns the number of peers that activated
// the endorsement plugin
func CountEndorsementPluginActivations() int {
	logger.Info("=====CountEndorsementPluginActivations==========")
	return listDir(EndorsementPluginActivationFolder())
}

// CountValidationPluginActivations returns the number of peers that activated
// the validation plugin
func CountValidationPluginActivations() int {
	logger.Info("=====CountValidationPluginActivations==========")
	return listDir(ValidationPluginActivationFolder())
}

func listDir(d string) int {
	logger.Info("=====listDir==========")
	dir, err := ioutil.ReadDir(d)
	if err != nil {
		panic(fmt.Sprintf("failed listing directory %s: %v", d, err))
	}
	return len(dir)
}
