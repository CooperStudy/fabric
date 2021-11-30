/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package chaincode

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"strings"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/cauthdsl"
	"github.com/hyperledger/fabric/common/localmsp"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/core/chaincode"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/core/container"
	"github.com/hyperledger/fabric/msp"
	ccapi "github.com/hyperledger/fabric/peer/chaincode/api"
	"github.com/hyperledger/fabric/peer/common"
	"github.com/hyperledger/fabric/peer/common/api"
	pcommon "github.com/hyperledger/fabric/protos/common"
	ab "github.com/hyperledger/fabric/protos/orderer"
	pb "github.com/hyperledger/fabric/protos/peer"
	putils "github.com/hyperledger/fabric/protos/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// checkSpec to see if chaincode resides within current package capture for language.
func checkSpec(spec *pb.ChaincodeSpec) error {
	fmt.Println("=============checkSpec==================")
	// Don't allow nil value
	if spec == nil {
		return errors.New("expected chaincode specification, nil received")
	}

	return platformRegistry.ValidateSpec(spec.CCType(), spec.Path())
}

// getChaincodeDeploymentSpec get chaincode deployment spec given the chaincode spec
func getChaincodeDeploymentSpec(spec *pb.ChaincodeSpec, crtPkg bool) (*pb.ChaincodeDeploymentSpec, error) {
	logger.Info("=========func getChaincodeDeploymentSpec(spec *pb.ChaincodeSpec, crtPkg bool) (*pb.ChaincodeDeploymentSpec, error)=========")
	var codePackageBytes []byte
	logger.Info("======chaincode.IsDevMode() ===========",chaincode.IsDevMode())
	logger.Info("======crtPkg ===========",crtPkg)
	if chaincode.IsDevMode() == false && crtPkg {
		var err error
		if err = checkSpec(spec); err != nil {
			return nil, err
		}

		codePackageBytes, err = container.GetChaincodePackageBytes(platformRegistry, spec)
		logger.Info("=========codePackageBytes=============",codePackageBytes)
		if err != nil {
			err = errors.WithMessage(err, "error getting chaincode package bytes")
			return nil, err
		}
	}
	chaincodeDeploymentSpec := &pb.ChaincodeDeploymentSpec{ChaincodeSpec: spec, CodePackage: codePackageBytes}
	return chaincodeDeploymentSpec, nil
}

// getChaincodeSpec get chaincode spec from the cli cmd pramameters
func getChaincodeSpec(cmd *cobra.Command) (*pb.ChaincodeSpec, error) {
	logger.Info("============func getChaincodeSpec(cmd *cobra.Command) (*pb.ChaincodeSpec, error)============")
	spec := &pb.ChaincodeSpec{}
	if err := checkChaincodeCmdParams(cmd); err != nil {
		// unset usage silence because it's a command line usage error
		cmd.SilenceUsage = false
		return spec, err
	}

	// Build the spec
	input := &pb.ChaincodeInput{}
	logger.Info("============chaincodeCtorJSON============",chaincodeCtorJSON)
	err := json.Unmarshal([]byte(chaincodeCtorJSON), &input)
	logger.Info("============input============",input)
	if err != nil {
		return spec, errors.Wrap(err, "chaincode argument error")
	}

	chaincodeLang = strings.ToUpper(chaincodeLang)
	spec = &pb.ChaincodeSpec{
		Type:        pb.ChaincodeSpec_Type(pb.ChaincodeSpec_Type_value[chaincodeLang]),
		ChaincodeId: &pb.ChaincodeID{Path: chaincodePath, Name: chaincodeName, Version: chaincodeVersion},
		Input:       input,
	}
	return spec, nil
}

func chaincodeInvokeOrQuery(cmd *cobra.Command, invoke bool, cf *ChaincodeCmdFactory) (err error) {
	fmt.Println("=============chaincodeInvokeOrQuery==================")
	spec, err := getChaincodeSpec(cmd)
	if err != nil {
		return err
	}

	// call with empty txid to ensure production code generates a txid.
	// otherwise, tests can explicitly set their own txid
	txID := ""

	proposalResp, err := ChaincodeInvokeOrQuery(
		spec,
		channelID,
		txID,
		invoke,
		cf.Signer,
		cf.Certificate,
		cf.EndorserClients,
		cf.DeliverClients,
		cf.BroadcastClient)

	if err != nil {
		return errors.Errorf("%s - proposal response: %v", err, proposalResp)
	}

	if invoke {
		logger.Debugf("ESCC invoke result: %v", proposalResp)
		pRespPayload, err := putils.GetProposalResponsePayload(proposalResp.Payload)
		if err != nil {
			return errors.WithMessage(err, "error while unmarshaling proposal response payload")
		}
		ca, err := putils.GetChaincodeAction(pRespPayload.Extension)
		if err != nil {
			return errors.WithMessage(err, "error while unmarshaling chaincode action")
		}
		if proposalResp.Endorsement == nil {
			return errors.Errorf("endorsement failure during invoke. response: %v", proposalResp.Response)
		}
		logger.Infof("Chaincode invoke successful. result: %v", ca.Response)
	} else {
		if proposalResp == nil {
			return errors.New("error during query: received nil proposal response")
		}
		if proposalResp.Endorsement == nil {
			return errors.Errorf("endorsement failure during query. response: %v", proposalResp.Response)
		}

		if chaincodeQueryRaw && chaincodeQueryHex {
			return fmt.Errorf("options --raw (-r) and --hex (-x) are not compatible")
		}
		if chaincodeQueryRaw {
			fmt.Println(proposalResp.Response.Payload)
			return nil
		}
		if chaincodeQueryHex {
			fmt.Printf("%x\n", proposalResp.Response.Payload)
			return nil
		}
		fmt.Println(string(proposalResp.Response.Payload))
	}
	return nil
}

type collectionConfigJson struct {
	Name           string `json:"name"`
	Policy         string `json:"policy"`
	RequiredCount  int32  `json:"requiredPeerCount"`
	MaxPeerCount   int32  `json:"maxPeerCount"`
	BlockToLive    uint64 `json:"blockToLive"`
	MemberOnlyRead bool   `json:"memberOnlyRead"`
}

// getCollectionConfig retrieves the collection configuration
// from the supplied file; the supplied file must contain a
// json-formatted array of collectionConfigJson elements
func getCollectionConfigFromFile(ccFile string) ([]byte, error) {
	logger.Info("============func getCollectionConfigFromFile(ccFile string) ([]byte, error)=================")
	logger.Info("===========ccFile=========",ccFile)
	fileBytes, err := ioutil.ReadFile(ccFile)
	if err != nil {
		return nil, errors.Wrapf(err, "could not read file '%s'", ccFile)
	}

	return getCollectionConfigFromBytes(fileBytes)
}

// getCollectionConfig retrieves the collection configuration
// from the supplied byte array; the byte array must contain a
// json-formatted array of collectionConfigJson elements
func getCollectionConfigFromBytes(cconfBytes []byte) ([]byte, error) {
	logger.Info("====func getCollectionConfigFromBytes(cconfBytes []byte) ([]byte, error)======")
	cconf := &[]collectionConfigJson{}
	err := json.Unmarshal(cconfBytes, cconf)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse the collection configuration")
	}
	logger.Info("===============cconf := &[]collectionConfigJson{}==================")
	for k,v := range *cconf{
		logger.Info("==========k",k)
		logger.Info("====name",v.Name)
		logger.Info("====BlockToLive",v.BlockToLive)
		logger.Info("====MaxPeerCount",v.MaxPeerCount)
		logger.Info("====MemberOnlyRead",v.MemberOnlyRead)
		logger.Info("====Policy",v.Policy)
		logger.Info("====RequiredCount",v.RequiredCount)
	}

	ccarray := make([]*pcommon.CollectionConfig, 0, len(*cconf))
	for _, cconfitem := range *cconf {
		logger.Info("=====cconfitem====",cconfitem)
		p, err := cauthdsl.FromString(cconfitem.Policy)
		logger.Infof("========%v, %v := cauthdsl.FromString(%v)===================",p,err,cconfitem.Policy)
		if err != nil {
			return nil, errors.WithMessage(err, fmt.Sprintf("invalid policy %s", cconfitem.Policy))
		}

		cpc := &pcommon.CollectionPolicyConfig{
			Payload: &pcommon.CollectionPolicyConfig_SignaturePolicy{
				SignaturePolicy: p,
			},
		}

		cc := &pcommon.CollectionConfig{
			Payload: &pcommon.CollectionConfig_StaticCollectionConfig{
				StaticCollectionConfig: &pcommon.StaticCollectionConfig{
					Name:              cconfitem.Name,
					MemberOrgsPolicy:  cpc,
					RequiredPeerCount: cconfitem.RequiredCount,
					MaximumPeerCount:  cconfitem.MaxPeerCount,
					BlockToLive:       cconfitem.BlockToLive,
					MemberOnlyRead:    cconfitem.MemberOnlyRead,
				},
			},
		}

		ccarray = append(ccarray, cc)
	}

	ccp := &pcommon.CollectionConfigPackage{Config: ccarray}
	logger.Info("===========ccp := &pcommon.CollectionConfigPackage{Config: ccarray}===============")
	return proto.Marshal(ccp)
}

func checkChaincodeCmdParams(cmd *cobra.Command) error {
	logger.Info("==========func checkChaincodeCmdParams(cmd *cobra.Command) error=================")
	// we need chaincode name for everything, including deploy
	if chaincodeName == common.UndefinedParamValue {
		return errors.Errorf("must supply value for %s name parameter", chainFuncName)
	}

	if cmd.Name() == instantiateCmdName || cmd.Name() == installCmdName ||
		cmd.Name() == upgradeCmdName || cmd.Name() == packageCmdName {
		if chaincodeVersion == common.UndefinedParamValue {
			return errors.Errorf("chaincode version is not provided for %s", cmd.Name())
		}

		if escc != common.UndefinedParamValue {
			logger.Infof("Using escc %s", escc)
		} else {
			logger.Info("Using default escc")
			escc = "escc"
		}

		if vscc != common.UndefinedParamValue {
			logger.Infof("Using vscc %s", vscc)
		} else {
			logger.Info("Using default vscc")
			vscc = "vscc"
		}

		if policy != common.UndefinedParamValue {
			p, err := cauthdsl.FromString(policy)
			if err != nil {
				return errors.Errorf("invalid policy %s", policy)
			}
			policyMarshalled = putils.MarshalOrPanic(p)
		}

		if collectionsConfigFile != common.UndefinedParamValue {
			var err error
			logger.Info("=======collectionsConfigFile=====",collectionsConfigFile)
			collectionConfigBytes, err = getCollectionConfigFromFile(collectionsConfigFile)
			if err != nil {
				return errors.WithMessage(err, fmt.Sprintf("invalid collection configuration in file %s", collectionsConfigFile))
			}
		}
	}

	// Check that non-empty chaincode parameters contain only Args as a key.
	// Type checking is done later when the JSON is actually unmarshaled
	// into a pb.ChaincodeInput. To better understand what's going
	// on here with JSON parsing see http://blog.golang.org/json-and-go -
	// Generic JSON with interface{}
	if chaincodeCtorJSON != "{}" {
		var f interface{}
		err := json.Unmarshal([]byte(chaincodeCtorJSON), &f)
		if err != nil {
			return errors.Wrap(err, "chaincode argument error")
		}
		m := f.(map[string]interface{})
		sm := make(map[string]interface{})
		for k := range m {
			sm[strings.ToLower(k)] = m[k]
		}
		_, argsPresent := sm["args"]
		_, funcPresent := sm["function"]
		if !argsPresent || (len(m) == 2 && !funcPresent) || len(m) > 2 {
			return errors.New("non-empty JSON chaincode parameters must contain the following keys: 'Args' or 'Function' and 'Args'")
		}
	} else {
		if cmd == nil || (cmd != chaincodeInstallCmd && cmd != chaincodePackageCmd) {
			return errors.New("empty JSON chaincode parameters must contain the following keys: 'Args' or 'Function' and 'Args'")
		}
	}

	return nil
}

func validatePeerConnectionParameters(cmdName string) error {
	fmt.Println("============func validatePeerConnectionParameters(cmdName string) error ==================")
	fmt.Println("==========connectionProfile====================",connectionProfile)//""
	if connectionProfile != common.UndefinedParamValue {
		fmt.Println("=============connectionProfile != common.UndefinedParamValue ========================")
		networkConfig, err := common.GetConfig(connectionProfile)
		if err != nil {
			return err
		}
		fmt.Println("=======================networkConfig.Channels[channelID].Peers========================",networkConfig.Channels[channelID].Peers)
		if len(networkConfig.Channels[channelID].Peers) != 0 {
			fmt.Println("==============if len(networkConfig.Channels[channelID].Peers) != 0 ==============")
			peerAddresses = []string{}
			tlsRootCertFiles = []string{}
			for peer, peerChannelConfig := range networkConfig.Channels[channelID].Peers {
				if peerChannelConfig.EndorsingPeer {
					peerConfig, ok := networkConfig.Peers[peer]
					if !ok {
						return errors.Errorf("peer '%s' is defined in the channel config but doesn't have associated peer config", peer)
					}
					fmt.Println("===================== peerConfig.URL============", peerConfig.URL)
					peerAddresses = append(peerAddresses, peerConfig.URL)
					fmt.Println("=======================tlsRootCertFiles==========",peerConfig.TLSCACerts.Path)
					tlsRootCertFiles = append(tlsRootCertFiles, peerConfig.TLSCACerts.Path)
				}
			}
		}
	}

	// currently only support multiple peer addresses for invoke
	if cmdName != "invoke" && len(peerAddresses) > 1 {
		return errors.Errorf("'%s' command can only be executed against one peer. received %d", cmdName, len(peerAddresses))
	}

	if len(tlsRootCertFiles) > len(peerAddresses) {
		logger.Warningf("received more TLS root cert files (%d) than peer addresses (%d)", len(tlsRootCertFiles), len(peerAddresses))
	}

	if viper.GetBool("peer.tls.enabled") {
		if len(tlsRootCertFiles) != len(peerAddresses) {
			return errors.Errorf("number of peer addresses (%d) does not match the number of TLS root cert files (%d)", len(peerAddresses), len(tlsRootCertFiles))
		}
	} else {
		tlsRootCertFiles = nil
	}

	return nil
}

// ChaincodeCmdFactory holds the clients used by ChaincodeCmd
type ChaincodeCmdFactory struct {
	EndorserClients []pb.EndorserClient
	DeliverClients  []api.PeerDeliverClient
	Certificate     tls.Certificate
	Signer          msp.SigningIdentity
	BroadcastClient common.BroadcastClient
}

// InitCmdFactory init the ChaincodeCmdFactory with default clients
func InitCmdFactory(cmdName string, isEndorserRequired, isOrdererRequired bool) (*ChaincodeCmdFactory, error) {
	logger.Info("====func InitCmdFactory(cmdName string, isEndorserRequired, isOrdererRequired bool) (*ChaincodeCmdFactory, error)====")
	var err error
	var endorserClients []pb.EndorserClient
	var deliverClients []api.PeerDeliverClient
	logger.Info("====isEndorserRequired====",isEndorserRequired)//true
	logger.Info("====isOrdererRequired====",isOrdererRequired)//true
	if isEndorserRequired {
		logger.Infof("====validatePeerConnectionParameters(%v)====",cmdName)
		 err = validatePeerConnectionParameters(cmdName)

		if err != nil {
			return nil, errors.WithMessage(err, "error validating peer connection parameters")
		}
		logger.Infof("====peerAddresses:%v====",peerAddresses)
		/*
		1.initiate
		peerAddresses []

		2.invoke
		 [peer0.org1.example.com:7051 peer0.org2.example.com:7051]
		*/
		for i, address := range peerAddresses {
			logger.Info("====i",i)
			//0
			//1
			logger.Info("====address",address)
			//peer0.org1.example.com:7051
			//peer0.org2.example.com:7051
			var tlsRootCertFile string
			fmt.Println("====tlsRootCertFiles",tlsRootCertFiles)
			/*
			[/opt/gopath/src/github.com/hyperledger/fabric/peer/crypto/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt
			 /opt/gopath/src/github.com/hyperledger/fabric/peer/crypto/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt]
			 */
			//
			if tlsRootCertFiles != nil {
				tlsRootCertFile = tlsRootCertFiles[i]
				fmt.Println("==============================tlsRootCertFile",tlsRootCertFile)
				// /opt/gopath/src/github.com/hyperledger/fabric/peer/crypto/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt
				// /opt/gopath/src/github.com/hyperledger/fabric/peer/crypto/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt
			}
			endorserClient, err := common.GetEndorserClientFnc(address, tlsRootCertFile)
			fmt.Println("==================endorserClient========================",endorserClient)//&{0xc0000d2000}

			if err != nil {
				return nil, errors.WithMessage(err, fmt.Sprintf("error getting endorser client for %s", cmdName))
			}
			endorserClients = append(endorserClients, endorserClient)
			fmt.Println("==================endorserClients========================",endorserClients)//[0xc00014e038]
			deliverClient, err := common.GetPeerDeliverClientFnc(address, tlsRootCertFile)
			fmt.Println("==================deliverClient========================",deliverClient)
			if err != nil {
				return nil, errors.WithMessage(err, fmt.Sprintf("error getting deliver client for %s", cmdName))
			}
			deliverClients = append(deliverClients, deliverClient)
			fmt.Println("==================deliverClients========================",deliverClients)//[0xc00013c690 0xc000267ad0]
		}

		logger.Infof("====len(endorserClients):%v====",len(endorserClients))
		if len(endorserClients) == 0 {
			return nil, errors.New("no endorser clients retrieved - this might indicate a bug")
		}
	}
	certificate, err := common.GetCertificateFnc()
	if err != nil {
		return nil, errors.WithMessage(err, "error getting client cerificate")
	}

	signer, err := common.GetDefaultSignerFnc()
	if err != nil {
		return nil, errors.WithMessage(err, "error getting default signer")
	}
	var broadcastClient common.BroadcastClient

	if isOrdererRequired {
		//isOrdererRequired true
		logger.Infof("====len(common.OrderingEndpoint):%v====",len(common.OrderingEndpoint))//true
		logger.Infof("====common.OrderingEndpoint:%v====",common.OrderingEndpoint)//true
		if len(common.OrderingEndpoint) == 0 {
			if len(endorserClients) == 0 {
				return nil, errors.New("orderer is required, but no ordering endpoint or endorser client supplied")
			}
			endorserClient := endorserClients[0]

			orderingEndpoints, err := common.GetOrdererEndpointOfChainFnc(channelID, signer, endorserClient)
			if err != nil {
				return nil, errors.WithMessage(err, fmt.Sprintf("error getting channel (%s) orderer endpoint", channelID))
			}
			if len(orderingEndpoints) == 0 {
				return nil, errors.Errorf("no orderer endpoints retrieved for channel %s", channelID)
			}
			logger.Infof("Retrieved channel (%s) orderer endpoint: %s", channelID, orderingEndpoints[0])
			// override viper env
			viper.Set("orderer.address", orderingEndpoints[0])
		}
		broadcastClient, err = common.GetBroadcastClientFnc()

		if err != nil {
			return nil, errors.WithMessage(err, "error getting broadcast client")
		}
	}
	return &ChaincodeCmdFactory{
		EndorserClients: endorserClients,
		DeliverClients:  deliverClients,
		Signer:          signer,
		BroadcastClient: broadcastClient,
		Certificate:     certificate,
	}, nil
}

// ChaincodeInvokeOrQuery invokes or queries the chaincode. If successful, the
// INVOKE form prints the ProposalResponse to STDOUT, and the QUERY form prints
// the query result on STDOUT. A command-line flag (-r, --raw) determines
// whether the query result is output as raw bytes, or as a printable string.
// The printable form is optionally (-x, --hex) a hexadecimal representation
// of the query response. If the query response is NIL, nothing is output.
//
// NOTE - Query will likely go away as all interactions with the endorser are
// Proposal and ProposalResponses
func ChaincodeInvokeOrQuery(
	spec *pb.ChaincodeSpec,
	cID string,
	txID string,
	invoke bool,
	signer msp.SigningIdentity,
	certificate tls.Certificate,
	endorserClients []pb.EndorserClient,
	deliverClients []api.PeerDeliverClient,
	bc common.BroadcastClient,
) (*pb.ProposalResponse, error) {
	fmt.Println("=============ChaincodeInvokeOrQuery==================")
	// Build the ChaincodeInvocationSpec message

	invocation := &pb.ChaincodeInvocationSpec{ChaincodeSpec: spec}

	creator, err := signer.Serialize()
	if err != nil {
		return nil, errors.WithMessage(err, fmt.Sprintf("error serializing identity for %s", signer.GetIdentifier()))
	}

	funcName := "invoke"
	if !invoke {
		funcName = "query"
	}

	// extract the transient field if it exists
	var tMap map[string][]byte
	if transient != "" {
		if err := json.Unmarshal([]byte(transient), &tMap); err != nil {
			return nil, errors.Wrap(err, "error parsing transient string")
		}
	}

	prop, txid, err := putils.CreateChaincodeProposalWithTxIDAndTransient(pcommon.HeaderType_ENDORSER_TRANSACTION, cID, invocation, creator, txID, tMap)
	if err != nil {
		return nil, errors.WithMessage(err, fmt.Sprintf("error creating proposal for %s", funcName))
	}

	signedProp, err := putils.GetSignedProposal(prop, signer)
	if err != nil {
		return nil, errors.WithMessage(err, fmt.Sprintf("error creating signed proposal for %s", funcName))
	}
	var responses []*pb.ProposalResponse
	for k, endorser := range endorserClients {
		fmt.Println("================k======================",k)//0
		//1
		fmt.Println("================endorser======================",endorser)//&{0xc0000d2000}
		fmt.Println("====================signedProp==============================",signedProp)
		//proposal_bytes:"\n\303\007\ng\010\003\032\014\010\200\352\361\214\006\020\276\347\376\207\002\"\tmychannel*@38ec6dc6617d415b856aa186fd67959c6273ed9529f38a88e83387dc6007cd44:\010\022\006\022\004mycc\022\327\006\n\272\006\n\007Org1MSP\022\256\006-----BEGIN CERTIFICATE-----\nMIICKzCCAdGgAwIBAgIRAJH5pT6oad0DwaoVpYvsnRYwCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTIzMDQ1MzAwWhcNMzExMTIxMDQ1MzAw\nWjBsMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEPMA0GA1UECxMGY2xpZW50MR8wHQYDVQQDDBZBZG1pbkBv\ncmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEjkNw8fd0\nUNWAxVB9ge/V7yX7fuBWB7PLfcs3ir421JImYSDW0BwYxQMNNtaJzbrVh31Rp5fD\nsGg4CIs8RjR25aNNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYD\nVR0jBCQwIoAgwYCJKZTdr1gamBudkhq1imTZeKfgEKqV/J1R6+ojSpwwCgYIKoZI\nzj0EAwIDSAAwRQIhAIRBayA7nrL35q2p0IlcwuhQlgKQDVFySzrJSUtoHwQcAiA/\nr9pz6m5TGsbebgKgl3HqadC50EmPB18HHHCEtrs3dA==\n-----END CERTIFICATE-----\n\022\030j\377\250\006%%\3138\307\251\014\275\033\331\353FgD&w\020K\022z\022\"\n \n\036\010\001\022\006\022\004mycc\032\022\n\006invoke\n\001a\n\001b\n\00210" signature:"0E\002!\000\334\032K\332%\021nXnb\313\351gW\275\246)\026\034\010\"ot\"\3613j \241\342\0348\002 P\335\261(w\265K\255\325\350t\273\262\247\220\265K\220\367\257\013S\204\000\323\202\376\034\277R:\217"
		//proposal_bytes:"\n\303\007\ng\010\003\032\014\010\203\352\361\214\006\020\265\246\304\324\003\"\tmychannel*@2b68f779a01681b47885d1f4a4fe51c731653ba1a7f2b9230cd87f880354ea85:\010\022\006\022\004mycc\022\327\006\n\272\006\n\007Org2MSP\022\256\006-----BEGIN CERTIFICATE-----\nMIICKzCCAdGgAwIBAgIRAPwS3GA4sA8r4Hqq6/4/GeswCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzIuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzIuZXhhbXBsZS5jb20wHhcNMjExMTIzMDQ1MzAwWhcNMzExMTIxMDQ1MzAw\nWjBsMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEPMA0GA1UECxMGY2xpZW50MR8wHQYDVQQDDBZBZG1pbkBv\ncmcyLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEzjlPS5pg\nrDY4tthuQtUuXVt/N2BooFJpWmGmXAZGv0EtMqzicD93phVjeaPWWETQtYMUi1BD\n0dHxC3hbZ1SCbqNNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYD\nVR0jBCQwIoAg2LmIkR1Yf1jrjdib6ItgOUE2vQONqeem9DrA1sP+494wCgYIKoZI\nzj0EAwIDSAAwRQIhANdAeQCEJ2KA3h1VmL/GYgwS/KlaeN92pZQ9Fi9YHdV2AiA5\nGPh/sDkJc1IBZc7yolZzyoS1fMIkXwFYj0/l/NtGPg==\n-----END CERTIFICATE-----\n\022\030q\271\216\334\317aw\367\232|V\007\rN*R\244&)\343\371D\365\253\022\032\n\030\n\026\010\001\022\006\022\004mycc\032\n\n\005query\n\001a" signature:"0D\002 z\306\247_\352\014\240\222\032\274\021\024a\341\335m\275\337\255\016k\007\246\223\316^\007L\366\031\302\354\002 X\362\0018\005\325\250Z\350|\226(\r\257\0063\367\275\024\307/\014\325\023\311\014\357\021#>D,"
		proposalResp, err := endorser.ProcessProposal(context.Background(), signedProp)
		fmt.Println("===============proposalResp============================",proposalResp)
		/*
		version:1 response:<status:200 > payload:"\n q\036\365ac\233\364\2769\247\\\026\014\371\003\374\342\377E\320\260\341\365\250\311&`\202\032\245\310\007\022Y\nE\022\024\n\004lscc\022\014\n\n\n\004mycc\022\002\010\003\022-\n\004mycc\022%\n\007\n\001a\022\002\010\003\n\007\n\001b\022\002\010\003\032\007\n\001a\032\00290\032\010\n\001b\032\003210\032\003\010\310\001\"\013\022\004mycc\032\0031.0" endorsement:<endorser:"\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKDCCAc6gAwIBAgIQai9GfSR87xVbQ7+iETmmgjAKBggqhkjOPQQDAjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEcMBoGA1UEAxMTY2Eu\nb3JnMS5leGFtcGxlLmNvbTAeFw0yMTExMjMwNDUzMDBaFw0zMTExMjEwNDUzMDBa\nMGoxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMQ0wCwYDVQQLEwRwZWVyMR8wHQYDVQQDExZwZWVyMC5vcmcx\nLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEkPw4F4lx8qnd\n57OOCU9xTPpmQGXl6REwFrWY45WWFFgQHzHixWSnkjNNrjJ8Oq2X1KrsMT2xzLkv\njMiP9ArnIKNNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYDVR0j\nBCQwIoAgwYCJKZTdr1gamBudkhq1imTZeKfgEKqV/J1R6+ojSpwwCgYIKoZIzj0E\nAwIDSAAwRQIhAJ13cTfnCzUqeeYSnGvzJpt2Q6nN5gIsiU1pOs09oVChAiA5Maru\nXLPk9dKIa9xdD9X8HWn2is9JOesbCk+XiWJjpA==\n-----END CERTIFICATE-----\n" signature:"0D\002 \017\231\316\330\347\374\372\213Z\034\344\214\273M\246\002\275Q\225\364\226\262\370\005m\031%\005\253u\216\267\002 H\254~\354).I\\\3255\2455\212\254\321\013\030\241\343\300\216\333C\320\360\330\357\365\231\323\031\271" >
		version:1 response:<status:200 payload:"90" > payload:"\n dT\312\355RT\306@\202\223\301\030NINk\217\301\034\332(\326\304\016\207\370\004\307H\312\353\036\022A\n)\022\024\n\004lscc\022\014\n\n\n\004mycc\022\002\010\003\022\021\n\004mycc\022\t\n\007\n\001a\022\002\010\004\032\007\010\310\001\032\00290\"\013\022\004mycc\032\0031.0" endorsement:<endorser:"\n\007Org2MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAc+gAwIBAgIRAMM1T1QcvJxfWBgIn/lmubEwCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzIuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzIuZXhhbXBsZS5jb20wHhcNMjExMTIzMDQ1MzAwWhcNMzExMTIxMDQ1MzAw\nWjBqMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzENMAsGA1UECxMEcGVlcjEfMB0GA1UEAxMWcGVlcjEub3Jn\nMi5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABL/fGOhHLYjL\n/v0eZnwkGjCiEJ1GZ9HSxrrH2YFO7u3qglZ6Q3AF0+wIBmJb/Elx5g/7AtH2YfZQ\nWg/OgPbe5KCjTTBLMA4GA1UdDwEB/wQEAwIHgDAMBgNVHRMBAf8EAjAAMCsGA1Ud\nIwQkMCKAINi5iJEdWH9Y643Ym+iLYDlBNr0DjannpvQ6wNbD/uPeMAoGCCqGSM49\nBAMCA0gAMEUCIQDFAcKtsGJX9dYJ+UBTcqybNUx5Q/krI8NtLDhEP3wryAIgHgEw\nyXrcyHsSnHY9fGtopF2uRrdFAnUPno99gnRhOM4=\n-----END CERTIFICATE-----\n" signature:"0D\002 \021\2342\"\002\265\037\036\2114+uJ\305_;\324\241\266ZL\200\362\355\362\337.\352p\014\242\025\002 t!a\257\250\345V^\257\363\325\326\213:M\206\032\226JV\247\023+\327u\354\023\373\327['u" >
		*/
		if err != nil {
			return nil, errors.WithMessage(err, fmt.Sprintf("error endorsing %s", funcName))
		}
		responses = append(responses, proposalResp)
	}

	if len(responses) == 0 {
		// this should only happen if some new code has introduced a bug
		return nil, errors.New("no proposal responses received - this might indicate a bug")
	}
	// all responses will be checked when the signed transaction is created.
	// for now, just set this so we check the first response's status
	proposalResp := responses[0]

	if invoke {
		fmt.Println("==============if invoke {==================")
		if proposalResp != nil {
			if proposalResp.Response.Status >= shim.ERRORTHRESHOLD {
				fmt.Println("==================proposalResp.Response.Status >= shim.ERRORTHRESHOLD ===============================")
				return proposalResp, nil
			}
			// assemble a signed transaction (it's an Envelope message)
			env, err := putils.CreateSignedTx(prop, signer, responses...)
			fmt.Println("======================env, err := putils.CreateSignedTx(prop, signer, responses...)==============================")
			if err != nil {
				return proposalResp, errors.WithMessage(err, "could not assemble transaction")
			}
			var dg *deliverGroup
			var ctx context.Context
			fmt.Println("======================waitForEvent==============================",waitForEvent)
			if waitForEvent {//false
				fmt.Println("======================waitForEvent==============================")

				var cancelFunc context.CancelFunc
				ctx, cancelFunc = context.WithTimeout(context.Background(), waitForEventTimeout)
				defer cancelFunc()

				dg = newDeliverGroup(deliverClients, peerAddresses, certificate, channelID, txid)
				// connect to deliver service on all peers
				err := dg.Connect(ctx)
				if err != nil {
					return nil, err
				}
			}

			// send the envelope for ordering
			fmt.Println("===================1.err = bc.Send(env)=============================")
			 err = bc.Send(env)
			 fmt.Println("===================2.err = bc.Send(env)=============================")
			if err != nil {
				return proposalResp, errors.WithMessage(err, fmt.Sprintf("error sending transaction for %s", funcName))
			}

			if dg != nil && ctx != nil {
				// wait for event that contains the txid from all peers
				err = dg.Wait(ctx)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return proposalResp, nil
}

// deliverGroup holds all of the information needed to connect
// to a set of peers to wait for the interested txid to be
// committed to the ledgers of all peers. This functionality
// is currently implemented via the peer's DeliverFiltered service.
// An error from any of the peers/deliver clients will result in
// the invoke command returning an error. Only the first error that
// occurs will be set
type deliverGroup struct {
	Clients     []*deliverClient
	Certificate tls.Certificate
	ChannelID   string
	TxID        string
	mutex       sync.Mutex
	Error       error
	wg          sync.WaitGroup
}

// deliverClient holds the client/connection related to a specific
// peer. The address is included for logging purposes
type deliverClient struct {
	Client     api.PeerDeliverClient
	Connection ccapi.Deliver
	Address    string
}

func newDeliverGroup(deliverClients []api.PeerDeliverClient, peerAddresses []string, certificate tls.Certificate, channelID string, txid string) *deliverGroup {
	fmt.Println("=============newDeliverGroup==================")
	clients := make([]*deliverClient, len(deliverClients))
	for i, client := range deliverClients {
		dc := &deliverClient{
			Client:  client,
			Address: peerAddresses[i],
		}
		clients[i] = dc
	}

	dg := &deliverGroup{
		Clients:     clients,
		Certificate: certificate,
		ChannelID:   channelID,
		TxID:        txid,
	}

	return dg
}

// Connect waits for all deliver clients in the group to connect to
// the peer's deliver service, receive an error, or for the context
// to timeout. An error will be returned whenever even a single
// deliver client fails to connect to its peer
func (dg *deliverGroup) Connect(ctx context.Context) error {
	fmt.Println("=========deliverGroup====Connect==================")
	dg.wg.Add(len(dg.Clients))
	for _, client := range dg.Clients {
		go dg.ClientConnect(ctx, client)
	}
	readyCh := make(chan struct{})
	go dg.WaitForWG(readyCh)

	select {
	case <-readyCh:
		if dg.Error != nil {
			err := errors.WithMessage(dg.Error, "failed to connect to deliver on all peers")
			return err
		}
	case <-ctx.Done():
		err := errors.New("timed out waiting for connection to deliver on all peers")
		return err
	}

	return nil
}

// ClientConnect sends a deliver seek info envelope using the
// provided deliver client, setting the deliverGroup's Error
// field upon any error
func (dg *deliverGroup) ClientConnect(ctx context.Context, dc *deliverClient) {
	fmt.Println("=========deliverGroup====ClientConnect==================")
	defer dg.wg.Done()
	df, err := dc.Client.DeliverFiltered(ctx)
	if err != nil {
		err = errors.WithMessage(err, fmt.Sprintf("error connecting to deliver filtered at %s", dc.Address))
		dg.setError(err)
		return
	}
	defer df.CloseSend()
	dc.Connection = df

	envelope := createDeliverEnvelope(dg.ChannelID, dg.Certificate)
	err = df.Send(envelope)
	if err != nil {
		err = errors.WithMessage(err, fmt.Sprintf("error sending deliver seek info envelope to %s", dc.Address))
		dg.setError(err)
		return
	}
}

// Wait waits for all deliver client connections in the group to
// either receive a block with the txid, an error, or for the
// context to timeout
func (dg *deliverGroup) Wait(ctx context.Context) error {
	fmt.Println("=========deliverGroup====Wait==================")
	if len(dg.Clients) == 0 {
		return nil
	}

	dg.wg.Add(len(dg.Clients))
	for _, client := range dg.Clients {
		go dg.ClientWait(client)
	}
	readyCh := make(chan struct{})
	go dg.WaitForWG(readyCh)

	select {
	case <-readyCh:
		if dg.Error != nil {
			err := errors.WithMessage(dg.Error, "failed to receive txid on all peers")
			return err
		}
	case <-ctx.Done():
		err := errors.New("timed out waiting for txid on all peers")
		return err
	}

	return nil
}

// ClientWait waits for the specified deliver client to receive
// a block event with the requested txid
func (dg *deliverGroup) ClientWait(dc *deliverClient) {
	fmt.Println("=========deliverGroup====ClientWait==================")
	defer dg.wg.Done()
	for {
		resp, err := dc.Connection.Recv()
		if err != nil {
			err = errors.WithMessage(err, fmt.Sprintf("error receiving from deliver filtered at %s", dc.Address))
			dg.setError(err)
			return
		}
		switch r := resp.Type.(type) {
		case *pb.DeliverResponse_FilteredBlock:
			filteredTransactions := r.FilteredBlock.FilteredTransactions
			for _, tx := range filteredTransactions {
				if tx.Txid == dg.TxID {
					logger.Infof("txid [%s] committed with status (%s) at %s", dg.TxID, tx.TxValidationCode, dc.Address)
					return
				}
			}
		case *pb.DeliverResponse_Status:
			err = errors.Errorf("deliver completed with status (%s) before txid received", r.Status)
			dg.setError(err)
			return
		default:
			err = errors.Errorf("received unexpected response type (%T) from %s", r, dc.Address)
			dg.setError(err)
			return
		}
	}
}

// WaitForWG waits for the deliverGroup's wait group and closes
// the channel when ready
func (dg *deliverGroup) WaitForWG(readyCh chan struct{}) {
	fmt.Println("=========deliverGroup====WaitForWG==================")
	dg.wg.Wait()
	close(readyCh)
}

// setError serializes an error for the deliverGroup
func (dg *deliverGroup) setError(err error) {
	fmt.Println("=========deliverGroup====setError==================")
	dg.mutex.Lock()
	dg.Error = err
	dg.mutex.Unlock()
}

func createDeliverEnvelope(channelID string, certificate tls.Certificate) *pcommon.Envelope {
	fmt.Println("========createDeliverEnvelope================")
	var tlsCertHash []byte
	// check for client certificate and create hash if present
	if len(certificate.Certificate) > 0 {
		tlsCertHash = util.ComputeSHA256(certificate.Certificate[0])
	}

	start := &ab.SeekPosition{
		Type: &ab.SeekPosition_Newest{
			Newest: &ab.SeekNewest{},
		},
	}

	stop := &ab.SeekPosition{
		Type: &ab.SeekPosition_Specified{
			Specified: &ab.SeekSpecified{
				Number: math.MaxUint64,
			},
		},
	}

	seekInfo := &ab.SeekInfo{
		Start:    start,
		Stop:     stop,
		Behavior: ab.SeekInfo_BLOCK_UNTIL_READY,
	}

	env, err := putils.CreateSignedEnvelopeWithTLSBinding(
		pcommon.HeaderType_DELIVER_SEEK_INFO, channelID, localmsp.NewSigner(),
		seekInfo, int32(0), uint64(0), tlsCertHash,"","")
	if err != nil {
		logger.Errorf("Error signing envelope: %s", err)
		return nil
	}

	return env
}
