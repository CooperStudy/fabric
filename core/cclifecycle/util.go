/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cc

import (
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/chaincode"
	"github.com/hyperledger/fabric/core/common/ccprovider"
	"github.com/hyperledger/fabric/core/common/privdata"
	"github.com/pkg/errors"
)

var (
	// AcceptAll returns a predicate that accepts all Metadata
	AcceptAll ChaincodePredicate = func(cc chaincode.Metadata) bool {
		return true
	}
)

// ChaincodePredicate accepts or rejects chaincode based on its metadata
type ChaincodePredicate func(cc chaincode.Metadata) bool

// DeployedChaincodes retrieves the metadata of the given deployed chaincodes
func DeployedChaincodes(q Query, filter ChaincodePredicate, loadCollections bool, chaincodes ...string) (chaincode.MetadataSet, error) {
	logger.Info("===DeployedChaincodes==")
	defer q.Done()

	var res chaincode.MetadataSet
	for _, cc := range chaincodes {
		data, err := q.GetState("lscc", cc)
		if err != nil {
			logger.Error("Failed querying lscc namespace:", err)
			return nil, errors.WithStack(err)
		}
		if len(data) == 0 {
			logger.Info("Chaincode", cc, "isn't instantiated")
			continue
		}
		ccInfo, err := extractCCInfo(data)
		if err != nil {
			logger.Error("Failed extracting chaincode info about", cc, "from LSCC returned payload. Error:", err)
			continue
		}
		if ccInfo.Name != cc {
			logger.Error("Chaincode", cc, "is listed in LSCC as", ccInfo.Name)
			continue
		}

		instCC := chaincode.Metadata{
			Name:    ccInfo.Name,
			Version: ccInfo.Version,
			Id:      ccInfo.Id,
			Policy:  ccInfo.Policy,
		}

		if !filter(instCC) {
			logger.Debug("Filtered out", instCC)
			continue
		}

		if loadCollections {
			key := privdata.BuildCollectionKVSKey(cc)
			collectionData, err := q.GetState("lscc", key)
			if err != nil {
				logger.Errorf("Failed querying lscc namespace for %s: %v", key, err)
				return nil, errors.WithStack(err)
			}
			instCC.CollectionsConfig = collectionData
			logger.Debug("Retrieved collection config for", cc, "from", key)
		}

		res = append(res, instCC)
	}
	logger.Debug("Returning", res)
	return res, nil
}

func deployedCCToNameVersion(cc chaincode.Metadata) nameVersion {
	logger.Info("===deployedCCToNameVersion==")
	return nameVersion{
		name:    cc.Name,
		version: cc.Version,
	}
}

func extractCCInfo(data []byte) (*ccprovider.ChaincodeData, error) {
	logger.Info("===extractCCInfo==")
	cd := &ccprovider.ChaincodeData{}

	if err := proto.Unmarshal(data, cd); err != nil {
		return nil, errors.Wrap(err, "failed unmarshaling lscc read value into ChaincodeData")
	}
	return cd, nil
}

type nameVersion struct {
	name    string
	version string
}

func installedCCToNameVersion(cc chaincode.InstalledChaincode) nameVersion {
	logger.Info("===installedCCToNameVersion==")
	return nameVersion{
		name:    cc.Name,
		version: cc.Version,
	}
}

func names(installedChaincodes []chaincode.InstalledChaincode) []string {
	logger.Info("===names==")
	var ccs []string
	for _, cc := range installedChaincodes {
		ccs = append(ccs, cc.Name)
	}
	return ccs
}
