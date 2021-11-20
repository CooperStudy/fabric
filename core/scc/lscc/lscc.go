/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package lscc

import (
	"fmt"
	"regexp"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/cauthdsl"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/aclmgmt"
	"github.com/hyperledger/fabric/core/aclmgmt/resources"
	"github.com/hyperledger/fabric/core/chaincode/platforms"
	"github.com/hyperledger/fabric/core/chaincode/platforms/ccmetadata"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/core/common/ccprovider"
	"github.com/hyperledger/fabric/core/common/privdata"
	"github.com/hyperledger/fabric/core/common/sysccprovider"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/cceventmgmt"
	"github.com/hyperledger/fabric/core/peer"
	"github.com/hyperledger/fabric/core/policy"
	"github.com/hyperledger/fabric/core/policyprovider"
	"github.com/hyperledger/fabric/msp"
	"github.com/hyperledger/fabric/msp/mgmt"
	"github.com/hyperledger/fabric/protos/common"
	mb "github.com/hyperledger/fabric/protos/msp"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/hyperledger/fabric/protos/utils"
	"github.com/pkg/errors"
)

//The life cycle system chaincode manages chaincodes deployed
//on this peer. It manages chaincodes via Invoke proposals.
//     "Args":["deploy",<ChaincodeDeploymentSpec>]
//     "Args":["upgrade",<ChaincodeDeploymentSpec>]
//     "Args":["stop",<ChaincodeInvocationSpec>]
//     "Args":["start",<ChaincodeInvocationSpec>]

var logger = flogging.MustGetLogger("lscc")

const (
	// chaincode lifecycle commands

	// INSTALL install command
	INSTALL = "install"

	// DEPLOY deploy command
	DEPLOY = "deploy"

	// UPGRADE upgrade chaincode
	UPGRADE = "upgrade"

	// CCEXISTS get chaincode
	CCEXISTS = "getid"

	// CHAINCODEEXISTS get chaincode alias
	CHAINCODEEXISTS = "ChaincodeExists"

	// GETDEPSPEC get ChaincodeDeploymentSpec
	GETDEPSPEC = "getdepspec"

	// GETDEPLOYMENTSPEC get ChaincodeDeploymentSpec alias
	GETDEPLOYMENTSPEC = "GetDeploymentSpec"

	// GETCCDATA get ChaincodeData
	GETCCDATA = "getccdata"

	// GETCHAINCODEDATA get ChaincodeData alias
	GETCHAINCODEDATA = "GetChaincodeData"

	// GETCHAINCODES gets the instantiated chaincodes on a channel
	GETCHAINCODES = "getchaincodes"

	// GETCHAINCODESALIAS gets the instantiated chaincodes on a channel
	GETCHAINCODESALIAS = "GetChaincodes"

	// GETINSTALLEDCHAINCODES gets the installed chaincodes on a peer
	GETINSTALLEDCHAINCODES = "getinstalledchaincodes"

	// GETINSTALLEDCHAINCODESALIAS gets the installed chaincodes on a peer
	GETINSTALLEDCHAINCODESALIAS = "GetInstalledChaincodes"

	// GETCOLLECTIONSCONFIG gets the collections config for a chaincode
	GETCOLLECTIONSCONFIG = "GetCollectionsConfig"

	// GETCOLLECTIONSCONFIGALIAS gets the collections config for a chaincode
	GETCOLLECTIONSCONFIGALIAS = "getcollectionsconfig"

	allowedChaincodeName = "^[a-zA-Z0-9]+([-_][a-zA-Z0-9]+)*$"
	allowedCharsVersion  = "[A-Za-z0-9_.+-]+"
)

// FilesystemSupport contains functions that LSCC requires to execute its tasks
type FilesystemSupport interface {
	// PutChaincodeToLocalStorage stores the supplied chaincode
	// package to local storage (i.e. the file system)
	PutChaincodeToLocalStorage(ccprovider.CCPackage) error

	// GetChaincodeFromLocalStorage retrieves the chaincode package
	// for the requested chaincode, specified by name and version
	GetChaincodeFromLocalStorage(ccname string, ccversion string) (ccprovider.CCPackage, error)

	// GetChaincodesFromLocalStorage returns an array of all chaincode
	// data that have previously been persisted to local storage
	GetChaincodesFromLocalStorage() (*pb.ChaincodeQueryResponse, error)

	// GetInstantiationPolicy returns the instantiation policy for the
	// supplied chaincode (or the channel's default if none was specified)
	GetInstantiationPolicy(channel string, ccpack ccprovider.CCPackage) ([]byte, error)

	// CheckInstantiationPolicy checks whether the supplied signed proposal
	// complies with the supplied instantiation policy
	CheckInstantiationPolicy(signedProposal *pb.SignedProposal, chainName string, instantiationPolicy []byte) error
}

//---------- the LSCC -----------------

// LifeCycleSysCC implements chaincode lifecycle and policies around it
type LifeCycleSysCC struct {
	// aclProvider is responsible for access control evaluation
	ACLProvider aclmgmt.ACLProvider

	// SCCProvider is the interface which is passed into system chaincodes
	// to access other parts of the system
	SCCProvider sysccprovider.SystemChaincodeProvider

	// PolicyChecker is the interface used to perform
	// access control
	PolicyChecker policy.PolicyChecker

	// Support provides the implementation of several
	// static functions
	Support FilesystemSupport

	PlatformRegistry *platforms.Registry
}

// New creates a new instance of the LSCC
// Typically there is only one of these per peer
func New(sccp sysccprovider.SystemChaincodeProvider, ACLProvider aclmgmt.ACLProvider, platformRegistry *platforms.Registry) *LifeCycleSysCC {
	fmt.Println("===New===")
	return &LifeCycleSysCC{
		Support:          &supportImpl{},
		PolicyChecker:    policyprovider.GetPolicyChecker(),
		SCCProvider:      sccp,
		ACLProvider:      ACLProvider,
		PlatformRegistry: platformRegistry,
	}
}

func (lscc *LifeCycleSysCC) Name() string              { return "lscc" }
func (lscc *LifeCycleSysCC) Path() string              { return "github.com/hyperledger/fabric/core/scc/lscc" }
func (lscc *LifeCycleSysCC) InitArgs() [][]byte        { return nil }
func (lscc *LifeCycleSysCC) Chaincode() shim.Chaincode { return lscc }
func (lscc *LifeCycleSysCC) InvokableExternal() bool   { return true }
func (lscc *LifeCycleSysCC) InvokableCC2CC() bool      { return true }
func (lscc *LifeCycleSysCC) Enabled() bool             { return true }

func (lscc *LifeCycleSysCC) ChaincodeContainerInfo(chaincodeName string, qe ledger.QueryExecutor) (*ccprovider.ChaincodeContainerInfo, error) {
	fmt.Println("===LifeCycleSysCC==ChaincodeContainerInfo=")
	chaincodeDataBytes, err := qe.GetState("lscc", chaincodeName)
	if err != nil {
		return nil, errors.Wrapf(err, "could not retrieve state for chaincode %s", chaincodeName)
	}

	if chaincodeDataBytes == nil {
		return nil, errors.Errorf("chaincode %s not found", chaincodeName)
	}

	// Note, although it looks very tempting to replace the bulk of this function with
	// the below 'ChaincodeDefinition' call, the 'getCCCode' call provides us security
	// by side-effect, so we must leave it as is for now.
	cds, _, err := lscc.getCCCode(chaincodeName, chaincodeDataBytes)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get chaincode code")
	}

	return ccprovider.DeploymentSpecToChaincodeContainerInfo(cds), nil
}

func (lscc *LifeCycleSysCC) ChaincodeDefinition(chaincodeName string, qe ledger.QueryExecutor) (ccprovider.ChaincodeDefinition, error) {
	fmt.Println("===LifeCycleSysCC==ChaincodeDefinition=")
	chaincodeDataBytes, err := qe.GetState("lscc", chaincodeName)
	if err != nil {
		return nil, errors.Wrapf(err, "could not retrieve state for chaincode %s", chaincodeName)
	}

	if chaincodeDataBytes == nil {
		return nil, errors.Errorf("chaincode %s not found", chaincodeName)
	}

	chaincodeData := &ccprovider.ChaincodeData{}
	err = proto.Unmarshal(chaincodeDataBytes, chaincodeData)
	if err != nil {
		return nil, errors.Wrapf(err, "chaincode %s has bad definition", chaincodeName)
	}

	return chaincodeData, nil
}

//create the chaincode on the given chain
func (lscc *LifeCycleSysCC) putChaincodeData(stub shim.ChaincodeStubInterface, cd *ccprovider.ChaincodeData) error {
	fmt.Println("===LifeCycleSysCC==putChaincodeData=")
	cdbytes, err := proto.Marshal(cd)
	if err != nil {
		return err
	}

	if cdbytes == nil {
		return MarshallErr(cd.Name)
	}

	err = stub.PutState(cd.Name, cdbytes)

	return err
}

// checkCollectionMemberPolicy checks whether the supplied collection configuration
// complies to the given msp configuration
func checkCollectionMemberPolicy(collectionConfig *common.CollectionConfig, mspmgr msp.MSPManager) error {
	fmt.Println("===checkCollectionMemberPolicy====")
	if mspmgr == nil {
		return fmt.Errorf("msp manager not set")
	}
	msps, err := mspmgr.GetMSPs()
	if err != nil {
		return errors.Wrapf(err, "error getting channel msp")
	}
	if collectionConfig == nil {
		return fmt.Errorf("collection configuration is not set")
	}
	coll := collectionConfig.GetStaticCollectionConfig()
	if coll == nil {
		return fmt.Errorf("collection configuration is empty")
	}
	if coll.MemberOrgsPolicy == nil {
		return fmt.Errorf("collection member policy is not set")
	}
	if coll.MemberOrgsPolicy.GetSignaturePolicy() == nil {
		return fmt.Errorf("collection member org policy is empty")
	}
	// make sure that the orgs listed are actually part of the channel
	// check all principals in the signature policy
	for _, principal := range coll.MemberOrgsPolicy.GetSignaturePolicy().Identities {
		found := false
		var orgID string
		// the member org policy only supports certain principal types
		switch principal.PrincipalClassification {

		case mb.MSPPrincipal_ROLE:
			msprole := &mb.MSPRole{}
			err := proto.Unmarshal(principal.Principal, msprole)
			if err != nil {
				return errors.Wrapf(err, "collection-name: %s -- cannot unmarshal identities", coll.GetName())
			}
			orgID = msprole.MspIdentifier
			// the msp map is indexed using msp IDs - this behavior is implementation specific, making the following check a bit of a hack
			for mspid := range msps {
				if mspid == orgID {
					found = true
					break
				}
			}

		case mb.MSPPrincipal_ORGANIZATION_UNIT:
			mspou := &mb.OrganizationUnit{}
			err := proto.Unmarshal(principal.Principal, mspou)
			if err != nil {
				return errors.Wrapf(err, "collection-name: %s -- cannot unmarshal identities", coll.GetName())
			}
			orgID = mspou.MspIdentifier
			// the msp map is indexed using msp IDs - this behavior is implementation specific, making the following check a bit of a hack
			for mspid := range msps {
				if mspid == orgID {
					found = true
					break
				}
			}

		case mb.MSPPrincipal_IDENTITY:
			orgID = "identity principal"
			for _, msp := range msps {
				_, err := msp.DeserializeIdentity(principal.Principal)
				if err == nil {
					found = true
					break
				}
			}
		default:
			return fmt.Errorf("collection-name: %s -- principal type %v is not supported", coll.GetName(), principal.PrincipalClassification)
		}
		if !found {
			logger.Warningf("collection-name: %s collection member %s is not part of the channel", coll.GetName(), orgID)
		}
	}

	return nil
}

// putChaincodeCollectionData adds collection data for the chaincode
func (lscc *LifeCycleSysCC) putChaincodeCollectionData(stub shim.ChaincodeStubInterface, cd *ccprovider.ChaincodeData, collectionConfigBytes []byte) error {
	fmt.Println("===LifeCycleSysCC==putChaincodeCollectionData==")
	if cd == nil {
		return errors.New("nil ChaincodeData")
	}

	if len(collectionConfigBytes) == 0 {
		logger.Debug("No collection configuration specified")
		return nil
	}

	collections := &common.CollectionConfigPackage{}
	err := proto.Unmarshal(collectionConfigBytes, collections)
	if err != nil {
		return errors.Errorf("invalid collection configuration supplied for chaincode %s:%s", cd.Name, cd.Version)
	}

	mspmgr := mgmt.GetManagerForChain(stub.GetChannelID())
	if mspmgr == nil {
		return fmt.Errorf("could not get MSP manager for channel %s", stub.GetChannelID())
	}
	for _, collectionConfig := range collections.Config {
		err = checkCollectionMemberPolicy(collectionConfig, mspmgr)
		if err != nil {
			return errors.Wrapf(err, "collection member policy check failed")
		}
	}

	key := privdata.BuildCollectionKVSKey(cd.Name)

	err = stub.PutState(key, collectionConfigBytes)
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("error putting collection for chaincode %s:%s", cd.Name, cd.Version))
	}

	return nil
}

// getChaincodeCollectionData retrieve collections config.
func (lscc *LifeCycleSysCC) getChaincodeCollectionData(stub shim.ChaincodeStubInterface, chaincodeName string) pb.Response {
	fmt.Println("===LifeCycleSysCC==getChaincodeCollectionData==")
	key := privdata.BuildCollectionKVSKey(chaincodeName)
	collectionsConfigBytes, err := stub.GetState(key)
	if err != nil {
		return shim.Error(err.Error())
	}
	if len(collectionsConfigBytes) == 0 {
		return shim.Error(fmt.Sprintf("collections config not defined for chaincode %s", chaincodeName))
	}
	return shim.Success(collectionsConfigBytes)
}

//checks for existence of chaincode on the given channel
func (lscc *LifeCycleSysCC) getCCInstance(stub shim.ChaincodeStubInterface, ccname string) ([]byte, error) {
	fmt.Println("===LifeCycleSysCC==getCCInstance==")
	cdbytes, err := stub.GetState(ccname)
	if err != nil {
		return nil, TXNotFoundErr(err.Error())
	}
	if cdbytes == nil {
		return nil, NotFoundErr(ccname)
	}

	return cdbytes, nil
}

//gets the cd out of the bytes
func (lscc *LifeCycleSysCC) getChaincodeData(ccname string, cdbytes []byte) (*ccprovider.ChaincodeData, error) {
	fmt.Println("===LifeCycleSysCC==getChaincodeData==")
	cd := &ccprovider.ChaincodeData{}
	err := proto.Unmarshal(cdbytes, cd)
	if err != nil {
		return nil, MarshallErr(ccname)
	}

	//this should not happen but still a sanity check is not a bad thing
	if cd.Name != ccname {
		return nil, ChaincodeMismatchErr(fmt.Sprintf("%s!=%s", ccname, cd.Name))
	}

	return cd, nil
}

//checks for existence of chaincode on the given chain
func (lscc *LifeCycleSysCC) getCCCode(ccname string, cdbytes []byte) (*pb.ChaincodeDeploymentSpec, []byte, error) {
	fmt.Println("===LifeCycleSysCC==getCCCode==")
	cd, err := lscc.getChaincodeData(ccname, cdbytes)
	if err != nil {
		return nil, nil, err
	}

	ccpack, err := lscc.Support.GetChaincodeFromLocalStorage(ccname, cd.Version)
	if err != nil {
		return nil, nil, InvalidDeploymentSpecErr(err.Error())
	}

	//this is the big test and the reason every launch should go through
	//getChaincode call. We validate the chaincode entry against the
	//the chaincode in FS
	if err = ccpack.ValidateCC(cd); err != nil {
		return nil, nil, InvalidCCOnFSError(err.Error())
	}

	//these are guaranteed to be non-nil because we got a valid ccpack
	depspec := ccpack.GetDepSpec()
	depspecbytes := ccpack.GetDepSpecBytes()

	return depspec, depspecbytes, nil
}

// getChaincodes returns all chaincodes instantiated on this LSCC's channel
func (lscc *LifeCycleSysCC) getChaincodes(stub shim.ChaincodeStubInterface) pb.Response {
	fmt.Println("===LifeCycleSysCC==getChaincodes==")
	// get all rows from LSCC
	itr, err := stub.GetStateByRange("", "")

	if err != nil {
		return shim.Error(err.Error())
	}
	defer itr.Close()

	// array to store metadata for all chaincode entries from LSCC
	var ccInfoArray []*pb.ChaincodeInfo

	for itr.HasNext() {
		response, err := itr.Next()
		if err != nil {
			return shim.Error(err.Error())
		}

		// CollectionConfig isn't ChaincodeData
		if privdata.IsCollectionConfigKey(response.Key) {
			continue
		}

		ccdata := &ccprovider.ChaincodeData{}
		if err = proto.Unmarshal(response.Value, ccdata); err != nil {
			return shim.Error(err.Error())
		}

		var path string
		var input string

		// if chaincode is not installed on the system we won't have
		// data beyond name and version
		ccpack, err := lscc.Support.GetChaincodeFromLocalStorage(ccdata.Name, ccdata.Version)
		if err == nil {
			path = ccpack.GetDepSpec().GetChaincodeSpec().ChaincodeId.Path
			input = ccpack.GetDepSpec().GetChaincodeSpec().Input.String()
		}

		// add this specific chaincode's metadata to the array of all chaincodes
		ccInfo := &pb.ChaincodeInfo{Name: ccdata.Name, Version: ccdata.Version, Path: path, Input: input, Escc: ccdata.Escc, Vscc: ccdata.Vscc}
		ccInfoArray = append(ccInfoArray, ccInfo)
	}
	// add array with info about all instantiated chaincodes to the query
	// response proto
	cqr := &pb.ChaincodeQueryResponse{Chaincodes: ccInfoArray}

	cqrbytes, err := proto.Marshal(cqr)
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(cqrbytes)
}

// getInstalledChaincodes returns all chaincodes installed on the peer
func (lscc *LifeCycleSysCC) getInstalledChaincodes() pb.Response {
	fmt.Println("===LifeCycleSysCC==getInstalledChaincodes==")
	// get chaincode query response proto which contains information about all
	// installed chaincodes
	cqr, err := lscc.Support.GetChaincodesFromLocalStorage()
	if err != nil {
		return shim.Error(err.Error())
	}

	cqrbytes, err := proto.Marshal(cqr)
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(cqrbytes)
}

// check validity of channel name
func (lscc *LifeCycleSysCC) isValidChannelName(channel string) bool {
	fmt.Println("===LifeCycleSysCC==isValidChannelName==")
	// TODO we probably need more checks
	if channel == "" {
		return false
	}
	return true
}

// isValidChaincodeName checks the validity of chaincode name. Chaincode names
// should never be blank and should only consist of alphanumerics, '_', and '-'
func (lscc *LifeCycleSysCC) isValidChaincodeName(chaincodeName string) error {
	fmt.Println("===LifeCycleSysCC==isValidChaincodeName==")
	if chaincodeName == "" {
		return EmptyChaincodeNameErr("")
	}

	if !isValidCCNameOrVersion(chaincodeName, allowedChaincodeName) {
		return InvalidChaincodeNameErr(chaincodeName)
	}

	return nil
}

// isValidChaincodeVersion checks the validity of chaincode version. Versions
// should never be blank and should only consist of alphanumerics, '_',  '-',
// '+', and '.'
func (lscc *LifeCycleSysCC) isValidChaincodeVersion(chaincodeName string, version string) error {
	fmt.Println("===LifeCycleSysCC==isValidChaincodeVersion==")
	if version == "" {
		return EmptyVersionErr(chaincodeName)
	}

	if !isValidCCNameOrVersion(version, allowedCharsVersion) {
		return InvalidVersionErr(version)
	}

	return nil
}

func isValidCCNameOrVersion(ccNameOrVersion string, regExp string) bool {
	fmt.Println("===isValidCCNameOrVersion==")
	re, _ := regexp.Compile(regExp)

	matched := re.FindString(ccNameOrVersion)
	if len(matched) != len(ccNameOrVersion) {
		return false
	}

	return true
}

func isValidStatedbArtifactsTar(statedbArtifactsTar []byte) error {
	fmt.Println("===isValidStatedbArtifactsTar==")
	// Extract the metadata files from the archive
	// Passing an empty string for the databaseType will validate all artifacts in
	// the archive
	archiveFiles, err := ccprovider.ExtractFileEntries(statedbArtifactsTar, "")
	if err != nil {
		return err
	}
	// iterate through the files and validate
	for _, archiveDirectoryFiles := range archiveFiles {
		for _, fileEntry := range archiveDirectoryFiles {
			indexData := fileEntry.FileContent
			// Validation is based on the passed file name, e.g. META-INF/statedb/couchdb/indexes/indexname.json
			err = ccmetadata.ValidateMetadataFile(fileEntry.FileHeader.Name, indexData)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// executeInstall implements the "install" Invoke transaction
func (lscc *LifeCycleSysCC) executeInstall(stub shim.ChaincodeStubInterface, ccbytes []byte) error {
	fmt.Println("===LifeCycleSysCC==executeInstall==")
	fmt.Println("===========ccbytes====================",ccbytes)
	/*
	[10 84 8 1 18 80 10 70 103 105 116 104 117 98 46 99 111 109 47 104 121 112 101 114 108 101 100 103 101 114 47 102 97 98 114 105 99 45 115 97 109 112 108 101 115 47 99 104 97 105 110 99 111 100 101 47 99 104 97 105 110 99 111 100 101 95 101 120 97 109 112 108 101 48 50 47 103 111 18 3 97 99 98 26 1 48 26 176 7 31 139 8 0 0 0 0 0 0 255 212 86 203 107 220 70 24 247 85 243 87 124 17 20 164 226 74 178 93 28 48 232 16 210 38 49 129 98 226 210 139 49 101 86 250 180 43 44 205 136 153 79 78 150 98 104 104 75 82 72 160 80 231 208 180 148 62 46 133 30 108 122 40 164 110 154 127 198 187 182 79 253 23 202 232 177 15 103 215 110 19 40 68 23 141 230 245 123 124 191 217 89 173 34 191 155 82 175 236 120 145 204 253 94 191 64 149 97 220 69 229 39 188 163 210 232 29 205 243 34 67 237 71 61 158 138 72 198 56 110 125 140 247 170 193 96 217 239 202 113 183 215 149 11 147 79 16 4 193 234 234 187 213 59 8 130 243 239 149 171 43 75 109 187 238 95 190 186 186 188 178 0 193 194 255 240 148 154 184 90 8 94 27 235 188 184 55 228 41 120 180 195 187 8 57 79 5 99 105 94 72 69 224 48 203 78 114 178 153 101 95 152 13 63 146 106 34 14 190 238 165 249 229 139 10 37 73 106 191 64 84 54 115 25 163 126 129 240 33 106 2 77 170 140 8 62 97 214 109 236 131 249 76 69 151 89 239 113 226 237 199 30 99 73 41 34 112 168 151 106 120 219 44 115 97 93 164 228 104 42 59 96 24 120 215 91 66 155 84 118 214 5 161 74 120 132 46 24 68 239 14 234 66 10 141 6 69 33 149 74 212 139 54 203 40 66 173 29 145 102 238 60 148 93 185 131 175 130 147 228 228 109 168 84 80 38 28 59 12 195 122 167 48 12 109 151 89 137 88 132 130 43 158 35 161 210 176 22 130 65 240 110 34 221 40 69 68 169 20 215 68 188 49 154 224 184 204 242 125 56 251 244 233 233 139 7 131 7 207 135 79 14 7 95 61 30 60 122 118 246 197 227 193 193 179 193 31 251 39 79 63 63 251 250 175 227 163 159 79 126 184 95 79 56 61 252 236 100 255 23 102 165 9 36 2 194 16 108 30 199 198 85 219 176 123 137 94 51 216 240 107 93 50 86 120 205 80 101 194 36 109 151 89 123 128 153 70 24 99 116 145 230 99 52 131 179 48 154 161 153 24 172 18 63 252 237 199 225 119 95 30 31 29 29 63 127 210 40 252 233 215 198 130 135 135 127 255 249 232 244 197 254 224 219 239 207 246 191 57 61 56 24 254 254 112 120 255 96 186 216 239 43 37 149 99 223 193 8 211 93 140 161 20 59 66 222 133 164 49 28 82 177 43 35 110 154 246 156 48 76 26 113 81 26 22 129 171 174 134 173 237 58 191 151 134 163 245 222 118 25 179 118 185 2 50 7 195 96 86 229 203 80 56 102 67 23 174 132 176 92 57 59 67 215 181 56 6 212 196 9 193 80 82 10 35 2 81 230 29 84 32 19 195 168 204 81 144 246 236 198 83 3 226 153 51 23 86 116 183 130 237 10 109 220 29 130 109 207 67 219 193 62 68 92 128 144 4 29 4 204 11 234 79 239 92 29 224 102 235 165 109 198 44 84 106 20 244 141 146 54 13 85 167 69 91 132 173 237 78 191 237 48 107 93 183 162 99 86 93 9 65 164 217 28 42 168 84 211 114 27 248 255 118 194 39 147 247 122 69 157 2 108 228 152 216 79 6 63 180 27 93 83 69 93 154 103 243 122 93 73 113 81 37 45 83 139 181 137 34 42 212 101 70 122 157 80 113 146 106 17 38 157 191 137 116 43 213 36 85 255 134 84 183 177 239 236 96 223 196 238 85 172 182 98 76 80 193 57 60 239 122 38 53 58 109 150 99 147 131 218 141 139 44 234 97 150 201 149 145 67 137 124 121 223 91 92 127 128 247 200 113 27 122 181 243 35 121 231 167 215 115 153 53 67 218 165 218 170 28 89 113 29 225 22 201 251 136 103 37 86 194 103 69 204 204 30 103 204 220 173 53 211 214 253 106 46 113 69 142 192 187 78 21 190 153 17 159 250 109 168 40 129 249 187 66 169 232 66 34 101 12 163 187 23 214 222 210 118 165 191 42 199 30 251 151 247 255 63 0 0 0 255 255 26 197 163 96 20 140 92 0 8 0 0 255 255 245 98 185 42 0 14 0 0]
	 */
	ccpack, err := ccprovider.GetCCPackage(ccbytes)
	if err != nil {
		return err
	}

	cds := ccpack.GetDepSpec()

	if cds == nil {
		return fmt.Errorf("nil deployment spec from from the CC package")
	}

	if err = lscc.isValidChaincodeName(cds.ChaincodeSpec.ChaincodeId.Name); err != nil {
		return err
	}

	if err = lscc.isValidChaincodeVersion(cds.ChaincodeSpec.ChaincodeId.Name, cds.ChaincodeSpec.ChaincodeId.Version); err != nil {
		return err
	}

	if lscc.SCCProvider.IsSysCC(cds.ChaincodeSpec.ChaincodeId.Name) {
		return errors.Errorf("cannot install: %s is the name of a system chaincode", cds.ChaincodeSpec.ChaincodeId.Name)
	}

	// Get any statedb artifacts from the chaincode package, e.g. couchdb index definitions
	statedbArtifactsTar, err := ccprovider.ExtractStatedbArtifactsFromCCPackage(ccpack, lscc.PlatformRegistry)
	if err != nil {
		return err
	}

	if err = isValidStatedbArtifactsTar(statedbArtifactsTar); err != nil {
		return InvalidStatedbArtifactsErr(err.Error())
	}

	chaincodeDefinition := &cceventmgmt.ChaincodeDefinition{
		Name:    ccpack.GetChaincodeData().Name,
		Version: ccpack.GetChaincodeData().Version,
		Hash:    ccpack.GetId()} // Note - The chaincode 'id' is the hash of chaincode's (CodeHash || MetaDataHash), aka fingerprint

	// HandleChaincodeInstall will apply any statedb artifacts (e.g. couchdb indexes) to
	// any channel's statedb where the chaincode is already instantiated
	// Note - this step is done prior to PutChaincodeToLocalStorage() since this step is idempotent and harmless until endorsements start,
	// that is, if there are errors deploying the indexes the chaincode install can safely be re-attempted later.
	err = cceventmgmt.GetMgr().HandleChaincodeInstall(chaincodeDefinition, statedbArtifactsTar)
	defer func() {
		cceventmgmt.GetMgr().ChaincodeInstallDone(err == nil)
	}()
	if err != nil {
		return err
	}

	// Finally, if everything is good above, install the chaincode to local peer file system so that endorsements can start
	if err = lscc.Support.PutChaincodeToLocalStorage(ccpack); err != nil {
		return err
	}

	logger.Infof("Installed Chaincode [%s] Version [%s] to peer", ccpack.GetChaincodeData().Name, ccpack.GetChaincodeData().Version)
	/*
	Installed Chaincode [acb] Version [0] to peer
	*/

	return nil
}

// executeDeployOrUpgrade routes the code path either to executeDeploy or executeUpgrade
// depending on its function argument
func (lscc *LifeCycleSysCC) executeDeployOrUpgrade(stub shim.ChaincodeStubInterface, chainname string, cds *pb.ChaincodeDeploymentSpec, policy, escc, vscc, collectionConfigBytes []byte, function string, ) (*ccprovider.ChaincodeData, error) {
	fmt.Println("===LifeCycleSysCC==executeDeployOrUpgrade==")
	chaincodeName := cds.ChaincodeSpec.ChaincodeId.Name
	chaincodeVersion := cds.ChaincodeSpec.ChaincodeId.Version

	if err := lscc.isValidChaincodeName(chaincodeName); err != nil {
		return nil, err
	}

	if err := lscc.isValidChaincodeVersion(chaincodeName, chaincodeVersion); err != nil {
		return nil, err
	}

	ccpack, err := lscc.Support.GetChaincodeFromLocalStorage(chaincodeName, chaincodeVersion)
	if err != nil {
		retErrMsg := fmt.Sprintf("cannot get package for chaincode (%s:%s)", chaincodeName, chaincodeVersion)
		logger.Errorf("%s-err:%s", retErrMsg, err)
		return nil, fmt.Errorf("%s", retErrMsg)
	}
	cd := ccpack.GetChaincodeData()

	switch function {
	case DEPLOY:
		return lscc.executeDeploy(stub, chainname, cds, policy, escc, vscc, cd, ccpack, collectionConfigBytes)
	case UPGRADE:
		return lscc.executeUpgrade(stub, chainname, cds, policy, escc, vscc, cd, ccpack, collectionConfigBytes)
	default:
		logger.Panicf("Programming error, unexpected function '%s'", function)
		panic("") // unreachable code
	}
}

// executeDeploy implements the "instantiate" Invoke transaction
func (lscc *LifeCycleSysCC) executeDeploy(
	stub shim.ChaincodeStubInterface,
	chainname string,
	cds *pb.ChaincodeDeploymentSpec,
	policy []byte,
	escc []byte,
	vscc []byte,
	cdfs *ccprovider.ChaincodeData,
	ccpackfs ccprovider.CCPackage,
	collectionConfigBytes []byte,
) (*ccprovider.ChaincodeData, error) {
	fmt.Println("===LifeCycleSysCC==executeDeploy==")
	//just test for existence of the chaincode in the LSCC
	chaincodeName := cds.ChaincodeSpec.ChaincodeId.Name
	_, err := lscc.getCCInstance(stub, chaincodeName)
	if err == nil {
		return nil, ExistsErr(chaincodeName)
	}

	//retain chaincode specific data and fill channel specific ones
	cdfs.Escc = string(escc)
	cdfs.Vscc = string(vscc)
	cdfs.Policy = policy

	// retrieve and evaluate instantiation policy
	cdfs.InstantiationPolicy, err = lscc.Support.GetInstantiationPolicy(chainname, ccpackfs)
	if err != nil {
		return nil, err
	}
	// get the signed instantiation proposal
	signedProp, err := stub.GetSignedProposal()
	if err != nil {
		return nil, err
	}
	err = lscc.Support.CheckInstantiationPolicy(signedProp, chainname, cdfs.InstantiationPolicy)
	if err != nil {
		return nil, err
	}

	err = lscc.putChaincodeData(stub, cdfs)
	if err != nil {
		return nil, err
	}

	err = lscc.putChaincodeCollectionData(stub, cdfs, collectionConfigBytes)
	if err != nil {
		return nil, err
	}

	return cdfs, nil
}

// executeUpgrade implements the "upgrade" Invoke transaction.
func (lscc *LifeCycleSysCC) executeUpgrade(stub shim.ChaincodeStubInterface, chainName string, cds *pb.ChaincodeDeploymentSpec, policy []byte, escc []byte, vscc []byte, cdfs *ccprovider.ChaincodeData, ccpackfs ccprovider.CCPackage, collectionConfigBytes []byte) (*ccprovider.ChaincodeData, error) {
	fmt.Println("===LifeCycleSysCC==executeUpgrade==")
	chaincodeName := cds.ChaincodeSpec.ChaincodeId.Name

	// check for existence of chaincode instance only (it has to exist on the channel)
	// we dont care about the old chaincode on the FS. In particular, user may even
	// have deleted it
	cdbytes, _ := lscc.getCCInstance(stub, chaincodeName)
	if cdbytes == nil {
		return nil, NotFoundErr(chaincodeName)
	}

	//we need the cd to compare the version
	cdLedger, err := lscc.getChaincodeData(chaincodeName, cdbytes)
	if err != nil {
		return nil, err
	}

	//do not upgrade if same version
	if cdLedger.Version == cds.ChaincodeSpec.ChaincodeId.Version {
		return nil, IdenticalVersionErr(chaincodeName)
	}

	//do not upgrade if instantiation policy is violated
	if cdLedger.InstantiationPolicy == nil {
		return nil, InstantiationPolicyMissing("")
	}
	// get the signed instantiation proposal
	signedProp, err := stub.GetSignedProposal()
	if err != nil {
		return nil, err
	}
	err = lscc.Support.CheckInstantiationPolicy(signedProp, chainName, cdLedger.InstantiationPolicy)
	if err != nil {
		return nil, err
	}

	//retain chaincode specific data and fill channel specific ones
	cdfs.Escc = string(escc)
	cdfs.Vscc = string(vscc)
	cdfs.Policy = policy

	// retrieve and evaluate new instantiation policy
	cdfs.InstantiationPolicy, err = lscc.Support.GetInstantiationPolicy(chainName, ccpackfs)
	if err != nil {
		return nil, err
	}
	err = lscc.Support.CheckInstantiationPolicy(signedProp, chainName, cdfs.InstantiationPolicy)
	if err != nil {
		return nil, err
	}

	err = lscc.putChaincodeData(stub, cdfs)
	if err != nil {
		return nil, err
	}

	ac, exists := lscc.SCCProvider.GetApplicationConfig(chainName)
	if !exists {
		logger.Panicf("programming error, non-existent appplication config for channel '%s'", chainName)
	}

	if ac.Capabilities().CollectionUpgrade() {
		err = lscc.putChaincodeCollectionData(stub, cdfs, collectionConfigBytes)
		if err != nil {
			return nil, err
		}
	} else {
		if collectionConfigBytes != nil {
			return nil, errors.New(CollectionsConfigUpgradesNotAllowed("").Error())
		}
	}

	lifecycleEvent := &pb.LifecycleEvent{ChaincodeName: chaincodeName}
	lifecycleEventBytes := utils.MarshalOrPanic(lifecycleEvent)
	stub.SetEvent(UPGRADE, lifecycleEventBytes)
	return cdfs, nil
}

//-------------- the chaincode stub interface implementation ----------

//Init is mostly useless for SCC
func (lscc *LifeCycleSysCC) Init(stub shim.ChaincodeStubInterface) pb.Response {
	fmt.Println("===LifeCycleSysCC==Init==")
	return shim.Success(nil)
}

// Invoke implements lifecycle functions "deploy", "start", "stop", "upgrade".
// Deploy's arguments -  {[]byte("deploy"), []byte(<chainname>), <unmarshalled pb.ChaincodeDeploymentSpec>}
//
// Invoke also implements some query-like functions
// Get chaincode arguments -  {[]byte("getid"), []byte(<chainname>), []byte(<chaincodename>)}
func (lscc *LifeCycleSysCC) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	fmt.Println("===LifeCycleSysCC==Invoke==")
	args := stub.GetArgs()
	if len(args) < 1 {
		return shim.Error(InvalidArgsLenErr(len(args)).Error())
	}

	function := string(args[0])
	fmt.Println("===========function=================",function)
	//deploy
    //===========function=================
	// Handle ACL:
	// 1. get the signed proposal
	sp, err := stub.GetSignedProposal()
	if err != nil {
		return shim.Error(fmt.Sprintf("Failed retrieving signed proposal on executing %s with error %s", function, err))
	}

	switch function {
	case INSTALL:
		fmt.Println("==========INSTALL===================")
		if len(args) < 2 {
			return shim.Error(InvalidArgsLenErr(len(args)).Error())
		}

		// 2. check local MSP Admins policy
		if err = lscc.PolicyChecker.CheckPolicyNoChannel(mgmt.Admins, sp); err != nil {
			return shim.Error(fmt.Sprintf("access denied for [%s]: %s", function, err))
		}

		depSpec := args[1]
		err := lscc.executeInstall(stub, depSpec)
		if err != nil {
			return shim.Error(err.Error())
		}
		return shim.Success([]byte("OK"))
	case DEPLOY, UPGRADE:
		fmt.Println("==========DEPLOY, UPGRADE==================")
		// we expect a minimum of 3 arguments, the function
		// name, the chain name and deployment spec
		if len(args) < 3 {
			return shim.Error(InvalidArgsLenErr(len(args)).Error())
		}

		// channel the chaincode should be associated with. It
		// should be created with a register call
		channel := string(args[1])

		if !lscc.isValidChannelName(channel) {
			return shim.Error(InvalidChannelNameErr(channel).Error())
		}

		ac, exists := lscc.SCCProvider.GetApplicationConfig(channel)
		if !exists {
			logger.Panicf("programming error, non-existent appplication config for channel '%s'", channel)
		}

		// the maximum number of arguments depends on the capability of the channel
		if !ac.Capabilities().PrivateChannelData() && len(args) > 6 {
			return shim.Error(PrivateChannelDataNotAvailable("").Error())
		}
		if ac.Capabilities().PrivateChannelData() && len(args) > 7 {
			return shim.Error(InvalidArgsLenErr(len(args)).Error())
		}

		depSpec := args[2]
		cds := &pb.ChaincodeDeploymentSpec{}
		err := proto.Unmarshal(depSpec, cds)
		if err != nil {
			return shim.Error(fmt.Sprintf("error unmarshaling ChaincodeDeploymentSpec: %s", err))
		}

		// optional arguments here (they can each be nil and may or may not be present)
		// args[3] is a marshalled SignaturePolicyEnvelope representing the endorsement policy
		// args[4] is the name of escc
		// args[5] is the name of vscc
		// args[6] is a marshalled CollectionConfigPackage struct
		var EP []byte
		if len(args) > 3 && len(args[3]) > 0 {
			EP = args[3]
		} else {
			p := cauthdsl.SignedByAnyMember(peer.GetMSPIDs(channel))
			EP, err = utils.Marshal(p)
			if err != nil {
				return shim.Error(err.Error())
			}
		}

		var escc []byte
		if len(args) > 4 && len(args[4]) > 0 {
			escc = args[4]
		} else {
			escc = []byte("escc")
		}

		var vscc []byte
		if len(args) > 5 && len(args[5]) > 0 {
			vscc = args[5]
		} else {
			vscc = []byte("vscc")
		}

		var collectionsConfig []byte
		// we proceed with a non-nil collection configuration only if
		// we Support the PrivateChannelData capability
		if ac.Capabilities().PrivateChannelData() && len(args) > 6 {
			collectionsConfig = args[6]
		}

		cd, err := lscc.executeDeployOrUpgrade(stub, channel, cds, EP, escc, vscc, collectionsConfig, function)
		if err != nil {
			return shim.Error(err.Error())
		}
		cdbytes, err := proto.Marshal(cd)
		if err != nil {
			return shim.Error(err.Error())
		}
		return shim.Success(cdbytes)
	case CCEXISTS, CHAINCODEEXISTS, GETDEPSPEC, GETDEPLOYMENTSPEC, GETCCDATA, GETCHAINCODEDATA:
		fmt.Println("==========CCEXISTS, CHAINCODEEXISTS, GETDEPSPEC, GETDEPLOYMENTSPEC, GETCCDATA, GETCHAINCODEDATA==================")
		if len(args) != 3 {
			return shim.Error(InvalidArgsLenErr(len(args)).Error())
		}

		channel := string(args[1])
		ccname := string(args[2])

		// 2. check policy for ACL resource
		var resource string
		switch function {
		case CCEXISTS, CHAINCODEEXISTS:
			resource = resources.Lscc_ChaincodeExists
		case GETDEPSPEC, GETDEPLOYMENTSPEC:
			resource = resources.Lscc_GetDeploymentSpec
		case GETCCDATA, GETCHAINCODEDATA:
			resource = resources.Lscc_GetChaincodeData
		}
		if err = lscc.ACLProvider.CheckACL(resource, channel, sp); err != nil {
			return shim.Error(fmt.Sprintf("access denied for [%s][%s]: %s", function, channel, err))
		}

		cdbytes, err := lscc.getCCInstance(stub, ccname)
		if err != nil {
			logger.Errorf("error getting chaincode %s on channel [%s]: %s", ccname, channel, err)
			return shim.Error(err.Error())
		}

		switch function {
		case CCEXISTS, CHAINCODEEXISTS:
			fmt.Println("==========CCEXISTS, CHAINCODEEXISTS==================")
			cd, err := lscc.getChaincodeData(ccname, cdbytes)
			if err != nil {
				return shim.Error(err.Error())
			}
			return shim.Success([]byte(cd.Name))
		case GETCCDATA, GETCHAINCODEDATA:
			fmt.Println("==========GETCCDATA, GETCHAINCODEDATA==================")
			return shim.Success(cdbytes)
		case GETDEPSPEC, GETDEPLOYMENTSPEC:
			fmt.Println("==========GETDEPSPEC, GETDEPLOYMENTSPEC==================")
			_, depspecbytes, err := lscc.getCCCode(ccname, cdbytes)
			if err != nil {
				return shim.Error(err.Error())
			}
			return shim.Success(depspecbytes)
		default:
			panic("unreachable")
		}
	case GETCHAINCODES, GETCHAINCODESALIAS:
		if len(args) != 1 {
			return shim.Error(InvalidArgsLenErr(len(args)).Error())
		}

		if err = lscc.ACLProvider.CheckACL(resources.Lscc_GetInstantiatedChaincodes, stub.GetChannelID(), sp); err != nil {
			return shim.Error(fmt.Sprintf("access denied for [%s][%s]: %s", function, stub.GetChannelID(), err))
		}

		return lscc.getChaincodes(stub)
	case GETINSTALLEDCHAINCODES, GETINSTALLEDCHAINCODESALIAS:
		if len(args) != 1 {
			return shim.Error(InvalidArgsLenErr(len(args)).Error())
		}

		// 2. check local MSP Admins policy
		if err = lscc.PolicyChecker.CheckPolicyNoChannel(mgmt.Admins, sp); err != nil {
			return shim.Error(fmt.Sprintf("access denied for [%s]: %s", function, err))
		}

		return lscc.getInstalledChaincodes()
	case GETCOLLECTIONSCONFIG, GETCOLLECTIONSCONFIGALIAS:
		if len(args) != 2 {
			return shim.Error(InvalidArgsLenErr(len(args)).Error())
		}

		chaincodeName := string(args[1])

		logger.Debugf("GetCollectionsConfig, chaincodeName:%s, start to check ACL for current identity policy", chaincodeName)
		if err = lscc.ACLProvider.CheckACL(resources.Lscc_GetCollectionsConfig, stub.GetChannelID(), sp); err != nil {
			logger.Debugf("ACL Check Failed for channel:%s, chaincode:%s", stub.GetChannelID(), chaincodeName)
			return shim.Error(fmt.Sprintf("access denied for [%s]: %s", function, err))
		}

		return lscc.getChaincodeCollectionData(stub, chaincodeName)
	}

	return shim.Error(InvalidFunctionErr(function).Error())
}
