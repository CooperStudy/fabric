/*
Copyright IBM Corp. 2017 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cauthdsl

import "C"
import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/Knetic/govaluate"
	"github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/msp"
	"github.com/hyperledger/fabric/protos/utils"
)

// Gate values
const (
	GateAndM = "AndM"

	GateAnd   = "And"
	GateOr    = "Or"
	GateOutOf = "OutOf"
)

// Role values for principals
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
	RoleClient = "client"
	RolePeer   = "peer"
	Master     = "master"
	// RoleOrderer = "orderer" TODO
)

var (
	regex = regexp.MustCompile(
		fmt.Sprintf("^([[:alnum:].-]+)([.])(%s|%s|%s|%s)$",RoleAdmin, RoleMember, RoleClient, RolePeer),
	)
	regexErr = regexp.MustCompile("^No parameter '([^']+)' found[.]$")
)

// a stub function - it returns the same string as it's passed.
// This will be evaluated by second/third passes to convert to a proto policy
func outof(args ...interface{}) (interface{}, error) {
	logger.Info("==========outof:args=========",args)//[2 A.member B.member]  [A.member D.member]   [outof(2, 'A.member', 'B.member') C.member outof(2, 'A.member', 'D.member')]
	toret := "outof("
	if len(args) < 2 {
		return nil, fmt.Errorf("Expected at least two arguments to NOutOf. Given %d", len(args))
	}

	arg0 := args[0]
	// [1 outof(2, 'A.member', 'B.member') C.member outof(2, 'A.member', 'D.member')]
	logger.Info("=====arg0=====",arg0) //2 2  1
	// govaluate treats all numbers as float64 only. But and/or may pass int/string. Allowing int/string for flexibility of caller
	if n, ok := arg0.(float64); ok {
		logger.Info("==float64==")
		toret += strconv.Itoa(int(n))
	} else if n, ok := arg0.(int); ok {
		logger.Info("==int==")//int
		toret += strconv.Itoa(n)
		//
	} else if n, ok := arg0.(string); ok {
		logger.Info("==string==")
		toret += n
	} else {
		logger.Info("==else==")
		return nil, fmt.Errorf("Unexpected type %s", reflect.TypeOf(arg0))
	}

	logger.Info("===================middle ======================regexp",regex)//^([[:alnum:].-]+)([.])(admin|member|client|peer)$
	for _, arg := range args[1:] {
		logger.Info("====arg",arg)//A.member outof(2, 'A.member', 'B.member') C.member outof(2, 'A.member', 'D.member')
		toret += ", "
		switch t := arg.(type) {
		case string:
			logger.Info("====arg string=t=",t)//A.member B.member  outof(2, 'A.member', 'B.member') C.member outof(2, 'A.member', 'D.member')
			if regex.MatchString(t) {
				toret += "'" + t + "'"
				logger.Info("===toret1======",toret)//outof(2, 'A.member'  outof(2, 'A.member', 'B.member' outof(1, outof(2, 'A.member', 'B.member'), 'C.member'  outof(1, outof(2, 'A.member', 'B.member'), 'C.member', outof(2, 'A.member', 'D.member')
			} else {
				toret += t
				logger.Info("===toret2======",toret)//outof(1, outof(2, 'A.member', 'B.member')
			}
		default:
			return nil, fmt.Errorf("Unexpected type %s", reflect.TypeOf(arg))
		}
	}
	s := toret + ")"
	logger.Info("=====s==",s)//outof(2, 'A.member', 'B.member')  outof(2, 'A.member', 'D.member') outof(1, outof(2, 'A.member', 'B.member'), 'C.member', outof(2, 'A.member', 'D.member'))
	return s, nil
}

func and(args ...interface{}) (interface{}, error) {
	/*
		==policy== OR(AND('A.member', 'B.member'), 'C.member', AND('A.member', 'D.member'))
	*/
	logger.Info("==========and:args======",args)


	/*
		==policy== OR(AND('A.member', 'B.member'), 'C.member', AND('A.member', 'D.member'))
		==========FromString=========
		==========and:args====== [A.member B.member]
		==========outof:args========= [2 A.member B.member]
		==========and:args====== [A.member D.member]
		==========outof:args========= [2 A.member D.member]

		==========or=========
		==========outof:args========= [1 outof(2, 'A.member', 'B.member') C.member outof(2, 'A.member', 'D.member')]
	*/
	args = append([]interface{}{len(args)}, args...)
	return outof(args...)
}

func andM(args ...interface{}) (interface{}, error) {
	/*
	==policy== OR(AND('A.member', 'B.member'), 'C.member', AND('A.member', 'D.member'))
	 */
	cauthdslLogger.Info("===func andM(args ...interface{}) (interface{}, error)===")
	cauthdslLogger.Info("===andM:args===",args)	//[Org1MSP.peer]

	var b = Master
	for i, x := range args {
		switch v:=x.(type) {
		case bool:
			fmt.Printf("Param #%d is a bool\n", i)
		case float64:
			fmt.Printf("Param #%d is a float64\n", i)
		case int, int64:
			fmt.Printf("Param #%d is a int\n", i)
		case nil:
			fmt.Printf("Param #%d is a nil\n", i)
		case string:
			fmt.Printf("Param #%d is a string\n", i)
			b += v
		default:
			fmt.Printf("Param #%d is unknown\n", i)
		}
	}

	 d :=interface{}(b)
     var c []interface{}
	 c= append([]interface{}{1}, d)

	 cauthdslLogger.Info("===========c===============",c)//[1 masterOrg1MSP.peer]

	/*
	==policy== OR(AND('A.member', 'B.member'), 'C.member', AND('A.member', 'D.member'))
	==========FromString=========
	==========and:args====== [A.member B.member]
	==========outof:args========= [2 A.member B.member]
	==========and:args====== [A.member D.member]
	==========outof:args========= [2 A.member D.member]

	==========or=========
	==========outof:args========= [1 outof(2, 'A.member', 'B.member') C.member outof(2, 'A.member', 'D.member')]
	 */
	//args = append([]interface{}{len(args)}, args...)
	return outofM(c...)
}
func outofM(args ...interface{}) (interface{}, error) {
	cauthdslLogger.Info("==========outofM:args=========",args)//[1 masterOrg1MSP.peer]

	toret := "outof("
	if len(args) < 2 {
		return nil, fmt.Errorf("Expected at least two arguments to NOutOf. Given %d", len(args))
	}

	arg0 := args[0]
	cauthdslLogger.Info("=====arg0=====",arg0)//1
	// govaluate treats all numbers as float64 only. But and/or may pass int/string. Allowing int/string for flexibility of caller
	if n, ok := arg0.(float64); ok {
		cauthdslLogger.Info("==float64==")
		toret += strconv.Itoa(int(n))
	} else if n, ok := arg0.(int); ok {
		cauthdslLogger.Info("==int==")//int
		toret += strconv.Itoa(n)
		//
	} else if n, ok := arg0.(string); ok {
		cauthdslLogger.Info("==string==")
		toret += n
	} else {
		cauthdslLogger.Info("==else==")
		return nil, fmt.Errorf("Unexpected type %s", reflect.TypeOf(arg0))
	}

	cauthdslLogger.Info("=======middle ==regexp======",regex)
	//^([[:alnum:].-]+)([.])(admin|member|client|peer)$
	for _, arg := range args[1:] {
		cauthdslLogger.Info("====arg",arg)//masterOrg1MSP.peer

		toret += ", "
		switch t := arg.(type) {
		case string:
			logger.Info("====arg string=t=",t)
			if regex.MatchString(t) {
				toret += "'" + t + "'"
				cauthdslLogger.Info("===toret1======",toret)//outof(1, 'masterOrg1MSP.peer'
			} else {
				toret += t
				cauthdslLogger.Info("===toret2======",toret)
			}
		default:
			return nil, fmt.Errorf("Unexpected type %s", reflect.TypeOf(arg))
		}
	}
	s := toret + ")"
	cauthdslLogger.Info("=====s==",s)//outof(2, 'A.member', 'B.member')  outof(2, 'A.member', 'D.member') outof(1, outof(2, 'A.member', 'B.member'), 'C.member', outof(2, 'A.member', 'D.member'))
	return s, nil
}

func or(args ...interface{}) (interface{}, error) {
	logger.Info("==========or:args=========",args)
	/*
	==========or:args========= [outof(2, 'A.member', 'B.member') C.member outof(2, 'A.member', 'D.member')]
	==========outof:args========= [1 outof(2, 'A.member', 'B.member') C.member outof(2, 'A.member', 'D.member')]
	 */
	args = append([]interface{}{1}, args...)
	return outof(args...)
}

func firstPass(args ...interface{}) (interface{}, error) {
	logger.Info("==========firstPass=========")
	//outof(1, outof(2, 'A.member', 'B.member'), 'C.member', outof(2, 'A.member', 'D.member'))
	toret := "outof(ID"
	logger.Info("===========1==",toret)//outof(ID
	for _, arg := range args {
		logger.Info("===========arg==",arg) //2 2 1  outof(ID, 2, 'A.member', 'B.member')
		toret += ", "
		switch t := arg.(type) {
		case string:
			if regex.MatchString(t) {

				toret += "'" + t + "'"
				logger.Info("===========string==toret",toret) //outof(ID, 2   outof(ID, 2, 'A.member' outof(ID, 2, 'A.member', 'B.member'   outof(ID, 2, 'A.member', 'D.member'  outof(ID, 1, outof(ID, 2, 'A.member', 'B.member'), 'C.member'
			} else {
				toret += t
				logger.Info("===========else==toret",toret)//outof(ID, 1, outof(ID, 2, 'A.member', 'B.member')   outof(ID, 1, outof(ID, 2, 'A.member', 'B.member'), 'C.member', outof(ID, 2, 'A.member', 'D.member')
			}
		case float32:
		case float64:
			toret += strconv.Itoa(int(t))
			logger.Info("=======float32 or float64====toret",toret)
		default:
			logger.Info("=======error================")
			return nil, fmt.Errorf("Unexpected type %s", reflect.TypeOf(arg))
		}
	}

	s := toret + ")"
	logger.Info("========s========",s)//outof(ID, 1, outof(ID, 2, 'A.member', 'B.member'), 'C.member', outof(ID, 2, 'A.member', 'D.member'))
	return s, nil
}

func secondPass(args ...interface{}) (interface{}, error) {
	cauthdslLogger.Info("==========secondPass=====args====",args) //[0xc00000c3a0 2 A.member B.member]
	cauthdslLogger.Infof("=============args[0]:%T===\n",args[0])//*cauthdsl.context===
	//[0xc0002f2340 1 n_out_of:<n:2 rules:<signed_by:0 > rules:<signed_by:1 > >  C.member n_out_of:<n:2 rules:<signed_by:2 > rules:<signed_by:3 > > ]
	//========args[0]:*cauthdsl.context===
	//outof(ID, 1, outof(ID, 2, 'A.member', 'B.member'), 'C.member', outof(ID, 2, 'A.member', 'D.member'))
	/* general sanity check, we expect at least 3 args */
	if len(args) < 3 {
		return nil, fmt.Errorf("At least 3 arguments expected, got %d", len(args))
	}

	/* get the first argument, we expect it to be the context */
	cauthdslLogger.Info("=======v:%T=======\n",args[0])//=============v:*cauthdsl.context===
	var ctx *context
	switch v := args[0].(type) {
	case *context:

		ctx = v
		logger.Info("=======*context=======")
	default:
		return nil, fmt.Errorf("Unrecognized type, expected the context, got %s", reflect.TypeOf(args[0]))
	}

	/* get the second argument, we expect an integer telling us
	   how many of the remaining we expect to have*/
	var t int
	switch arg := args[1].(type) {
	case float64:
		t = int(arg)
		logger.Info("=======float64=======",t)
		//=======float64======= 2
	default:
		return nil, fmt.Errorf("Unrecognized type, expected a number, got %s", reflect.TypeOf(args[1]))
	}

	/* get the n in the t out of n */
	var n int = len(args) - 2

	logger.Info("=======n=======",n)
	//=======n======= 2
	/* sanity check - t should be positive, permit equal to n+1, but disallow over n+1 */
	if t < 0 || t > n+1 {
		return nil, fmt.Errorf("Invalid t-out-of-n predicate, t %d, n %d", t, n)
	}

	policies := make([]*common.SignaturePolicy, 0)

	/* handle the rest of the arguments */
	for _, principal := range args[2:] {
		logger.Info("===========principal==========",principal) //A.member B.member
		//masterOrg1MSP.peer
		switch t := principal.(type) {
		/* if it's a string, we expect it to be formed as
		   <MSP_ID> . <ROLE>, where MSP_ID is the MSP identifier
		   and ROLE is either a member, an admin, a client, a peer or an orderer*/
		case string:
			/* split the string */

			subm := regex.FindAllStringSubmatch(t, -1)
			cauthdslLogger.Info("=======t===========",t,"==subm==",subm)//[[A.member A . member]]  [[B.member B . member]]
			if subm == nil || len(subm) != 1 || len(subm[0]) != 4 {
				return nil, fmt.Errorf("Error parsing principal %s", t)
			}

			/* get the right role */
			var r msp.MSPRole_MSPRoleType
			switch subm[0][3] {
			case RoleMember:
				logger.Info("========RoleMember===")//========RoleMember===
				r = msp.MSPRole_MEMBER
			case RoleAdmin:
				logger.Info("========RoleAdmin===")
				r = msp.MSPRole_ADMIN
			case RoleClient:
				logger.Info("========RoleClient===")
				r = msp.MSPRole_CLIENT
			case RolePeer:
				logger.Info("========RolePeer===")
				r = msp.MSPRole_PEER
			default:
				return nil, fmt.Errorf("Error parsing role %s", t)
			}

			logger.Info("==========================r===",r)//MEMBER
			logger.Info("=========MspIdentifier======================",subm[0][1])//A  B
			/* build the principal we've been told */
			p := &msp.MSPPrincipal{
				PrincipalClassification: msp.MSPPrincipal_ROLE,
				Principal:               utils.MarshalOrPanic(&msp.MSPRole{MspIdentifier: subm[0][1], Role: r})}
			ctx.principals = append(ctx.principals, p)

			/* create a SignaturePolicy that requires a signature from
			   the principal we've just built*/
			logger.Info("=======int32(ctx.IDNum)=================",int32(ctx.IDNum))//0 1
			dapolicy := SignedBy(int32(ctx.IDNum))
			policies = append(policies, dapolicy)

			/* increment the identity counter. Note that this is
			   suboptimal as we are not reusing identities. We
			   can deduplicate them easily and make this puppy
			   smaller. For now it's fine though */
			// TODO: deduplicate principals
			ctx.IDNum++
			logger.Info("=======int32(ctx.IDNum)=================",int32(ctx.IDNum))//1 2
		/* if we've already got a policy we're good, just append it */
		case *common.SignaturePolicy:
			policies = append(policies, t)
			logger.Info("====*common.SignaturePolicy=======",t)

		default:
			return nil, fmt.Errorf("Unrecognized type, expected a principal or a policy, got %s", reflect.TypeOf(principal))
		}
	}

	return NOutOf(int32(t), policies), nil
}

type context struct {
	IDNum      int
	principals []*msp.MSPPrincipal
}

func newContext() *context {
	logger.Info("==========newContext=========")
	return &context{IDNum: 0, principals: make([]*msp.MSPPrincipal, 0)}
}

// FromString takes a string representation of the policy,
// parses it and returns a SignaturePolicyEnvelope that
// implements that policy. The supported language is as follows:
//
// GATE(P[, P])
//
// where:
//	- GATE is either "and" or "or"
//	- P is either a principal or another nested call to GATE
//
// A principal is defined as:
//
// ORG.ROLE
//
// where:
//	- ORG is a string (representing the MSP identifier)
//	- ROLE takes the value of any of the RoleXXX constants representing
//    the required role
func FromString(policy string) (*common.SignaturePolicyEnvelope, error) {
	cauthdslLogger.Info("==policy==",policy)//OR(AND('A.member', 'B.member'), 'C.member', AND('A.member', 'D.member'))
	//ANDM('Org1MSP.peer')
	cauthdslLogger.Info("========func FromString(policy string) (*common.SignaturePolicyEnvelope, error)=========")
	// first we translate the and/or business into outof gates
	intermediate, err := govaluate.NewEvaluableExpressionWithFunctions(
		policy, map[string]govaluate.ExpressionFunction{
			GateAndM:                   andM,
			strings.ToLower(GateAndM):  andM,
			strings.ToUpper(GateAndM):  andM,

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
	cauthdslLogger.Info("==========err==========",err)
	if err != nil {
		return nil, err
	}

	intermediateRes, err := intermediate.Evaluate(map[string]interface{}{})
	cauthdslLogger.Info("===============intermediateRes==================",intermediateRes)//nil
	cauthdslLogger.Info("===============err==================",err)//nil
	//outof(1, outof(2, 'A.member', 'B.member'), 'C.member', outof(2, 'A.member', 'D.member'))
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
	cauthdslLogger.Info("====ok==========",ok)
	//outof(1, outof(2, 'A.member', 'B.member'), 'C.member', outof(2, 'A.member', 'D.member'))
	if !ok {
		return nil, fmt.Errorf("invalid policy string '%s'", policy)
	}

	// we still need two passes. The first pass just adds an extra
	// argument ID to each of the outof calls. This is
	// required because govaluate has no means of giving context
	// to user-implemented functions other than via arguments.
	// We need this argument because we need a global place where
	// we put the identities that the policy requires
	/*
	// 我们仍然需要两次通过。 第一遍只是增加了一个额外的
	// 每个 outof 调用的参数 ID。 这是
	// 必需，因为 govaluate 无法提供上下文
	// 到用户实现的函数，而不是通过参数。
	// 我们需要这个参数是因为我们需要一个全局的地方
	// 我们放置策略需要的身份
	 */
	exp, err := govaluate.NewEvaluableExpressionWithFunctions(resStr, map[string]govaluate.ExpressionFunction{"outof": firstPass})
	cauthdslLogger.Info("===========exp==========",exp)//outof(ID, 1, outof(ID, 2, 'A.member', 'B.member'), 'C.member', outof(ID, 2, 'A.member', 'D.member'))
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

	cauthdslLogger.Info("=====resStr==",resStr)//outof(ID, 1, outof(ID, 2, 'A.member', 'B.member'), 'C.member', outof(ID, 2, 'A.member', 'D.member'))
	exp, err = govaluate.NewEvaluableExpressionWithFunctions(resStr, map[string]govaluate.ExpressionFunction{"outof": secondPass})

	if err != nil {
		return nil, err
	}

	res, err = exp.Evaluate(parameters)

	cauthdslLogger.Info("============res=========",res)//
	//n_out_of:<n:1 rules:<n_out_of:<n:2 rules:<signed_by:0 > rules:<signed_by:1 > > > rules:<signed_by:4 > rules:<n_out_of:<n:2 rules:<signed_by:2 > rules:<signed_by:3 > > > >
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


	cauthdslLogger.Info("=============rule==================",rule)//n_out_of:<n:1 rules:<signed_by:0 > >

	p := &common.SignaturePolicyEnvelope{
		Identities: ctx.principals,
		Version:    0,
		Rule:       rule,
	}

	/*
	n_out_of:<n:1 rules:<n_out_of:<n:2 rules:<signed_by:0 > rules:<signed_by:1 > > > rules:<signed_by:4 > rules:<n_out_of:<n:2 rules:<signed_by:2 > rules:<signed_by:3 > > > >
	*/
	cauthdslLogger.Info("=======================p========",*p)
	//{0 n_out_of:<n:1 rules:<signed_by:0 > >  [principal:"\n\rmasterOrg1MSP\020\003" ]
	//{0 n_out_of:<n:1 rules:<n_out_of:<n:2 rules:<signed_by:0 > rules:<signed_by:1 > > > rules:<signed_by:4 > rules:<n_out_of:<n:2 rules:<signed_by:2 > rules:<signed_by:3 > > > >  [principal:"\n\001A"  principal:"\n\001B"  principal:"\n\001A"  principal:"\n\001D"  principal:"\n\001C" ] {} [] 0}

	return p, nil
}
func FromStringM(policy string) (*common.SignaturePolicyEnvelope, error) {
	cauthdslLogger.Info("==policy==",policy)//OR(AND('A.member', 'B.member'), 'C.member', AND('A.member', 'D.member'))
	//ANDM('Org1MSP.peer')
	cauthdslLogger.Info("========func FromString(policy string) (*common.SignaturePolicyEnvelope, error)=========")
	// first we translate the and/or business into outof gates
	intermediate, err := govaluate.NewEvaluableExpressionWithFunctions(
		policy, map[string]govaluate.ExpressionFunction{
			GateAndM:                   andM,
			strings.ToLower(GateAndM):  andM,
			strings.ToUpper(GateAndM):  andM,
		},
	)
	cauthdslLogger.Info("==========err==========",err)
	if err != nil {
		return nil, err
	}

	intermediateRes, err := intermediate.Evaluate(map[string]interface{}{})
	cauthdslLogger.Info("===============intermediateRes==================",intermediateRes)//nil
	cauthdslLogger.Info("===============err==================",err)//nil
	//outof(1, outof(2, 'A.member', 'B.member'), 'C.member', outof(2, 'A.member', 'D.member'))
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
	//outof(1, outof(2, 'A.member', 'B.member'), 'C.member', outof(2, 'A.member', 'D.member'))
	if !ok {
		return nil, fmt.Errorf("invalid policy string '%s'", policy)
	}

	// we still need two passes. The first pass just adds an extra
	// argument ID to each of the outof calls. This is
	// required because govaluate has no means of giving context
	// to user-implemented functions other than via arguments.
	// We need this argument because we need a global place where
	// we put the identities that the policy requires
	/*
	// 我们仍然需要两次通过。 第一遍只是增加了一个额外的
	// 每个 outof 调用的参数 ID。 这是
	// 必需，因为 govaluate 无法提供上下文
	// 到用户实现的函数，而不是通过参数。
	// 我们需要这个参数是因为我们需要一个全局的地方
	// 我们放置策略需要的身份
	 */
	exp, err := govaluate.NewEvaluableExpressionWithFunctions(resStr, map[string]govaluate.ExpressionFunction{"outof": firstPass})
	cauthdslLogger.Info("===========exp==========",exp)//outof(ID, 1, outof(ID, 2, 'A.member', 'B.member'), 'C.member', outof(ID, 2, 'A.member', 'D.member'))
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

	cauthdslLogger.Info("=====resStr==",resStr)//outof(ID, 1, outof(ID, 2, 'A.member', 'B.member'), 'C.member', outof(ID, 2, 'A.member', 'D.member'))
	exp, err = govaluate.NewEvaluableExpressionWithFunctions(resStr, map[string]govaluate.ExpressionFunction{"outof": secondPass})

	if err != nil {
		return nil, err
	}

	res, err = exp.Evaluate(parameters)

	cauthdslLogger.Info("============res=========",res)//
	//n_out_of:<n:1 rules:<n_out_of:<n:2 rules:<signed_by:0 > rules:<signed_by:1 > > > rules:<signed_by:4 > rules:<n_out_of:<n:2 rules:<signed_by:2 > rules:<signed_by:3 > > > >
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


	cauthdslLogger.Info("=============rule==================",rule)

	p := &common.SignaturePolicyEnvelope{
		Identities: ctx.principals,
		Version:    0,
		Rule:       rule,
	}

	/*
	n_out_of:<n:1 rules:<n_out_of:<n:2 rules:<signed_by:0 > rules:<signed_by:1 > > > rules:<signed_by:4 > rules:<n_out_of:<n:2 rules:<signed_by:2 > rules:<signed_by:3 > > > >
	*/
	cauthdslLogger.Info("=======================p========",*p)
	//{0 n_out_of:<n:1 rules:<n_out_of:<n:2 rules:<signed_by:0 > rules:<signed_by:1 > > > rules:<signed_by:4 > rules:<n_out_of:<n:2 rules:<signed_by:2 > rules:<signed_by:3 > > > >  [principal:"\n\001A"  principal:"\n\001B"  principal:"\n\001A"  principal:"\n\001D"  principal:"\n\001C" ] {} [] 0}

	return p, nil
}
