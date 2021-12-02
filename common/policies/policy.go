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
	//logger.Info("===PrincipalSets==ContainingOnly===")
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
	//logger.Info("===PrincipalSet==ContainingOnly===")
	for _, principal := range ps {
		if !f(principal) {
			return false
		}
	}
	return true
}

// UniqueSet returns a histogram that is induced by the PrincipalSet
/*
UniqueSet 返回由 PrincipalSet 诱导的直方图
 */
func (ps PrincipalSet) UniqueSet() map[*msp.MSPPrincipal]int {
	//logger.Info("===PrincipalSet==UniqueSet===")
	// Create a histogram that holds the MSPPrincipals and counts them
	// 创建一个包含 MSPPrincipals 并对其进行计数的直方图
	histogram := make(map[struct {
		cls       int32
		principal string
	}]int)
	// Now, populate the histogram
	for _, principal := range ps {
		//logger.Info("==principal=====",principal) //principal:"\n\001A"  principal:"\n\001B"
		//logger.Info("=============cls: int32(principal.PrincipalClassification),=================",int32(principal.PrincipalClassification))// 0
		//logger.Info("=============principal: string(principal.Principal),=================",string(principal.Principal))//A B
		key := struct {
			cls       int32
			principal string
		}{
			cls:       int32(principal.PrincipalClassification),
			principal: string(principal.Principal),
		}
		histogram[key]++
		//logger.Info("=======histogram[key]=============",histogram)
		/*
		map[{0 A}:1] map[{0 B}:1] map[{0 C}:1]
		 */
	}
	// Finally, convert to a histogram of MSPPrincipal pointers
	res := make(map[*msp.MSPPrincipal]int)
	for principal, count := range histogram {
		//logger.Info("====principal======",principal) //{0 A}
		//logger.Info("=========count============",count)//1
		a := msp.MSPPrincipal{
			PrincipalClassification: msp.MSPPrincipal_Classification(principal.cls),
			Principal:               []byte(principal.principal),
		}
		logger.Info("====res =======msp.MSPPrincipal=============",a,"==========count=============",count)
		/*
		{ROLE [10 1 65] {} [] 0}
		count 1
		{ROLE [10 1 66] {} [] 0}
		1
		 */
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
	//logger.Info("===NewManagerImpl=====")
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
	//logger.Info("===rejectPolicy==Evaluate===")
	return fmt.Errorf("No such policy: '%s'", rp)
}

// Manager returns the sub-policy manager for a given path and whether it exists
func (pm *ManagerImpl) Manager(path []string) (Manager, bool) {
	//logger.Info("===ManagerImpl==Manager===")
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
	//logger.Info("===policyLogger==Evaluate===")
	if logger.IsEnabledFor(zapcore.DebugLevel) {
		//logger.Infof("== Evaluating %T Policy %s ==", pl.policy, pl.policyName)// /Channel/Application/Readers
		defer logger.Infof("== Done Evaluating %T Policy %s", pl.policy, pl.policyName)
	}

	err := pl.policy.Evaluate(signatureSet)
	if err != nil {
		logger.Infof("Signature set did not satisfy policy %s", pl.policyName)
	} else {
		//Signature set satisfies policy %s /Channel/Application/Org1MSP/Readers
		//Signature set satisfies policy %s /Channel/Application/Readers
		logger.Infof("Signature set satisfies policy %s", pl.policyName)
	}
	return err
}

// GetPolicy returns a policy and true if it was the policy requested, or false if it is the default reject policy
func (pm *ManagerImpl) GetPolicy(id string) (Policy, bool) {
	//logger.Info("===ManagerImpl==GetPolicy===")
	//logger.Info("=======id======",id)
	//ChannelCreationPolicy

	//Writers
	//Readers
	//Admins
	//
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

	//logger.Info("=========relpath=========",relpath)
	//ChannelCreationPolicy

	//Readers
	//Writers
	//Admins
	policy, ok := pm.policies[relpath]
	// /Channel/Application/Org1MSP/Readers
	//
	if !ok {
		logger.Debugf("Returning dummy reject all policy because %s could not be found in %s/%s", id, pm.path, relpath)
		return rejectPolicy(relpath), false
	}

	//logger.Info("==========policyName================",PathSeparator + pm.path + PathSeparator + relpath)
    // /Channel/Application/Org1MSP/Readers
    // /Channel/Application/Org1MSP/Writers
    // /Channel/Orderer/OrdererOrg/Admins

    // /Channel/Orderer/OrdererOrg/Writers
    // /Channel/Orderer/OrdererOrg/Readers
    // /Channel/Orderer/OrdererOrg/Admins

    //  /Channel/Application/Admins
    //  /Channel/Orderer/Admins
    //  /Channel/Application/Readers
    //  /Channel/Orderer/Readers
    //  /Channel/Application/Writers
    //  /Channel/Orderer/Writers

    ///Channel/Application/ChannelCreationPolicy
	return &policyLogger{
		policy:     policy,
		policyName: PathSeparator + pm.path + PathSeparator + relpath,
	}, true
}
