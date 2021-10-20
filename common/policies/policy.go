/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package policies

import (
	"fmt"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/flogging"
	cb "github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/msp"
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
)

const (
	// Path separator is used to separate policy names in paths
	PathSeparator = "/"

	// ChannelPrefix is used in the path of standard channel policy managers
	ChannelPrefix = "Channel"

	// ApplicationPrefix is used in the path of standard application policy paths
	ApplicationPrefix = "Application"

	// OrdererPrefix is used in the path of standard orderer policy paths
	OrdererPrefix = "Orderer"

	// ChannelReaders is the label for the channel's readers policy (encompassing both orderer and application readers)
	ChannelReaders = PathSeparator + ChannelPrefix + PathSeparator + "Readers"

	// ChannelWriters is the label for the channel's writers policy (encompassing both orderer and application writers)
	ChannelWriters = PathSeparator + ChannelPrefix + PathSeparator + "Writers"

	// ChannelApplicationReaders is the label for the channel's application readers policy
	ChannelApplicationReaders = PathSeparator + ChannelPrefix + PathSeparator + ApplicationPrefix + PathSeparator + "Readers"

	// ChannelApplicationWriters is the label for the channel's application writers policy
	ChannelApplicationWriters = PathSeparator + ChannelPrefix + PathSeparator + ApplicationPrefix + PathSeparator + "Writers"

	// ChannelApplicationAdmins is the label for the channel's application admin policy
	ChannelApplicationAdmins = PathSeparator + ChannelPrefix + PathSeparator + ApplicationPrefix + PathSeparator + "Admins"

	// BlockValidation is the label for the policy which should validate the block signatures for the channel
	BlockValidation = PathSeparator + ChannelPrefix + PathSeparator + OrdererPrefix + PathSeparator + "BlockValidation"
)

var logger = flogging.MustGetLogger("policies")

// PrincipalSet is a collection of MSPPrincipals
type PrincipalSet []*msp.MSPPrincipal

// PrincipalSets aggregates PrincipalSets
type PrincipalSets []PrincipalSet

// ContainingOnly returns PrincipalSets that contain only principals of the given predicate
func (psSets PrincipalSets) ContainingOnly(f func(*msp.MSPPrincipal) bool) PrincipalSets {
	var res PrincipalSets
	for _, set := range psSets {
		if !set.ContainingOnly(f) {
			continue
		}
		res = append(res, set)
	}
	return res
}

// ContainingOnly returns whether the given PrincipalSet contains only Principals
// that satisfy the given predicate
func (ps PrincipalSet) ContainingOnly(f func(*msp.MSPPrincipal) bool) bool {
	for _, principal := range ps {
		if !f(principal) {
			return false
		}
	}
	return true
}

// UniqueSet returns a histogram that is induced by the PrincipalSet
func (ps PrincipalSet) UniqueSet() map[*msp.MSPPrincipal]int {
	// Create a histogram that holds the MSPPrincipals and counts them
	histogram := make(map[struct {
		cls       int32
		principal string
	}]int)
	// Now, populate the histogram
	for _, principal := range ps {
		key := struct {
			cls       int32
			principal string
		}{
			cls:       int32(principal.PrincipalClassification),
			principal: string(principal.Principal),
		}
		histogram[key]++
	}
	// Finally, convert to a histogram of MSPPrincipal pointers
	res := make(map[*msp.MSPPrincipal]int)
	for principal, count := range histogram {
		res[&msp.MSPPrincipal{
			PrincipalClassification: msp.MSPPrincipal_Classification(principal.cls),
			Principal:               []byte(principal.principal),
		}] = count
	}
	return res
}

// Policy is used to determine if a signature is valid
type Policy interface {
	// Evaluate takes a set of SignedData and evaluates whether this set of signatures satisfies the policy
	Evaluate(signatureSet []*cb.SignedData) error
}

// InquireablePolicy is a Policy that one can inquire
type InquireablePolicy interface {
	// SatisfiedBy returns a slice of PrincipalSets that each of them
	// satisfies the policy.
	SatisfiedBy() []PrincipalSet
}

// Manager is a read only subset of the policy ManagerImpl
type Manager interface {
	// GetPolicy returns a policy and true if it was the policy requested, or false if it is the default policy
	GetPolicy(id string) (Policy, bool)

	// Manager returns the sub-policy manager for a given path and whether it exists
	Manager(path []string) (Manager, bool)
}

// Provider provides the backing implementation of a policy
type Provider interface {
	// NewPolicy creates a new policy based on the policy bytes
	NewPolicy(data []byte) (Policy, proto.Message, error)
}

// ChannelPolicyManagerGetter is a support interface
// to get access to the policy manager of a given channel
type ChannelPolicyManagerGetter interface {
	// Returns the policy manager associated to the passed channel
	// and true if it was the manager requested, or false if it is the default manager
	Manager(channelID string) (Manager, bool)
}

// ManagerImpl is an implementation of Manager and configtx.ConfigHandler
// In general, it should only be referenced as an Impl for the configtx.ConfigManager
type ManagerImpl struct {
	path     string // The group level path
	policies map[string]Policy
	managers map[string]*ManagerImpl
}

// NewManagerImpl creates a new ManagerImpl with the given CryptoHelper
func NewManagerImpl(path string, providers map[int32]Provider, root *cb.ConfigGroup) (*ManagerImpl, error) {
	logger.Info("=====NewManagerImpl:start=====")
	defer func() {
		logger.Info("=====NewManagerImpl:end=====")
	}()
	var err error
	_, ok := providers[int32(cb.Policy_IMPLICIT_META)]
	if ok {
		logger.Panicf("ImplicitMetaPolicy type must be provider by the policy manager")
	}

	managers := make(map[string]*ManagerImpl)

	for groupName, group := range root.Groups {
		managers[groupName], err = NewManagerImpl(path+PathSeparator+groupName, providers, group)
		if err != nil {
			return nil, err
		}
	}

	policies := make(map[string]Policy)
	for policyName, configPolicy := range root.Policies {
		policy := configPolicy.Policy
		if policy == nil {
			return nil, fmt.Errorf("policy %s at path %s was nil", policyName, path)
		}

		var cPolicy Policy

		if policy.Type == int32(cb.Policy_IMPLICIT_META) {
			imp, err := newImplicitMetaPolicy(policy.Value, managers)
			if err != nil {
				return nil, errors.Wrapf(err, "implicit policy %s at path %s did not compile", policyName, path)
			}
			cPolicy = imp
		} else {
			provider, ok := providers[int32(policy.Type)]
			if !ok {
				return nil, fmt.Errorf("policy %s at path %s has unknown policy type: %v", policyName, path, policy.Type)
			}

			var err error
			cPolicy, _, err = provider.NewPolicy(policy.Value)
			if err != nil {
				return nil, errors.Wrapf(err, "policy %s at path %s did not compile", policyName, path)
			}
		}

		policies[policyName] = cPolicy

		/*
		2021-10-19 08:33:50.825 UTC [policies] NewManagerImpl -> DEBU 144 Proposed new policy Admins for Channel/Orderer/OrdererOrg
		2021-10-19 08:33:50.825 UTC [policies] NewManagerImpl -> DEBU 145 Proposed new policy Readers for Channel/Orderer/OrdererOrg
		2021-10-19 08:33:50.825 UTC [policies] NewManagerImpl -> DEBU 146 Proposed new policy Writers for Channel/Orderer/OrdererOrg
		2021-10-19 08:33:50.825 UTC [policies] NewManagerImpl -> DEBU 147 Proposed new policy Writers for Channel/Orderer
		2021-10-19 08:33:50.825 UTC [policies] NewManagerImpl -> DEBU 148 Proposed new policy Admins for Channel/Orderer
		2021-10-19 08:33:50.825 UTC [policies] NewManagerImpl -> DEBU 149 Proposed new policy BlockValidation for Channel/Orderer
		2021-10-19 08:33:50.825 UTC [policies] NewManagerImpl -> DEBU 14a Proposed new policy Readers for Channel/Orderer
		2021-10-19 08:33:50.825 UTC [policies] NewManagerImpl -> DEBU 14b Proposed new policy Admins for Channel/Consortiums/SampleConsortium/Org1MS
		P
		2021-10-19 08:33:50.825 UTC [policies] NewManagerImpl -> DEBU 14c Proposed new policy Readers for Channel/Consortiums/SampleConsortium/Org1M
		SP
		2021-10-19 08:33:50.826 UTC [policies] NewManagerImpl -> DEBU 14d Proposed new policy Writers for Channel/Consortiums/SampleConsortium/Org1M
		SP
		2021-10-19 08:33:50.826 UTC [policies] NewManagerImpl -> DEBU 14e Proposed new policy Writers for Channel/Consortiums/SampleConsortium/Org2M
		SP
		2021-10-19 08:33:50.826 UTC [policies] NewManagerImpl -> DEBU 14f Proposed new policy Admins for Channel/Consortiums/SampleConsortium/Org2MS
		P
		2021-10-19 08:33:50.826 UTC [policies] NewManagerImpl -> DEBU 150 Proposed new policy Readers for Channel/Consortiums/SampleConsortium/Org2M
		SP
		2021-10-19 08:33:50.826 UTC [policies] NewManagerImpl -> DEBU 151 Proposed new policy Admins for Channel/Consortiums
		2021-10-19 08:33:50.826 UTC [policies] GetPolicy -> DEBU 152 Returning dummy reject all policy because Readers could not be found in Channel
		/Consortiums/Readers
		2021-10-19 08:33:50.826 UTC [policies] NewManagerImpl -> DEBU 153 Proposed new policy Readers for Channel
		2021-10-19 08:33:50.826 UTC [policies] GetPolicy -> DEBU 154 Returning dummy reject all policy because Writers could not be found in Channel
		/Consortiums/Writers
		2021-10-19 08:33:50.826 UTC [policies] NewManagerImpl -> DEBU 155 Proposed new policy Writers for Channel
		2021-10-19 08:33:50.826 UTC [policies] NewManagerImpl -> DEBU 156 Proposed new policy Admins for Channel
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 157 Adding to config map: [Group]  /Channel
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 158 Adding to config map: [Group]  /Channel/Orderer
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 159 Adding to config map: [Group]  /Channel/Orderer/OrdererOrg
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 15a Adding to config map: [Value]  /Channel/Orderer/OrdererOrg/MSP
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 15b Adding to config map: [Policy] /Channel/Orderer/OrdererOrg/Admins
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 15c Adding to config map: [Policy] /Channel/Orderer/OrdererOrg/Readers
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 15d Adding to config map: [Policy] /Channel/Orderer/OrdererOrg/Writers
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 15e Adding to config map: [Value]  /Channel/Orderer/BatchTimeout
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 15f Adding to config map: [Value]  /Channel/Orderer/ChannelRestrictions
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 160 Adding to config map: [Value]  /Channel/Orderer/Capabilities
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 161 Adding to config map: [Value]  /Channel/Orderer/ConsensusType
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 162 Adding to config map: [Value]  /Channel/Orderer/BatchSize
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 163 Adding to config map: [Policy] /Channel/Orderer/Writers
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 160 Adding to config map: [Value]  /Channel/Orderer/Capabilities
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 161 Adding to config map: [Value]  /Channel/Orderer/ConsensusType
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 162 Adding to config map: [Value]  /Channel/Orderer/BatchSize
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 163 Adding to config map: [Policy] /Channel/Orderer/Writers
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 164 Adding to config map: [Policy] /Channel/Orderer/Admins
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 165 Adding to config map: [Policy] /Channel/Orderer/BlockValidation
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 166 Adding to config map: [Policy] /Channel/Orderer/Readers
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 167 Adding to config map: [Group]  /Channel/Consortiums
		2021-10-19 08:33:50.826 UTC [common.configtx] addToMap -> DEBU 168 Adding to config map: [Group]  /Channel/Consortiums/SampleConsortium
		2021-10-19 08:33:50.827 UTC [common.configtx] addToMap -> DEBU 169 Adding to config map: [Group]  /Channel/Consortiums/SampleConsortium/Org1
		MSP
		2021-10-19 08:33:50.827 UTC [common.configtx] addToMap -> DEBU 16a Adding to config map: [Value]  /Channel/Consortiums/SampleConsortium/Org1
		MSP/MSP
		2021-10-19 08:33:50.827 UTC [common.configtx] addToMap -> DEBU 16b Adding to config map: [Policy] /Channel/Consortiums/SampleConsortium/Org1
		MSP/Admins
		2021-10-19 08:33:50.827 UTC [common.configtx] addToMap -> DEBU 16c Adding to config map: [Policy] /Channel/Consortiums/SampleConsortium/Org1
		MSP/Readers
		2021-10-19 08:33:50.827 UTC [common.configtx] addToMap -> DEBU 16d Adding to config map: [Policy] /Channel/Consortiums/SampleConsortium/Org1
		MSP/Writers
		2021-10-19 08:33:50.827 UTC [common.configtx] addToMap -> DEBU 16e Adding to config map: [Group]  /Channel/Consortiums/SampleConsortium/Org2
		MSP
		2021-10-19 08:33:50.827 UTC [common.configtx] addToMap -> DEBU 16f Adding to config map: [Value]  /Channel/Consortiums/SampleConsortium/Org2
		MSP/MSP
		2021-10-19 08:33:50.827 UTC [common.configtx] addToMap -> DEBU 170 Adding to config map: [Policy] /Channel/Consortiums/SampleConsortium/Org2
		MSP/Admins
		2021-10-19 08:33:50.827 UTC [common.configtx] addToMap -> DEBU 171 Adding to config map: [Policy] /Channel/Consortiums/SampleConsortium/Org2
		MSP/Readers
		2021-10-19 08:33:50.827 UTC [common.configtx] addToMap -> DEBU 172 Adding to config map: [Policy] /Channel/Consortiums/SampleConsortium/Org2
		MSP/Writers
		2021-10-19 08:33:50.827 UTC [common.configtx] addToMap -> DEBU 173 Adding to config map: [Value]  /Channel/Consortiums/SampleConsortium/Chan
		nelCreationPolicy
		2021-10-19 08:33:50.827 UTC [common.configtx] addToMap -> DEBU 174 Adding to config map: [Policy] /Channel/Consortiums/Admins
		2021-10-19 08:33:50.827 UTC [common.configtx] addToMap -> DEBU 175 Adding to config map: [Value]  /Channel/OrdererAddresses
		2021-10-19 08:33:50.827 UTC [common.configtx] addToMap -> DEBU 176 Adding to config map: [Value]  /Channel/Capabilities
		2021-10-19 08:33:50.827 UTC [common.configtx] addToMap -> DEBU 177 Adding to config map: [Value]  /Channel/HashingAlgorithm
		2021-10-19 08:33:50.827 UTC [common.configtx] addToMap -> DEBU 178 Adding to config map: [Value]  /Channel/BlockDataHashingStructure
		2021-10-19 08:33:50.827 UTC [common.configtx] addToMap -> DEBU 179 Adding to config map: [Policy] /Channel/Readers
		2021-10-19 08:33:50.827 UTC [common.configtx] addToMap -> DEBU 17a Adding to config map: [Policy] /Channel/Writers
		2021-10-19 08:33:50.827 UTC [common.configtx] addToMap -> DEBU 17b Adding to config map: [Policy] /Channel/Admins




		*/
		logger.Debugf("Proposed new policy %s for %s", policyName, path)
	}

	for groupName, manager := range managers {
		for policyName, policy := range manager.policies {
			policies[groupName+PathSeparator+policyName] = policy
		}
	}

	return &ManagerImpl{
		path:     path,
		policies: policies,
		managers: managers,
	}, nil
}

type rejectPolicy string

func (rp rejectPolicy) Evaluate(signedData []*cb.SignedData) error {
	return fmt.Errorf("No such policy: '%s'", rp)
}

// Manager returns the sub-policy manager for a given path and whether it exists
func (pm *ManagerImpl) Manager(path []string) (Manager, bool) {
	logger.Info("====Manager:start======")
	defer func() {
		logger.Info("====Manager:end======")
	}()
	/*
	2021-10-19 08:33:50.827 UTC [policies] Manager -> DEBU 180 Manager Channel looking up path [Application]
	2021-10-19 08:33:50.827 UTC [policies] Manager -> DEBU 181 Manager Channel has managers Orderer
	2021-10-19 08:33:50.827 UTC [policies] Manager -> DEBU 182 Manager Channel has managers Consortiums
	2021-10-19 08:33:50.827 UTC [policies] Manager -> DEBU 183 Manager Channel looking up path [Orderer]
	2021-10-19 08:33:50.827 UTC [policies] Manager -> DEBU 184 Manager Channel has managers Orderer
	2021-10-19 08:33:50.827 UTC [policies] Manager -> DEBU 185 Manager Channel has managers Consortiums
	2021-10-19 08:33:50.827 UTC [policies] Manager -> DEBU 186 Manager Channel/Orderer looking up path []
	2021-10-19 08:33:50.827 UTC [policies] Manager -> DEBU 187 Manager Channel/Orderer has managers OrdererOrg
	*/
	//Manager Channel looking up path [Application]
	logger.Debugf("Manager %s looking up path %v", pm.path, path)
	for manager := range pm.managers {
		logger.Debugf("Manager %s has managers %s", pm.path, manager)
	}
	if len(path) == 0 {
		return pm, true
	}

	m, ok := pm.managers[path[0]]
	if !ok {
		return nil, false
	}

	return m.Manager(path[1:])
}

type policyLogger struct {
	policy     Policy
	policyName string
}

func (pl *policyLogger) Evaluate(signatureSet []*cb.SignedData) error {
	if logger.IsEnabledFor(zapcore.DebugLevel) {
		logger.Debugf("== Evaluating %T Policy %s ==", pl.policy, pl.policyName)
		defer logger.Debugf("== Done Evaluating %T Policy %s", pl.policy, pl.policyName)
	}

	err := pl.policy.Evaluate(signatureSet)
	if err != nil {
		logger.Debugf("Signature set did not satisfy policy %s", pl.policyName)
	} else {
		logger.Debugf("Signature set satisfies policy %s", pl.policyName)
	}
	return err
}

// GetPolicy returns a policy and true if it was the policy requested, or false if it is the default reject policy
func (pm *ManagerImpl) GetPolicy(id string) (Policy, bool) {
	if id == "" {
		logger.Errorf("Returning dummy reject all policy because no policy ID supplied")
		return rejectPolicy(id), false
	}
	var relpath string

	if strings.HasPrefix(id, PathSeparator) {
		if !strings.HasPrefix(id, PathSeparator+pm.path) {
			logger.Debugf("Requested absolute policy %s from %s, returning rejectAll", id, pm.path)
			return rejectPolicy(id), false
		}
		// strip off the leading slash, the path, and the trailing slash
		relpath = id[1+len(pm.path)+1:]
	} else {
		relpath = id
	}

	policy, ok := pm.policies[relpath]
	if !ok {
		logger.Debugf("Returning dummy reject all policy because %s could not be found in %s/%s", id, pm.path, relpath)
		return rejectPolicy(relpath), false
	}

	return &policyLogger{
		policy:     policy,
		policyName: PathSeparator + pm.path + PathSeparator + relpath,
	}, true
}
