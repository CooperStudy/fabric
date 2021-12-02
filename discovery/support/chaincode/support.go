/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package chaincode

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/chaincode"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/common/policies"
	"github.com/hyperledger/fabric/common/policies/inquire"
	common2 "github.com/hyperledger/fabric/protos/common"
)

var logger = flogging.MustGetLogger("discovery.DiscoverySupport")

type MetadataRetriever interface {
	Metadata(channel string, cc string, loadCollections bool) *chaincode.Metadata
}

// DiscoverySupport implements support that is used for service discovery
// that is related to chaincode
type DiscoverySupport struct {
	ci MetadataRetriever
}

// NewDiscoverySupport creates a new DiscoverySupport
func NewDiscoverySupport(ci MetadataRetriever) *DiscoverySupport {
	logger.Info("=======NewDiscoverySupport===")
	s := &DiscoverySupport{
		ci: ci,
	}
	return s
}

func (s *DiscoverySupport) PolicyByChaincode(channel string, cc string) policies.InquireablePolicy {
	logger.Info("=======DiscoverySupport==PolicyByChaincode=")
	logger.Info("============channel=======",channel)
	logger.Info("============cc=======",cc)
	chaincodeData := s.ci.Metadata(channel, cc, false)
	logger.Info("==========chaincodeData==================",chaincodeData)
	if chaincodeData == nil {
		logger.Info("Chaincode", cc, "wasn't found")
		return nil
	}
	pol := &common2.SignaturePolicyEnvelope{}
	if err := proto.Unmarshal(chaincodeData.Policy, pol); err != nil {
		logger.Warning("Failed unmarshaling policy for chaincode", cc, ":", err)
		return nil
	}
	logger.Info("=======len(pol.Identities)===========",len(pol.Identities))
	logger.Info("=======pol.Rule===========",pol.Rule)
	if len(pol.Identities) == 0 || pol.Rule == nil {
		logger.Warningf("Invalid policy, either Identities(%v) or Rule(%v) are empty:", pol.Identities, pol.Rule)
		return nil
	}
	return inquire.NewInquireableSignaturePolicy(pol)
}
