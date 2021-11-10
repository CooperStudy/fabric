package main

import (
	"fmt"
	"github.com/Knetic/govaluate"
	"github.com/hyperledger/fabric/protos/common"
	"regexp"
	"strings"
	"testing"
)

// Role values for principals
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
	RoleClient = "client"
	RolePeer   = "peer"
	// RoleOrderer = "orderer" TODO
)


const (
	GateAnd   = "And"
	GateOr    = "Or"
	GateOutOf = "OutOf"
)

var (
	regex = regexp.MustCompile(
		fmt.Sprintf("^([[:alnum:].-]+)([.])(%s|%s|%s|%s)$",
			RoleAdmin, RoleMember, RoleClient, RolePeer),
	)
	regexErr = regexp.MustCompile("^No parameter '([^']+)' found[.]$")
)

func Test1(t *testing.T) {

}

func FromString(policy string) (*common.SignaturePolicyEnvelope, error) {
	fmt.Println("==policy==",policy)//OR(AND('A.member', 'B.member'), 'C.member', AND('A.member', 'D.member'))
	fmt.Println("==========FromString=========")
	// first we translate the and/or business into outof gates
	intermediate, err := govaluate.NewEvaluableExpressionWithFunctions(
		policy, map[string]govaluate.ExpressionFunction{
			GateAnd:                    and,
			strings.ToLower(GateAnd):   and,
			strings.ToUpper(GateAnd):   and,
			GateOr:                     or,
			strings.ToLower(GateOr):    or,
			strings.ToUpper(GateOr):    or,
			GateOutOf:                  outof,
			strings.ToLower(GateOutOf): outof,
			strings.ToUpper(GateOutOf): outof,
		},
	)
	if err != nil {
		return nil, err
	}

	intermediateRes, err := intermediate.Evaluate(map[string]interface{}{})
	if err != nil {
		// attempt to produce a meaningful error
		if regexErr.MatchString(err.Error()) {
			sm := regexErr.FindStringSubmatch(err.Error())
			if len(sm) == 2 {
				return nil, fmt.Errorf("unrecognized token '%s' in policy string", sm[1])
			}
		}

		return nil, err
	}
	resStr, ok := intermediateRes.(string)
	if !ok {
		return nil, fmt.Errorf("invalid policy string '%s'", policy)
	}

	// we still need two passes. The first pass just adds an extra
	// argument ID to each of the outof calls. This is
	// required because govaluate has no means of giving context
	// to user-implemented functions other than via arguments.
	// We need this argument because we need a global place where
	// we put the identities that the policy requires
	exp, err := govaluate.NewEvaluableExpressionWithFunctions(resStr, map[string]govaluate.ExpressionFunction{"outof": firstPass})
	if err != nil {
		return nil, err
	}

	res, err := exp.Evaluate(map[string]interface{}{})
	if err != nil {
		// attempt to produce a meaningful error
		if regexErr.MatchString(err.Error()) {
			sm := regexErr.FindStringSubmatch(err.Error())
			if len(sm) == 2 {
				return nil, fmt.Errorf("unrecognized token '%s' in policy string", sm[1])
			}
		}

		return nil, err
	}
	resStr, ok = res.(string)
	if !ok {
		return nil, fmt.Errorf("invalid policy string '%s'", policy)
	}

	ctx := newContext()
	parameters := make(map[string]interface{}, 1)
	parameters["ID"] = ctx

	exp, err = govaluate.NewEvaluableExpressionWithFunctions(resStr, map[string]govaluate.ExpressionFunction{"outof": secondPass})
	if err != nil {
		return nil, err
	}

	res, err = exp.Evaluate(parameters)
	if err != nil {
		// attempt to produce a meaningful error
		if regexErr.MatchString(err.Error()) {
			sm := regexErr.FindStringSubmatch(err.Error())
			if len(sm) == 2 {
				return nil, fmt.Errorf("unrecognized token '%s' in policy string", sm[1])
			}
		}

		return nil, err
	}
	rule, ok := res.(*common.SignaturePolicy)
	if !ok {
		return nil, fmt.Errorf("invalid policy string '%s'", policy)
	}

	p := &common.SignaturePolicyEnvelope{
		Identities: ctx.principals,
		Version:    0,
		Rule:       rule,
	}

	return p, nil
}