/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package chaincode

import (
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/metrics"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/core/chaincode/platforms"
	"github.com/hyperledger/fabric/core/common/ccprovider"
	"github.com/hyperledger/fabric/core/common/sysccprovider"
	"github.com/hyperledger/fabric/core/container/ccintf"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/peer"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/pkg/errors"
)

// Runtime is used to manage chaincode runtime instances.
type Runtime interface {
	Start(ccci *ccprovider.ChaincodeContainerInfo, codePackage []byte) error
	Stop(ccci *ccprovider.ChaincodeContainerInfo) error
}

// Launcher is used to launch chaincode runtimes.
type Launcher interface {
	Launch(ccci *ccprovider.ChaincodeContainerInfo) error
}

// Lifecycle provides a way to retrieve chaincode definitions and the packages necessary to run them
type Lifecycle interface {
	// ChaincodeDefinition returns the details for a chaincode by name
	ChaincodeDefinition(chaincodeName string, qe ledger.QueryExecutor) (ccprovider.ChaincodeDefinition, error)

	// ChaincodeContainerInfo returns the package necessary to launch a chaincode
	ChaincodeContainerInfo(chaincodeName string, qe ledger.QueryExecutor) (*ccprovider.ChaincodeContainerInfo, error)
}

// ChaincodeSupport responsible for providing interfacing with chaincodes from the Peer.
type ChaincodeSupport struct {
	Keepalive        time.Duration
	ExecuteTimeout   time.Duration
	UserRunsCC       bool
	Runtime          Runtime
	ACLProvider      ACLProvider
	HandlerRegistry  *HandlerRegistry
	Launcher         Launcher
	SystemCCProvider sysccprovider.SystemChaincodeProvider
	Lifecycle        Lifecycle
	appConfig        ApplicationConfigRetriever
	HandlerMetrics   *HandlerMetrics
	LaunchMetrics    *LaunchMetrics
}

// NewChaincodeSupport creates a new ChaincodeSupport instance.
func NewChaincodeSupport(
	config *Config,
	peerAddress string,
	userRunsCC bool,
	caCert []byte,
	certGenerator CertGenerator,
	packageProvider PackageProvider,
	lifecycle Lifecycle,
	aclProvider ACLProvider,
	processor Processor,
	SystemCCProvider sysccprovider.SystemChaincodeProvider,
	platformRegistry *platforms.Registry,
	appConfig ApplicationConfigRetriever,
	metricsProvider metrics.Provider,
) *ChaincodeSupport {
	fmt.Println("====NewChaincodeSupport======")
	cs := &ChaincodeSupport{
		UserRunsCC:       userRunsCC,
		Keepalive:        config.Keepalive,
		ExecuteTimeout:   config.ExecuteTimeout,
		HandlerRegistry:  NewHandlerRegistry(userRunsCC),
		ACLProvider:      aclProvider,
		SystemCCProvider: SystemCCProvider,
		Lifecycle:        lifecycle,
		appConfig:        appConfig,
		HandlerMetrics:   NewHandlerMetrics(metricsProvider),
		LaunchMetrics:    NewLaunchMetrics(metricsProvider),
	}

	// Keep TestQueries working
	if !config.TLSEnabled {
		certGenerator = nil
	}

	cs.Runtime = &ContainerRuntime{
		CertGenerator:    certGenerator,
		Processor:        processor,
		CACert:           caCert,
		PeerAddress:      peerAddress,
		PlatformRegistry: platformRegistry,
		CommonEnv: []string{
			"CORE_CHAINCODE_LOGGING_LEVEL=" + config.LogLevel,
			"CORE_CHAINCODE_LOGGING_SHIM=" + config.ShimLogLevel,
			"CORE_CHAINCODE_LOGGING_FORMAT=" + config.LogFormat,
		},
	}

	cs.Launcher = &RuntimeLauncher{
		Runtime:         cs.Runtime,
		Registry:        cs.HandlerRegistry,
		PackageProvider: packageProvider,
		StartupTimeout:  config.StartupTimeout,
		Metrics:         cs.LaunchMetrics,
	}

	return cs
}

// LaunchInit bypasses getting the chaincode spec from the LSCC table
// as in the case of v1.0-v1.2 lifecycle, the chaincode will not yet be
// defined in the LSCC table
func (cs *ChaincodeSupport) LaunchInit(ccci *ccprovider.ChaincodeContainerInfo) error {
	fmt.Println("====ChaincodeSupport==LaunchInit====")
	cname := ccci.Name + ":" + ccci.Version
	if cs.HandlerRegistry.Handler(cname) != nil {
		return nil
	}

	return cs.Launcher.Launch(ccci)
}

// Launch starts executing chaincode if it is not already running. This method
// blocks until the peer side handler gets into ready state or encounters a fatal
// error. If the chaincode is already running, it simply returns.
func (cs *ChaincodeSupport) Launch(chainID, chaincodeName, chaincodeVersion string, qe ledger.QueryExecutor) (*Handler, error) {
	fmt.Println("====ChaincodeSupport==Launch====")
	cname := chaincodeName + ":" + chaincodeVersion
	fmt.Println("========cname===========",cname)
	if h := cs.HandlerRegistry.Handler(cname); h != nil {
		return h, nil
	}

	ccci, err := cs.Lifecycle.ChaincodeContainerInfo(chaincodeName, qe)
	if err != nil {
		// TODO: There has to be a better way to do this...
		if cs.UserRunsCC {
			chaincodeLogger.Error(
				"You are attempting to perform an action other than Deploy on Chaincode that is not ready and you are in developer mode. Did you forget to Deploy your chaincode?",
			)
		}

		return nil, errors.Wrapf(err, "[channel %s] failed to get chaincode container info for %s", chainID, cname)
	}

	if err := cs.Launcher.Launch(ccci); err != nil {
		return nil, errors.Wrapf(err, "[channel %s] could not launch chaincode %s", chainID, cname)
	}

	h := cs.HandlerRegistry.Handler(cname)
	if h == nil {
		return nil, errors.Wrapf(err, "[channel %s] claimed to start chaincode container for %s but could not find handler", chainID, cname)
	}

	return h, nil
}

// Stop stops a chaincode if running.
func (cs *ChaincodeSupport) Stop(ccci *ccprovider.ChaincodeContainerInfo) error {
	fmt.Println("====ChaincodeSupport==Stop====")
	return cs.Runtime.Stop(ccci)
}

// HandleChaincodeStream implements ccintf.HandleChaincodeStream for all vms to call with appropriate stream
func (cs *ChaincodeSupport) HandleChaincodeStream(stream ccintf.ChaincodeStream) error {
	fmt.Println("====ChaincodeSupport==HandleChaincodeStream====")
	handler := &Handler{
		Invoker:                    cs,
		DefinitionGetter:           cs.Lifecycle,
		Keepalive:                  cs.Keepalive,
		Registry:                   cs.HandlerRegistry,
		ACLProvider:                cs.ACLProvider,
		TXContexts:                 NewTransactionContexts(),
		ActiveTransactions:         NewActiveTransactions(),
		SystemCCProvider:           cs.SystemCCProvider,
		SystemCCVersion:            util.GetSysCCVersion(),
		InstantiationPolicyChecker: CheckInstantiationPolicyFunc(ccprovider.CheckInstantiationPolicy),
		QueryResponseBuilder:       &QueryResponseGenerator{MaxResultLimit: 100},
		UUIDGenerator:              UUIDGeneratorFunc(util.GenerateUUID),
		LedgerGetter:               peer.Default,
		AppConfig:                  cs.appConfig,
		Metrics:                    cs.HandlerMetrics,
	}

	return handler.ProcessStream(stream)
}

// Register the bidi stream entry point called by chaincode to register with the Peer.
func (cs *ChaincodeSupport) Register(stream pb.ChaincodeSupport_RegisterServer) error {
	fmt.Println("====ChaincodeSupport==Register====")
	return cs.HandleChaincodeStream(stream)
}

// createCCMessage creates a transaction message.
func createCCMessage(messageType pb.ChaincodeMessage_Type, cid string, txid string, cMsg *pb.ChaincodeInput) (*pb.ChaincodeMessage, error) {
	fmt.Println("====createCCMessage====")
	payload, err := proto.Marshal(cMsg)
	if err != nil {
		return nil, err
	}
	ccmsg := &pb.ChaincodeMessage{
		Type:      messageType,
		Payload:   payload,
		Txid:      txid,
		ChannelId: cid,
	}
	return ccmsg, nil
}

// ExecuteLegacyInit is a temporary method which should be removed once the old style lifecycle
// is entirely deprecated.  Ideally one release after the introduction of the new lifecycle.
// It does not attempt to start the chaincode based on the information from lifecycle, but instead
// accepts the container information directly in the form of a ChaincodeDeploymentSpec.
func (cs *ChaincodeSupport) ExecuteLegacyInit(txParams *ccprovider.TransactionParams, cccid *ccprovider.CCContext, spec *pb.ChaincodeDeploymentSpec) (*pb.Response, *pb.ChaincodeEvent, error) {
	fmt.Println("====ChaincodeSupport===ExecuteLegacyInit=")
	ccci := ccprovider.DeploymentSpecToChaincodeContainerInfo(spec)
	ccci.Version = cccid.Version

	err := cs.LaunchInit(ccci)
	if err != nil {
		return nil, nil, err
	}

	cname := ccci.Name + ":" + ccci.Version
	h := cs.HandlerRegistry.Handler(cname)
	if h == nil {
		return nil, nil, errors.Wrapf(err, "[channel %s] claimed to start chaincode container for %s but could not find handler", txParams.ChannelID, cname)
	}

	resp, err := cs.execute(pb.ChaincodeMessage_INIT, txParams, cccid, spec.GetChaincodeSpec().Input, h)
	return processChaincodeExecutionResult(txParams.TxID, cccid.Name, resp, err)
}

// Execute invokes chaincode and returns the original response.
func (cs *ChaincodeSupport) Execute(txParams *ccprovider.TransactionParams, cccid *ccprovider.CCContext, input *pb.ChaincodeInput) (*pb.Response, *pb.ChaincodeEvent, error) {
	fmt.Println("====ChaincodeSupport===Execute=")
	//fmt.Println("=====txParams========",txParams)
	/*
	&{49fc589652cc88ba05d6955b3b6ccea12a8bb88c8270807c2dcce6299cd2b002
	proposal_bytes:"\n\263\007\n[\010\003\032\013\010\267\346\327\214\006\020\306\345\317n*@49fc589652cc88ba05d6955b3b6ccea12a8bb88c8270807c2dcce6299cd2b002:\010\022\006\022\004lscc\022\323\006\n\266\006\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKjCCAdGgAwIBAgIRALv8NdhtIyJGHk1DO+cZfC8wCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTE1MDkyNzAwWhcNMzExMTEzMDkyNzAw\nWjBsMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEPMA0GA1UECxMGY2xpZW50MR8wHQYDVQQDDBZBZG1pbkBv\ncmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE/CCjMHcx\nvfqLB3z5Vk2reOaI1PRaaYorpw/Mp7ClnyESh2zNQZs5pgHJ9YvOcldEmBphsFpx\nwNMZzLszVl+rH6NNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYD\nVR0jBCQwIoAgK2A1hrR6qxl2EB98w7Jf2ym/5tg1vgaeXeJ3fLbWL2QwCgYIKoZI\nzj0EAwIDRwAwRAIgad6tAd+oYmiTPBB2jSLMTXsULylfGyDEIal7QWgaf0gCIDCL\njrYGhOqn1dbt/YRLCvDeOdexUJFj/0rFnJ9tWKrl\n-----END CERTIFICATE-----\n\022\030]\315\311\230\003L\365\010\034U\373a\334\tA\255\216\310\316\030\222\333\240\277\022(\n&\n$\010\001\022\006\022\004lscc\032\030\n\026getinstalledchaincodes" signature:"0D\002 ~\\\236-M\013q\373\243\017wp\202i>.Q\r-!\250n\342\261\000\311hS\334V\2713\002 \026\036\317\301\231\331\307\345\t\362B\325\226\256\241kF\237\202\342\250\\\256L\326\031E\261w\025\3407"  header:"\n[\010\003\032\013\010\267\346\327\214\006\020\306\345\317n*@49fc589652cc88ba05d6955b3b6ccea12a8bb88c8270807c2dcce6299cd2b002:\010\022\006\022\004lscc\022\323\006\n\266\006\n\007Org1MSP\022\252\006
	-----BEGIN CERTIFICATE-----\nMIICKjCCAdGgAwIBAgIRALv8NdhtIyJGHk1DO+cZfC8wCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTE1MDkyNzAwWhcNMzExMTEzMDkyNzAw\nWjBsMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEPMA0GA1UECxMGY2xpZW50MR8wHQYDVQQDDBZBZG1pbkBv\ncmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE/CCjMHcx\nvfqLB3z5Vk2reOaI1PRaaYorpw/Mp7ClnyESh2zNQZs5pgHJ9YvOcldEmBphsFpx\nwNMZzLszVl+rH6NNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYD\nVR0jBCQwIoAgK2A1hrR6qxl2EB98w7Jf2ym/5tg1vgaeXeJ3fLbWL2QwCgYIKoZI\nzj0EAwIDRwAwRAIgad6tAd+oYmiTPBB2jSLMTXsULylfGyDEIal7QWgaf0gCIDCL\njrYGhOqn1dbt/YRLCvDeOdexUJFj/0rFnJ9tWKrl\n-----END CERTIFICATE-----\n\022\030]\315\311\230\003L\365\010\034U\373a\334\tA\255\216\310\316\030\222\333\240\277" payload:"\n&\n$\010\001\022\006\022\004lscc\032\030\n\026getinstalledchaincodes"  <nil> <nil> <nil> false map[]}

	*/
	fmt.Println("=====cccid========",cccid)//&{lscc 1.4.0}
	fmt.Println("=====input========",input)
	//args:"getinstalledchaincodes"
	/*
	 =====input======== args:"install args:"\nT\010\001\022P\nFgithub.com/hyperledger/fabric-samples/chaincode/chaincode_example02/go\022\003acb\032\0010\032\260\007\037\213\010\000\000\000\000\000\000\377\324V\313k\334F\030\367U\363W|\021\024\244\342J\262]\0340\350\020\322&1\201b\342\322\2131eV\372\264+,\315\210\231ON\226bhhKRH\240P\347\320\264\224>.\205\036lz(\244n\232\177\306\273\266O\375\027\312\350\261\017g\327n\023(D\027\215\346\365{|\277\331Y\255\"\277\233R\257\354x\221\314\375^\277@\225a\334E\345'\274\243\322\350\035\315\363\"C\355G=\236\212H\3068n}\214\367\252\301`\331\357\312q\267\327\225\013\223O\020\004\301\352\352\273\325;\010\202\363\357\225\253+Km\273\356_\276\272\272\274\262\000\301\302\377\360\224\232\270Z\010^\033\353\274\2707\344)x\264\303\273\0109O\005ci^HE\3400\313Nr\262\231e_\230\r?\222j\"\016\276\356\245\371\345\213\n%Ij\277@T6s\031\243~\201\360!j\002M\252\214\010>a\326m\354\203\371LE\227Y\357q\342\355\307\036cI)\"p\250\227jx\333,sa]\244\344h*;`\030x\327[B\233Tv\326\005\241Jx\204.\030D\357\016\352B\n\215\006E!\225J\324\2136\313(B\255\035\221f\356<\224]\271\203\257\202\223\344\344m\250TP&\034;\014\303z\2470\014m\227Y\211X\204\202+\236#\241\322\260\026\202A\360n\"\335(ED\251\024\327D\2741\232\340\270\314\362}8\373\364\351\351\213\007\203\007\317\207O\016\007_=\036<zv\366\305\343\301\301\263\301\037\373'O??\373\372\257\343\243\237O~\270_O8=\374\354d\377\027f\245\t$\002\302\020l\036\307\306U\333\260{\211^3\330\360k]2Vx\315Pe\302$m\227Y{\200\231F\030ct\221\346c4\203\2630\232\241\231\030\254\022?\374\355\307\341w_\036\037\035\035?\177\322(\374\351\327\306\202\207\207\177\377\371\350\364\305\376\340\333\357\317\366\2779=8\030\376\376px\377`\272\330\357+%\225c\337\301\010\323]\214\241\024;B\336\205\2441\034R\261+#n\232\366\2340L\032qQ\032\026\201\253\256\206\255\355:\277\227\206\243\365\336v\031\263v\271\0022\007\303`V\345\313P8fC\027\256\204\260\\9;C\327\2658\006\324\304\t\301PR\n#\002Q\346\035T \023\303\250\314Q\220\366\354\306S\003\342\2313\027Vt\267\202\355\nm\334\035\202m\317C\333\301>D\\\200\220\004\035\004\314\013\352O\357\\\035\340f\353\245m\306,Tj\024\364\215\2226\rU\247E[\204\255\355N\277\3550k]\267\242cV]\tA\244\331\034*\250T\323r\033\370\377v\302'\223\367zE\235\002l\344\230\330O\006?\264\033]SE]\232g\363z]IqQ%-S\213\265\211\"*\324eFz\235Pq\222j\021&\235\277\211t+\325$U\377\206T\267\261\357\354`\337\304\356U\254\266bLP\3019<\357z&5:m\226c\223\203\332\215\213,\352a\226\311\225\221C\211|y\337[\\\177\200\367\310q\033z\265\363#y\347\247\327s\2315C\332\245\332\252\034Yq\035\341\026\311\373\210g%V\302gE\314\314\036g\314\334\2555\323\326\375j.qE\216\300\273N\025\276\231\021\237\372m\250(\201\371\273B\251\350B\"e\014\243\273\027\326\336\322v\245\277*\307\036\373\227\367\377?\000\000\000\377\377\032\305\243`\024\214\\\000\010\000\000\377\377\365b\271*\000\016\000\000"
	 =====input======== args:"deploy" args:"mychannel" args:"\nC\010\001\022'\n\035/home/cooper/project/union/go\022\003acb\032\0010\032\026\n\004init\n\001a\n\003100\n\001b\n\003200" args:"\022\010\022\006\010\001\022\002\010\000\032\r\022\013\n\007Org1MSP\020\003" args:"escc" args:"vscc"

	\022\010\022\006\010\001\022\002\010\000\032\r\022\013\n\007Org1MSP\020\003
	*/
	fmt.Println("=====input.Decorations========",input.Decorations)//args:"getinstalledchaincodes"
	resp, err := cs.Invoke(txParams, cccid, input)//=====input.Decorations======== map[]
	fmt.Println("=======resp=====",resp)
	//
	//type:COMPLETED payload:"\010\310\001\032\262\001\n\003acb\022\0010\032\004escc\"\004vscc*\031\022\010\022\006\010\001\022\002\010\000\032\r\022\013\n\007Org1MSP\020\0032D\n OO\267\000\357TF\034\372\002W\032\340\333\232\r\301\340\315\265Wt\204\246\327^h\3348\350\254\301\022 ~\010C\316\342/\007{\264\302\364\320b\366E\240\230\217\325\001\"\035\277\031\305\235\372\260l\230\232:: M\366\301aH\223\357\347KN\342\0338PE-\223*z,/\035\264\250\202\204J\0056\267l\207B\031\022\010\022\006\010\001\022\002\010\000\032\r\022\013\n\007Org1MSP\020\001" txid:"984a58ad2309e56ab128dfb77edf139a892dd2df44d53216c4fb3f4b26206a54" channel_id:"mychannel"
	//type:COMPLETED payload:"\010\310\001\032\002OK" txid:"0e3c9725098f1cd17bfeefbe4a69be61fe44af6962d9c7981fd01a7f209b40c9"
	//=======resp===== type:COMPLETED payload:"\010\310\001\032\262\001\n\003acb\022\0010\032\004escc\"\004vscc*\031\022\010\022\006\010\001\022\002\010\000\032\r\022\013\n\007Org1MSP\020\0032D\n OO\267\000\357TF\034\372\002W\032\340\333\232\r\301\340\315\265Wt\204\246\327^h\3348\350\254\301\022 ~\010C\316\342/\007{\264\302\364\320b\366E\240\230\217\325\001\"\035\277\031\305\235\372\260l\230\232:: M\366\301aH\223\357\347KN\342\0338PE-\223*z,/\035\264\250\202\204J\0056\267l\207B\031\022\010\022\006\010\001\022\002\010\000\032\r\022\013\n\007Org1MSP\020\001" txid:"984a58ad2309e56ab128dfb77edf139a892dd2df44d53216c4fb3f4b26206a54" channel_id:"mychannel"
	fmt.Println("=======err=====",err)
	return processChaincodeExecutionResult(txParams.TxID, cccid.Name, resp, err)
}

func processChaincodeExecutionResult(txid, ccName string, resp *pb.ChaincodeMessage, err error) (*pb.Response, *pb.ChaincodeEvent, error) {
	fmt.Println("====processChaincodeExecutionResult=")
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to execute transaction %s", txid)
	}
	if resp == nil {
		return nil, nil, errors.Errorf("nil response from transaction %s", txid)
	}

	if resp.ChaincodeEvent != nil {
		resp.ChaincodeEvent.ChaincodeId = ccName
		resp.ChaincodeEvent.TxId = txid
	}

	switch resp.Type {
	case pb.ChaincodeMessage_COMPLETED:
		res := &pb.Response{}
		err := proto.Unmarshal(resp.Payload, res)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to unmarshal response for transaction %s", txid)
		}
		return res, resp.ChaincodeEvent, nil

	case pb.ChaincodeMessage_ERROR:
		return nil, resp.ChaincodeEvent, errors.Errorf("transaction returned with failure: %s", resp.Payload)

	default:
		return nil, nil, errors.Errorf("unexpected response type %d for transaction %s", resp.Type, txid)
	}
}

func (cs *ChaincodeSupport) InvokeInit(txParams *ccprovider.TransactionParams, cccid *ccprovider.CCContext, input *pb.ChaincodeInput) (*pb.ChaincodeMessage, error) {
	fmt.Println("====ChaincodeSupport=")
	h, err := cs.Launch(txParams.ChannelID, cccid.Name, cccid.Version, txParams.TXSimulator)
	if err != nil {
		return nil, err
	}

	return cs.execute(pb.ChaincodeMessage_INIT, txParams, cccid, input, h)
}

// Invoke will invoke chaincode and return the message containing the response.
// The chaincode will be launched if it is not already running.
func (cs *ChaincodeSupport) Invoke(txParams *ccprovider.TransactionParams, cccid *ccprovider.CCContext, input *pb.ChaincodeInput) (*pb.ChaincodeMessage, error) {
	fmt.Println("====ChaincodeSupport=Invoke==")
	h, err := cs.Launch(txParams.ChannelID, cccid.Name, cccid.Version, txParams.TXSimulator)
	if err != nil {
		return nil, err
	}

	// TODO add Init exactly once semantics here once new lifecycle
	// is available.  Enforced if the target channel is using the new lifecycle
	//
	// First, the function name of the chaincode to invoke should be checked.  If it is
	// "init", then consider this invocation to be of type pb.ChaincodeMessage_INIT,
	// otherwise consider it to be of type pb.ChaincodeMessage_TRANSACTION,
	//
	// Secondly, A check should be made whether the chaincode has been
	// inited, then, if true, only allow cctyp pb.ChaincodeMessage_TRANSACTION,
	// otherwise, only allow cctype pb.ChaincodeMessage_INIT,
	cctype := pb.ChaincodeMessage_TRANSACTION

	return cs.execute(cctype, txParams, cccid, input, h)
}

// execute executes a transaction and waits for it to complete until a timeout value.
func (cs *ChaincodeSupport) execute(cctyp pb.ChaincodeMessage_Type, txParams *ccprovider.TransactionParams, cccid *ccprovider.CCContext, input *pb.ChaincodeInput, h *Handler) (*pb.ChaincodeMessage, error) {
	fmt.Println("====ChaincodeSupport=execute==")
	input.Decorations = txParams.ProposalDecorations

	ccMsg, err := createCCMessage(cctyp, txParams.ChannelID, txParams.TxID, input)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create chaincode message")
	}

	ccresp, err := h.Execute(txParams, cccid, ccMsg, cs.ExecuteTimeout)
	if err != nil {
		return nil, errors.WithMessage(err, fmt.Sprintf("error sending"))
	}

	return ccresp, nil
}
