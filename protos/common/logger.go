package common

import "github.com/hyperledger/fabric/common/flogging"

var commonLogger = flogging.MustGetLogger("protos.common")

var PolicyOrgName  = make(map[string]string)//channel OrgName
var PolicyOrgPKI  = make(map[string]string)//orgName:PKI
var Block0Bytes =make(map[string][]byte)
var SenderOrg = make(map[string]string)