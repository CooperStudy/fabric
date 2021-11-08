/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package ccprovider

import (
	"fmt"
	"path/filepath"

	"github.com/hyperledger/fabric/core/config"
)

// GetChaincodeInstallPathFromViper returns the path where chaincodes are installed
func GetChaincodeInstallPathFromViper() string {
	fmt.Println("==GetChaincodeInstallPathFromViper==")
	return filepath.Join(config.GetPath("peer.fileSystemPath"), "chaincodes")
}

// LoadPackage loads a chaincode package from the file system
func LoadPackage(ccname string, ccversion string, path string) (CCPackage, error) {
	fmt.Println("==LoadPackage==")
	return (&CCInfoFSImpl{}).GetChaincodeFromPath(ccname, ccversion, path)
}
