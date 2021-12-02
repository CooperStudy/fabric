/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package inquire

import (
	"bytes"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/policies"
	"github.com/hyperledger/fabric/protos/msp"
)

// ComparablePrincipal defines an MSPPrincipal that can be compared to other principals
type ComparablePrincipal struct {
	principal *msp.MSPPrincipal
	ou        *msp.OrganizationUnit
	role      *msp.MSPRole
	mspID     string
}

// NewComparablePrincipal creates a ComparablePrincipal out of the given MSPPrincipal.
// Returns nil if a failure occurs.
func NewComparablePrincipal(principal *msp.MSPPrincipal) *ComparablePrincipal {
	logger.Info("=========NewComparablePrincipal==========")
	if principal == nil {
		logger.Warning("Principal is nil")
		return nil
	}
	cp := &ComparablePrincipal{
		principal: principal,
	}
	logger.Info("===========principal.PrincipalClassification===============",principal.PrincipalClassification)
	//ROLE
	switch principal.PrincipalClassification {
	case msp.MSPPrincipal_ROLE:
		logger.Info("======msp.MSPPrincipal_ROLE==============")
		logger.Info("=====cp.ToRole=========",cp.ToRole())// &{0xc0000c4dc0 <nil> 0xc0000c5500 A}   &{0xc0000c4e40 <nil> 0xc0000c5780 B} &{0xc0000c4fc0 <nil> 0xc0000c5940 C} &{0xc0000c5400 <nil> 0xc0000c5a40 A} &{0xc0000c5480 <nil> 0xc0000c5b40 D}
		return cp.ToRole()
	case msp.MSPPrincipal_ORGANIZATION_UNIT:
		logger.Info("======msp.MSPPrincipal_ORGANIZATION_UNIT==============")
		logger.Info("=====cp.ToOURole========",cp.ToOURole())
		return cp.ToOURole()
	}
	logger.Info("===========int32(principal.PrincipalClassification)============",int32(principal.PrincipalClassification))

	mapping := msp.MSPPrincipal_Classification_name[int32(principal.PrincipalClassification)]
	logger.Info("===========mapping============",mapping)
	logger.Warning("Received an unsupported principal type:", principal.PrincipalClassification, "mapped to", mapping)
	return nil
}

// IsFound returns whether the ComparablePrincipal is found among the given set of ComparablePrincipals
// For the ComparablePrincipal x to be found, there needs to be some ComparablePrincipal y in the set
// such that x.IsA(y) will be true.
func (cp *ComparablePrincipal) IsFound(set ...*ComparablePrincipal) bool {
	logger.Info("=========ComparablePrincipal====IsFound======")
	for _, cp2 := range set {
		if cp.IsA(cp2) {
			return true
		}
	}
	return false
}

// IsA determines whether all identities that satisfy this ComparablePrincipal
// also satisfy the other ComparablePrincipal.
// Example: if this ComparablePrincipal is a Peer role,
// and the other ComparablePrincipal is a Member role, then
// all identities that satisfy this ComparablePrincipal (are peers)
// also satisfy the other principal (are members).
func (cp *ComparablePrincipal) IsA(other *ComparablePrincipal) bool {
	logger.Info("=========ComparablePrincipal====IsA======")
	this := cp

	if other == nil {
		return false
	}
	if this.principal == nil || other.principal == nil {
		logger.Warning("Used an un-initialized ComparablePrincipal")
		return false
	}
	// Compare the MSP ID
	if this.mspID != other.mspID {
		return false
	}

	// If the other Principal is a member, then any role or OU role
	// fits, because every role or OU role is also a member of the MSP
	if other.role != nil && other.role.Role == msp.MSPRole_MEMBER {
		return true
	}

	// Check if we're both OU roles
	if this.ou != nil && other.ou != nil {
		sameOU := this.ou.OrganizationalUnitIdentifier == other.ou.OrganizationalUnitIdentifier
		sameIssuer := bytes.Equal(this.ou.CertifiersIdentifier, other.ou.CertifiersIdentifier)
		return sameOU && sameIssuer
	}

	// Check if we're both the same MSP Role
	if this.role != nil && other.role != nil {
		return this.role.Role == other.role.Role
	}

	// Else, we can't say anything, because we have no knowledge
	// about the OUs that make up the MSP roles - so return false
	return false
}

// ToOURole converts this ComparablePrincipal to OU principal, and returns nil on failure
func (cp *ComparablePrincipal) ToOURole() *ComparablePrincipal {
	logger.Info("=========ComparablePrincipal====ToOURole======")
	ouRole := &msp.OrganizationUnit{}
	err := proto.Unmarshal(cp.principal.Principal, ouRole)
	if err != nil {
		logger.Warning("Failed unmarshaling principal:", err)
		return nil
	}
	cp.mspID = ouRole.MspIdentifier
	cp.ou = ouRole
	return cp
}

// ToRole converts this ComparablePrincipal to MSP Role, and returns nil if the conversion failed
func (cp *ComparablePrincipal) ToRole() *ComparablePrincipal {
	logger.Info("=========ComparablePrincipal====ToRole======")
	mspRole := &msp.MSPRole{}
	err := proto.Unmarshal(cp.principal.Principal, mspRole)
	if err != nil {
		logger.Warning("Failed unmarshaling principal:", err)
		return nil
	}
	cp.mspID = mspRole.MspIdentifier
	cp.role = mspRole
	logger.Info("=====cp.mspID====",cp.mspID)
	//=====cp.mspID==== Org1MSP
	//msp_identifier:"Org1MSP" role:PEER

	//=====cp.mspID==== A =====cp.mspID==== B
	logger.Info("=====cp.role====",cp.role)//=====cp.role==== msp_identifier:"A" =====cp.role==== msp_identifier:"B"
	return cp
}

// ComparablePrincipalSet aggregates ComparablePrincipals
type ComparablePrincipalSet []*ComparablePrincipal

// ToPrincipalSet converts this ComparablePrincipalSet to a PrincipalSet
func (cps ComparablePrincipalSet) ToPrincipalSet() policies.PrincipalSet {
	logger.Info("=========ComparablePrincipalSet====ToPrincipalSet======")
	var res policies.PrincipalSet
	for _, cp := range cps {
		logger.Info("=====cp===",cp)
		res = append(res, cp.principal)
	}
	logger.Info("===========res",res)
	//[principal:"\n\001A"  principal:"\n\001B" ]

	//[principal:"\n\007Org1MSP\020\003" ]
	//
	return res
}

// String returns a string representation of this ComparablePrincipalSet
func (cps ComparablePrincipalSet) String() string {
	//logger.Info("=========ComparablePrincipalSet====String======")
	buff := bytes.Buffer{}
	buff.WriteString("[")
	//logger.Info("============cps=============",cps)
	for i, cp := range cps {
		logger.Info("============i,",i,"========cp",cp)
		//============i, 0 ========cp &{0xc000215080 <nil> 0xc000215300 A}
		//============i, 1 ========cp &{0xc000215100 <nil> 0xc000215380 B}
		logger.Info("======cp.mspID=======",cp.mspID) //A
		buff.WriteString(cp.mspID)
		buff.WriteString(".")
		if cp.role != nil {
			logger.Info("=============cp.role != nil=================",fmt.Sprintf("%v", cp.role.Role))
			buff.WriteString(fmt.Sprintf("%v", cp.role.Role))
		}
		if cp.ou != nil {
			logger.Info("=============cp.ou != nil=================")
			logger.Info("=============cp.role != nil=================",fmt.Sprintf("%v", cp.ou.OrganizationalUnitIdentifier))
			buff.WriteString(fmt.Sprintf("%v", cp.ou.OrganizationalUnitIdentifier))
		}
		logger.Info("================i",i)
		logger.Info("========len(cps)-1========i",len(cps)-1)
		if i < len(cps)-1 {
			buff.WriteString(", ")
		}
	}
	buff.WriteString("]")
	logger.Info("====================buff==============",buff.String())//[A.MEMBER, B.MEMBER] [C.MEMBER]  [A.MEMBER, D.MEMBER]
	return buff.String()
}

// NewComparablePrincipalSet constructs a ComparablePrincipalSet out of the given PrincipalSet
func NewComparablePrincipalSet(set policies.PrincipalSet) ComparablePrincipalSet {
	logger.Info("========NewComparablePrincipalSet======")
	var res ComparablePrincipalSet
	for _, principal := range set {
		logger.Info("=======principal======",principal)
		//principal:"\n\001A" principal:"\n\001B"
		//=======principal====== principal:"\n\001C"
		//principal:"\n\007Org1MSP\020\003"
		cp := NewComparablePrincipal(principal)
		if cp == nil {
			return nil
		}
		res = append(res, cp)
	}
	return res
}

// Clone returns a copy of this ComparablePrincipalSet
func (cps ComparablePrincipalSet) Clone() ComparablePrincipalSet {
	logger.Info("======ComparablePrincipalSet==Clone======")
	res := make(ComparablePrincipalSet, len(cps))
	for i, cp := range cps {
		res[i] = cp
	}
	return res
}
