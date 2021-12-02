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
	logger.Info("====NewChaincodeSupport======")
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
	logger.Info("====ChaincodeSupport==LaunchInit====")
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
	logger.Info("====ChaincodeSupport==Launch====")
	cname := chaincodeName + ":" + chaincodeVersion
	logger.Info("========cname===========",cname)
	/*
	1.query
	mycc:2.0
	 */
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
	logger.Info("====ChaincodeSupport==Stop====")
	return cs.Runtime.Stop(ccci)
}

// HandleChaincodeStream implements ccintf.HandleChaincodeStream for all vms to call with appropriate stream
func (cs *ChaincodeSupport) HandleChaincodeStream(stream ccintf.ChaincodeStream) error {
	logger.Info("====ChaincodeSupport==HandleChaincodeStream====")
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
	chaincodeLogger.Info("====ChaincodeSupport==Register====")
	return cs.HandleChaincodeStream(stream)
}

// createCCMessage creates a transaction message.
func createCCMessage(messageType pb.ChaincodeMessage_Type, cid string, txid string, cMsg *pb.ChaincodeInput) (*pb.ChaincodeMessage, error) {
	chaincodeLogger.Info("====createCCMessage====")
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
	chaincodeLogger.Infof("==========ccmsg := &pb.ChaincodeMessage{Type: %T,Payload:%v,Txid:%v,ChannelId:%v}======================",messageType,payload,txid,cid)
	return ccmsg, nil
}

// ExecuteLegacyInit is a temporary method which should be removed once the old style lifecycle
// is entirely deprecated.  Ideally one release after the introduction of the new lifecycle.
// It does not attempt to start the chaincode based on the information from lifecycle, but instead
// accepts the container information directly in the form of a ChaincodeDeploymentSpec.
func (cs *ChaincodeSupport) ExecuteLegacyInit(txParams *ccprovider.TransactionParams, cccid *ccprovider.CCContext, spec *pb.ChaincodeDeploymentSpec) (*pb.Response, *pb.ChaincodeEvent, error) {
	chaincodeLogger.Info("====ChaincodeSupport===ExecuteLegacyInit=")
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
	chaincodeLogger.Info("====ChaincodeSupport===Execute=")
	//logger.Info("=====txParams========",txParams)
	/*
	&{49fc589652cc88ba05d6955b3b6ccea12a8bb88c8270807c2dcce6299cd2b002
	proposal_bytes:"\n\263\007\n[\010\003\032\013\010\267\346\327\214\006\020\306\345\317n*@49fc589652cc88ba05d6955b3b6ccea12a8bb88c8270807c2dcce6299cd2b002:\010\022\006\022\004lscc\022\323\006\n\266\006\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKjCCAdGgAwIBAgIRALv8NdhtIyJGHk1DO+cZfC8wCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTE1MDkyNzAwWhcNMzExMTEzMDkyNzAw\nWjBsMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEPMA0GA1UECxMGY2xpZW50MR8wHQYDVQQDDBZBZG1pbkBv\ncmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE/CCjMHcx\nvfqLB3z5Vk2reOaI1PRaaYorpw/Mp7ClnyESh2zNQZs5pgHJ9YvOcldEmBphsFpx\nwNMZzLszVl+rH6NNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYD\nVR0jBCQwIoAgK2A1hrR6qxl2EB98w7Jf2ym/5tg1vgaeXeJ3fLbWL2QwCgYIKoZI\nzj0EAwIDRwAwRAIgad6tAd+oYmiTPBB2jSLMTXsULylfGyDEIal7QWgaf0gCIDCL\njrYGhOqn1dbt/YRLCvDeOdexUJFj/0rFnJ9tWKrl\n-----END CERTIFICATE-----\n\022\030]\315\311\230\003L\365\010\034U\373a\334\tA\255\216\310\316\030\222\333\240\277\022(\n&\n$\010\001\022\006\022\004lscc\032\030\n\026getinstalledchaincodes" signature:"0D\002 ~\\\236-M\013q\373\243\017wp\202i>.Q\r-!\250n\342\261\000\311hS\334V\2713\002 \026\036\317\301\231\331\307\345\t\362B\325\226\256\241kF\237\202\342\250\\\256L\326\031E\261w\025\3407"  header:"\n[\010\003\032\013\010\267\346\327\214\006\020\306\345\317n*@49fc589652cc88ba05d6955b3b6ccea12a8bb88c8270807c2dcce6299cd2b002:\010\022\006\022\004lscc\022\323\006\n\266\006\n\007Org1MSP\022\252\006
	-----BEGIN CERTIFICATE-----\nMIICKjCCAdGgAwIBAgIRALv8NdhtIyJGHk1DO+cZfC8wCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTE1MDkyNzAwWhcNMzExMTEzMDkyNzAw\nWjBsMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEPMA0GA1UECxMGY2xpZW50MR8wHQYDVQQDDBZBZG1pbkBv\ncmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE/CCjMHcx\nvfqLB3z5Vk2reOaI1PRaaYorpw/Mp7ClnyESh2zNQZs5pgHJ9YvOcldEmBphsFpx\nwNMZzLszVl+rH6NNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYD\nVR0jBCQwIoAgK2A1hrR6qxl2EB98w7Jf2ym/5tg1vgaeXeJ3fLbWL2QwCgYIKoZI\nzj0EAwIDRwAwRAIgad6tAd+oYmiTPBB2jSLMTXsULylfGyDEIal7QWgaf0gCIDCL\njrYGhOqn1dbt/YRLCvDeOdexUJFj/0rFnJ9tWKrl\n-----END CERTIFICATE-----\n\022\030]\315\311\230\003L\365\010\034U\373a\334\tA\255\216\310\316\030\222\333\240\277" payload:"\n&\n$\010\001\022\006\022\004lscc\032\030\n\026getinstalledchaincodes"  <nil> <nil> <nil> false map[]}

	*/
	chaincodeLogger.Infof("=====cccid========",cccid)
	/*
	  &{lscc 1.4.0}
	2.query
	&{mycc 2.0}
	 */

	chaincodeLogger.Info("=====input========",input)
	//args:"getinstalledchaincodes"
	/*
	 =====input======== args:"install args:"\nT\010\001\022P\nFgithub.com/hyperledger/fabric-samples/chaincode/chaincode_example02/go\022\003acb\032\0010\032\260\007\037\213\010\000\000\000\000\000\000\377\324V\313k\334F\030\367U\363W|\021\024\244\342J\262]\0340\350\020\322&1\201b\342\322\2131eV\372\264+,\315\210\231ON\226bhhKRH\240P\347\320\264\224>.\205\036lz(\244n\232\177\306\273\266O\375\027\312\350\261\017g\327n\023(D\027\215\346\365{|\277\331Y\255\"\277\233R\257\354x\221\314\375^\277@\225a\334E\345'\274\243\322\350\035\315\363\"C\355G=\236\212H\3068n}\214\367\252\301`\331\357\312q\267\327\225\013\223O\020\004\301\352\352\273\325;\010\202\363\357\225\253+Km\273\356_\276\272\272\274\262\000\301\302\377\360\224\232\270Z\010^\033\353\274\2707\344)x\264\303\273\0109O\005ci^HE\3400\313Nr\262\231e_\230\r?\222j\"\016\276\356\245\371\345\213\n%Ij\277@T6s\031\243~\201\360!j\002M\252\214\010>a\326m\354\203\371LE\227Y\357q\342\355\307\036cI)\"p\250\227jx\333,sa]\244\344h*;`\030x\327[B\233Tv\326\005\241Jx\204.\030D\357\016\352B\n\215\006E!\225J\324\2136\313(B\255\035\221f\356<\224]\271\203\257\202\223\344\344m\250TP&\034;\014\303z\2470\014m\227Y\211X\204\202+\236#\241\322\260\026\202A\360n\"\335(ED\251\024\327D\2741\232\340\270\314\362}8\373\364\351\351\213\007\203\007\317\207O\016\007_=\036<zv\366\305\343\301\301\263\301\037\373'O??\373\372\257\343\243\237O~\270_O8=\374\354d\377\027f\245\t$\002\302\020l\036\307\306U\333\260{\211^3\330\360k]2Vx\315Pe\302$m\227Y{\200\231F\030ct\221\346c4\203\2630\232\241\231\030\254\022?\374\355\307\341w_\036\037\035\035?\177\322(\374\351\327\306\202\207\207\177\377\371\350\364\305\376\340\333\357\317\366\2779=8\030\376\376px\377`\272\330\357+%\225c\337\301\010\323]\214\241\024;B\336\205\2441\034R\261+#n\232\366\2340L\032qQ\032\026\201\253\256\206\255\355:\277\227\206\243\365\336v\031\263v\271\0022\007\303`V\345\313P8fC\027\256\204\260\\9;C\327\2658\006\324\304\t\301PR\n#\002Q\346\035T \023\303\250\314Q\220\366\354\306S\003\342\2313\027Vt\267\202\355\nm\334\035\202m\317C\333\301>D\\\200\220\004\035\004\314\013\352O\357\\\035\340f\353\245m\306,Tj\024\364\215\2226\rU\247E[\204\255\355N\277\3550k]\267\242cV]\tA\244\331\034*\250T\323r\033\370\377v\302'\223\367zE\235\002l\344\230\330O\006?\264\033]SE]\232g\363z]IqQ%-S\213\265\211\"*\324eFz\235Pq\222j\021&\235\277\211t+\325$U\377\206T\267\261\357\354`\337\304\356U\254\266bLP\3019<\357z&5:m\226c\223\203\332\215\213,\352a\226\311\225\221C\211|y\337[\\\177\200\367\310q\033z\265\363#y\347\247\327s\2315C\332\245\332\252\034Yq\035\341\026\311\373\210g%V\302gE\314\314\036g\314\334\2555\323\326\375j.qE\216\300\273N\025\276\231\021\237\372m\250(\201\371\273B\251\350B\"e\014\243\273\027\326\336\322v\245\277*\307\036\373\227\367\377?\000\000\000\377\377\032\305\243`\024\214\\\000\010\000\000\377\377\365b\271*\000\016\000\000"
	 =====input======== args:"deploy" args:"mychannel" args:"\nC\010\001\022'\n\035/home/cooper/project/union/go\022\003acb\032\0010\032\026\n\004init\n\001a\n\003100\n\001b\n\003200" args:"\022\010\022\006\010\001\022\002\010\000\032\r\022\013\n\007Org1MSP\020\003" args:"escc" args:"vscc"

	\022\010\022\006\010\001\022\002\010\000\032\r\022\013\n\007Org1MSP\020\003


	2.初始化链码
	args:"deploy"
	args:"mychannel"
	args:"\nC\010\001\022'\n\035/home/cooper/project/union/go\022\003acb\032\0010\032\026\n\004init\n\001a\n\003100\n\001b\n\003200"
	args:"\022\010\022\006\010\001\022\002\010\000\032\r\022\013\n\007Org1MSP\020\003"
	args:"escc"
	args:"vscc"

	3.写入数据
	args:"GetConfigBlock" args:"mychannel"
	args:"addData" args:"91e9240f415223982edc345532630710e94a7f52cd5f48f5ee1afc555078f0ab" args:"hello1"


	4.query
	args:"query" args:"a"
	*/
	chaincodeLogger.Info("=====input.Decorations========",input.Decorations)//args:"getinstalledchaincodes"
	resp, err := cs.Invoke(txParams, cccid, input)//=====input.Decorations======== map[]
	chaincodeLogger.Info("=======resp=====",resp)
	/*
	1.query
	type:COMPLETED payload:"\010\310\001\032\003100" txid:"f415d931d5b9b34e460e450769eb0c357d6156e56cc0de6a3f0e7221b35e00cb" channel_id:"mychannel"
	2.
	 payload:"\010\310\001\032\276Y\n\"\032 \322^3\270\256\347\031\265\313\306\022z\024@\0305\30560R\211\333\273\310\311\237,\263s\021\210\360\022\214Y\n\211Y\n\276X\n\307\006\n\025\010\001\032\006\010\317\230\354\214\006\"\tmychannel\022\255\006\n\220\006\n\nOrdererMSP\022\201\006-----BEGIN CERTIFICATE-----\nMIICDDCCAbKgAwIBAgIQSyw/sBRvJOcG8iCGUUyZbTAKBggqhkjOPQQDAjBpMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEUMBIGA1UEChMLZXhhbXBsZS5jb20xFzAVBgNVBAMTDmNhLmV4YW1w\nbGUuY29tMB4XDTIxMTExNTA5MjcwMFoXDTMxMTExMzA5MjcwMFowWDELMAkGA1UE\nBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBGcmFuY2lz\nY28xHDAaBgNVBAMTE29yZGVyZXIuZXhhbXBsZS5jb20wWTATBgcqhkjOPQIBBggq\nhkjOPQMBBwNCAASzETD86u+3LoFl89mN67duFZWlUY7aHrY/SgxQzpwty2XMhd4e\nQKnH74U4AcKgE/umYjnFyNwBsOLUTYLIp9B7o00wSzAOBgNVHQ8BAf8EBAMCB4Aw\nDAYDVR0TAQH/BAIwADArBgNVHSMEJDAigCD1I0NmtmKoS1t8cw05Psjd6d19S+Q8\nn652Jly5+vr1CzAKBggqhkjOPQQDAgNIADBFAiEAryhZWbARJHJyolXolFmZjUiZ\n0Cf35dESm95RAeKjIXQCIC+YyEG+74GYo+U+5zr611413TL8dwg3MiIyvu2Y1W2G\n-----END CERTIFICATE-----\n\022\030\255\017\327d\351e\360?\221!5\365\225\231\324\3226b\003a\252x\242\007\022\361Q\n\201@\010\001\022\374?\022\253%\n\013Application\022\233%\010\001\022\374#\n\007Org1MSP\022\360#\032\221\"\n\003MSP\022\211\"\022\376!\022\373!\n\007Org1MSP\022\343\006-----BEGIN CERTIFICATE-----\nMIICUjCCAfigAwIBAgIRAJbG4pSISvSaU13iG92KnsQwCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTE1MDkyNzAwWhcNMzExMTEzMDkyNzAw\nWjBzMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEcMBoGA1UE\nAxMTY2Eub3JnMS5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IA\nBJtYBZ1Uaf8E2TU2J+8Suam4zpL6lDWUbyMHUf+va5KIZSdZsOhAIpUdKc27FDax\n7LKwXidF1JzZLm5RTzzm3AWjbTBrMA4GA1UdDwEB/wQEAwIBpjAdBgNVHSUEFjAU\nBggrBgEFBQcDAgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUwAwEB/zApBgNVHQ4EIgQg\nK2A1hrR6qxl2EB98w7Jf2ym/5tg1vgaeXeJ3fLbWL2QwCgYIKoZIzj0EAwIDSAAw\nRQIhAJ2ko4txOL6NOz4bvNOilj3JFUt5qCFnlmB1w9RxavNzAiBgI4sGPb6sQGjD\n44Aw+R9JojI/9cg9liWu3YWUJCJd7Q==\n-----END CERTIFICATE-----\n\"\252\006-----BEGIN CERTIFICATE-----\nMIICKjCCAdGgAwIBAgIRALv8NdhtIyJGHk1DO+cZfC8wCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTE1MDkyNzAwWhcNMzExMTEzMDkyNzAw\nWjBsMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEPMA0GA1UECxMGY2xpZW50MR8wHQYDVQQDDBZBZG1pbkBv\ncmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE/CCjMHcx\nvfqLB3z5Vk2reOaI1PRaaYorpw/Mp7ClnyESh2zNQZs5pgHJ9YvOcldEmBphsFpx\nwNMZzLszVl+rH6NNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYD\nVR0jBCQwIoAgK2A1hrR6qxl2EB98w7Jf2ym/5tg1vgaeXeJ3fLbWL2QwCgYIKoZI\nzj0EAwIDRwAwRAIgad6tAd+oYmiTPBB2jSLMTXsULylfGyDEIal7QWgaf0gCIDCL\njrYGhOqn1dbt/YRLCvDeOdexUJFj/0rFnJ9tWKrl\n-----END CERTIFICATE-----\nB\016\n\004SHA2\022\006SHA256J\347\006-----BEGIN CERTIFICATE-----\nMIICVzCCAf2gAwIBAgIQdyhof8BrzURC8Ycpmr/8pTAKBggqhkjOPQQDAjB2MQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEfMB0GA1UEAxMWdGxz\nY2Eub3JnMS5leGFtcGxlLmNvbTAeFw0yMTExMTUwOTI3MDBaFw0zMTExMTMwOTI3\nMDBaMHYxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQH\nEw1TYW4gRnJhbmNpc2NvMRkwFwYDVQQKExBvcmcxLmV4YW1wbGUuY29tMR8wHQYD\nVQQDExZ0bHNjYS5vcmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0D\nAQcDQgAErv8iYoPsOj8NBVBguTIJrrxsKGOkuAaUAhkSKDkJdmsY63xlf+7S2YpC\n8l25Q0nusFn8Uvz6JDfTfbj1Lahi86NtMGswDgYDVR0PAQH/BAQDAgGmMB0GA1Ud\nJQQWMBQGCCsGAQUFBwMCBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1Ud\nDgQiBCDu9/G3386WQ6sJke5vW1vIcGQvGj9oGe2iOTro0IY3eDAKBggqhkjOPQQD\nAgNIADBFAiEArOG2XThTVadjudNY0Qra5kpftx1mxO9iX4hzDDTCAjwCIArQdEJo\nfhJajlvu5fGTSy6Z4UsfEHIsE1WIF2GgJ9w6\n-----END CERTIFICATE-----\nZ\342\r\010\001\022\356\006\n\343\006-----BEGIN CERTIFICATE-----\nMIICUjCCAfigAwIBAgIRAJbG4pSISvSaU13iG92KnsQwCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTE1MDkyNzAwWhcNMzExMTEzMDkyNzAw\nWjBzMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEcMBoGA1UE\nAxMTY2Eub3JnMS5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IA\nBJtYBZ1Uaf8E2TU2J+8Suam4zpL6lDWUbyMHUf+va5KIZSdZsOhAIpUdKc27FDax\n7LKwXidF1JzZLm5RTzzm3AWjbTBrMA4GA1UdDwEB/wQEAwIBpjAdBgNVHSUEFjAU\nBggrBgEFBQcDAgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUwAwEB/zApBgNVHQ4EIgQg\nK2A1hrR6qxl2EB98w7Jf2ym/5tg1vgaeXeJ3fLbWL2QwCgYIKoZIzj0EAwIDSAAw\nRQIhAJ2ko4txOL6NOz4bvNOilj3JFUt5qCFnlmB1w9RxavNzAiBgI4sGPb6sQGjD\n44Aw+R9JojI/9cg9liWu3YWUJCJd7Q==\n-----END CERTIFICATE-----\n\022\006client\032\354\006\n\343\006-----BEGIN CERTIFICATE-----\nMIICUjCCAfigAwIBAgIRAJbG4pSISvSaU13iG92KnsQwCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTE1MDkyNzAwWhcNMzExMTEzMDkyNzAw\nWjBzMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEcMBoGA1UE\nAxMTY2Eub3JnMS5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IA\nBJtYBZ1Uaf8E2TU2J+8Suam4zpL6lDWUbyMHUf+va5KIZSdZsOhAIpUdKc27FDax\n7LKwXidF1JzZLm5RTzzm3AWjbTBrMA4GA1UdDwEB/wQEAwIBpjAdBgNVHSUEFjAU\nBggrBgEFBQcDAgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUwAwEB/zApBgNVHQ4EIgQg\nK2A1hrR6qxl2EB98w7Jf2ym/5tg1vgaeXeJ3fLbWL2QwCgYIKoZIzj0EAwIDSAAw\nRQIhAJ2ko4txOL6NOz4bvNOilj3JFUt5qCFnlmB1w9RxavNzAiBgI4sGPb6sQGjD\n44Aw+R9JojI/9cg9liWu3YWUJCJd7Q==\n-----END CERTIFICATE-----\n\022\004peer\032\006Admins\"E\n\007Writers\022:\0220\010\001\022,\022\014\022\n\010\001\022\002\010\000\022\002\010\001\032\r\022\013\n\007Org1MSP\020\001\032\r\022\013\n\007Org1MSP\020\002\032\006Admins\"1\n\006Admins\022'\022\035\010\001\022\031\022\010\022\006\010\001\022\002\010\000\032\r\022\013\n\007Org1MSP\020\001\032\006Admins\"X\n\007Readers\022M\022C\010\001\022?\022\020\022\016\010\001\022\002\010\000\022\002\010\001\022\002\010\002\032\r\022\013\n\007Org1MSP\020\001\032\r\022\013\n\007Org1MSP\020\003\032\r\022\013\n\007Org1MSP\020\002\032\006Admins*\006Admins\032$\n\014Capabilities\022\024\022\n\n\010\n\004V1_3\022\000\032\006Admins\"\"\n\007Readers\022\027\022\r\010\003\022\t\n\007Readers\032\006Admins\"\"\n\007Writers\022\027\022\r\010\003\022\t\n\007Writers\032\006Admins\"\"\n\006Admins\022\030\022\016\010\003\022\n\n\006Admins\020\002\032\006Admins*\006Admins\022\343\027\n\007Orderer\022\327\027\022\203\025\n\nOrdererOrg\022\364\024\032\311\023\n\003MSP\022\301\023\022\266\023\022\263\023\n\nOrdererMSP\022\302\006-----BEGIN CERTIFICATE-----\nMIICPDCCAeOgAwIBAgIQK9Bj5FX48wVqGaSwVXJedTAKBggqhkjOPQQDAjBpMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEUMBIGA1UEChMLZXhhbXBsZS5jb20xFzAVBgNVBAMTDmNhLmV4YW1w\nbGUuY29tMB4XDTIxMTExNTA5MjcwMFoXDTMxMTExMzA5MjcwMFowaTELMAkGA1UE\nBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBGcmFuY2lz\nY28xFDASBgNVBAoTC2V4YW1wbGUuY29tMRcwFQYDVQQDEw5jYS5leGFtcGxlLmNv\nbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABKWatETBIS68oeKAfFWO+Zj1hNjm\ncIYG/OihwH4VxkelJ0Yoa+/tSDWUMps3cqeaC3AGEWDNsuPiOX/7vYOnJ9ejbTBr\nMA4GA1UdDwEB/wQEAwIBpjAdBgNVHSUEFjAUBggrBgEFBQcDAgYIKwYBBQUHAwEw\nDwYDVR0TAQH/BAUwAwEB/zApBgNVHQ4EIgQg9SNDZrZiqEtbfHMNOT7I3endfUvk\nPJ+udiZcufr69QswCgYIKoZIzj0EAwIDRwAwRAIgFTsIzEwIaWWwohj3k+2cxLMh\nxNQF/HYDcyX8sSeOI2ICIE9ILRcE5lvXl5atr8fiTm6VzA2w+17Xx9LB5wOp8Ybb\n-----END CERTIFICATE-----\n\"\201\006-----BEGIN CERTIFICATE-----\nMIICCzCCAbGgAwIBAgIRALMPTg+REO8nJW7MCQJ3v1AwCgYIKoZIzj0EAwIwaTEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xFDASBgNVBAoTC2V4YW1wbGUuY29tMRcwFQYDVQQDEw5jYS5leGFt\ncGxlLmNvbTAeFw0yMTExMTUwOTI3MDBaFw0zMTExMTMwOTI3MDBaMFYxCzAJBgNV\nBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1TYW4gRnJhbmNp\nc2NvMRowGAYDVQQDDBFBZG1pbkBleGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqG\nSM49AwEHA0IABBUPTU65m/ajKhrO8YFv9Utny2TG9DoZ/UNcuOaTD7T89DlmczM2\na5/nwOiiE65c2dcUmj3UYxHaxtrdLpqQXxGjTTBLMA4GA1UdDwEB/wQEAwIHgDAM\nBgNVHRMBAf8EAjAAMCsGA1UdIwQkMCKAIPUjQ2a2YqhLW3xzDTk+yN3p3X1L5Dyf\nrnYmXLn6+vULMAoGCCqGSM49BAMCA0gAMEUCIQCClIfFoBpd9tVnE9t0BKe5fr3/\nvr3fz0bfUGWMfB3y9AIgYaUOlPZ3q4t0s6yjG9uDuT+k6yC/zUzf/sdklzKENrg=\n-----END CERTIFICATE-----\nB\016\n\004SHA2\022\006SHA256J\313\006-----BEGIN CERTIFICATE-----\nMIICQjCCAemgAwIBAgIQX0ZKz/1/IzTHyYfiPoppbDAKBggqhkjOPQQDAjBsMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEUMBIGA1UEChMLZXhhbXBsZS5jb20xGjAYBgNVBAMTEXRsc2NhLmV4\nYW1wbGUuY29tMB4XDTIxMTExNTA5MjcwMFoXDTMxMTExMzA5MjcwMFowbDELMAkG\nA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBGcmFu\nY2lzY28xFDASBgNVBAoTC2V4YW1wbGUuY29tMRowGAYDVQQDExF0bHNjYS5leGFt\ncGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABIakYDGsEbWWBIoWywip\nbGIPw65vrc8aa2zp1w1nl3fgxSUHhTX6WlVckyqFydFasnMiigANrZaQGkSpr577\n0amjbTBrMA4GA1UdDwEB/wQEAwIBpjAdBgNVHSUEFjAUBggrBgEFBQcDAgYIKwYB\nBQUHAwEwDwYDVR0TAQH/BAUwAwEB/zApBgNVHQ4EIgQgTR/glD0LEtUTXPLj7Rne\nH3NNmQ/NTHN04NCc3EV8pX0wCgYIKoZIzj0EAwIDRwAwRAIgCc5M7ATAXw2cGOhz\nLkQHkldQO7yyHIH5Ns/H6CyeMXUCIAneNpK4pTH7lxOQm00TFuT2H/uTbfaKWVNy\ndm/FeD9b\n-----END CERTIFICATE-----\n\032\006Admins\"3\n\007Readers\022(\022\036\010\001\022\032\022\010\022\006\010\001\022\002\010\000\032\016\022\014\n\nOrdererMSP\032\006Admins\"3\n\007Writers\022(\022\036\010\001\022\032\022\010\022\006\010\001\022\002\010\000\032\016\022\014\n\nOrdererMSP\032\006Admins\"4\n\006Admins\022*\022 \010\001\022\034\022\010\022\006\010\001\022\002\010\000\032\020\022\016\n\nOrdererMSP\020\001\032\006Admins*\006Admins\032\037\n\023ChannelRestrictions\022\010\032\006Admins\032$\n\014Capabilities\022\024\022\n\n\010\n\004V1_1\022\000\032\006Admins\032!\n\rConsensusType\022\020\022\006\n\004solo\032\006Admins\032#\n\tBatchSize\022\026\022\014\010\220N\020\300\275\232/\030\200\240\037\032\006Admins\032 \n\014BatchTimeout\022\020\022\006\n\0041\302\265s\032\006Admins\"\"\n\007Writers\022\027\022\r\010\003\022\t\n\007Writers\032\006Admins\"\"\n\006Admins\022\030\022\016\010\003\022\n\n\006Admins\020\002\032\006Admins\"*\n\017BlockValidation\022\027\022\r\010\003\022\t\n\007Writers\032\006Admins\"\"\n\007Readers\022\027\022\r\010\003\022\t\n\007Readers\032\006Admins*\006Admins\032I\n\020OrdererAddresses\0225\022\032\n\030orderer.example.com:7050\032\027/Channel/Orderer/Admins\032$\n\014Capabilities\022\024\022\n\n\010\n\004V1_3\022\000\032\006Admins\032*\n\nConsortium\022\034\022\022\n\020SampleConsortium\032\006Admins\032&\n\020HashingAlgorithm\022\022\022\010\n\006SHA256\032\006Admins\032-\n\031BlockDataHashingStructure\022\020\022\006\010\377\377\377\377\017\032\006Admins\"\"\n\006Admins\022\030\022\016\010\003\022\n\n\006Admins\020\002\032\006Admins\"\"\n\007Readers\022\027\022\r\010\003\022\t\n\007Readers\032\006Admins\"\"\n\007Writers\022\027\022\r\010\003\022\t\n\007Writers\032\006Admins*\006Admins\022\352\021\n\236\021\n\325\007\n}\010\002\032\n\010\317\230\354\214\006\020\275\327W\"\tmychannel*@480b97a430f1b5ac5f0348626ef9d0d1de32577eb052f84e2a049faba10d2316B \343\260\304B\230\374\034\024\232\373\364\310\231o\271$'\256A\344d\233\223L\244\225\231\033xR\270U\022\323\006\n\266\006\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKjCCAdGgAwIBAgIRALv8NdhtIyJGHk1DO+cZfC8wCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTE1MDkyNzAwWhcNMzExMTEzMDkyNzAw\nWjBsMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEPMA0GA1UECxMGY2xpZW50MR8wHQYDVQQDDBZBZG1pbkBv\ncmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE/CCjMHcx\nvfqLB3z5Vk2reOaI1PRaaYorpw/Mp7ClnyESh2zNQZs5pgHJ9YvOcldEmBphsFpx\nwNMZzLszVl+rH6NNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYD\nVR0jBCQwIoAgK2A1hrR6qxl2EB98w7Jf2ym/5tg1vgaeXeJ3fLbWL2QwCgYIKoZI\nzj0EAwIDRwAwRAIgad6tAd+oYmiTPBB2jSLMTXsULylfGyDEIal7QWgaf0gCIDCL\njrYGhOqn1dbt/YRLCvDeOdexUJFj/0rFnJ9tWKrl\n-----END CERTIFICATE-----\n\022\030\340\364.\242\357L\374}\374\367<}X`\264\337\017\205@\025hx9s\022\303\t\n\236\002\n\tmychannel\022.\022\034\n\013Application\022\r\022\013\n\007Org1MSP\022\000\032\016\n\nConsortium\022\000\032\340\001\022\271\001\n\013Application\022\251\001\010\001\022\013\n\007Org1MSP\022\000\032$\n\014Capabilities\022\024\022\n\n\010\n\004V1_3\022\000\032\006Admins\"\"\n\007Readers\022\027\022\r\010\003\022\t\n\007Readers\032\006Admins\"\"\n\007Writers\022\027\022\r\010\003\022\t\n\007Writers\032\006Admins\"\"\n\006Admins\022\030\022\016\010\003\022\n\n\006Admins\020\002\032\006Admins*\006Admins\032\"\n\nConsortium\022\024\022\022\n\020SampleConsortium\022\237\007\n\323\006\n\266\006\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKjCCAdGgAwIBAgIRALv8NdhtIyJGHk1DO+cZfC8wCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTE1MDkyNzAwWhcNMzExMTEzMDkyNzAw\nWjBsMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEPMA0GA1UECxMGY2xpZW50MR8wHQYDVQQDDBZBZG1pbkBv\ncmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE/CCjMHcx\nvfqLB3z5Vk2reOaI1PRaaYorpw/Mp7ClnyESh2zNQZs5pgHJ9YvOcldEmBphsFpx\nwNMZzLszVl+rH6NNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYD\nVR0jBCQwIoAgK2A1hrR6qxl2EB98w7Jf2ym/5tg1vgaeXeJ3fLbWL2QwCgYIKoZI\nzj0EAwIDRwAwRAIgad6tAd+oYmiTPBB2jSLMTXsULylfGyDEIal7QWgaf0gCIDCL\njrYGhOqn1dbt/YRLCvDeOdexUJFj/0rFnJ9tWKrl\n-----END CERTIFICATE-----\n\022\030\004D;\"+\260\\).\207\357\351\300\313^\336(\257/\r\023\312x\221\022G0E\002!\000\216\014YI\326B\"_z\350\245\274\366YR\tAE\256\205\212\207S\224\315\331\312\310+\347D\004\002 )D\361\242m\013|g\255\300+\361*Z\323\361\376\351q\260\251q\354c\223\332\340\000\026\022\254d\022G0E\002!\000\252\207\032\202\233\274.\250\331\013S\313h\375\004T\241\374\312?6\372\371\236 \305\266\234-\271\232\236\002 9\356h\020\323S\334\277-M\0318\035\340\000\036\217\352\332\2204\273\320_\260)\200\202\272\314\330$\022F0D\002 F\206N\305\033\2300\374l*#\rvy\021\361\355\316K\013\315|\331h\032\344\376h\037\276VU\002 C\373\305ib.\375\247\003\321\252\003\013\214#\206UH\314y4\363\273\321\347\025\037\005\301\231C\220\032\t\n\000\n\000\n\001\000\n\000" txid:"09eedf2cbf08963ced4f5e71a430de941c7f20e4e211ceb449f75fc06a337dc0" channel_id:"mychannel"
	 */
	//
	//type:COMPLETED payload:"\010\310\001\032\262\001\n\003acb\022\0010\032\004escc\"\004vscc*\031\022\010\022\006\010\001\022\002\010\000\032\r\022\013\n\007Org1MSP\020\0032D\n OO\267\000\357TF\034\372\002W\032\340\333\232\r\301\340\315\265Wt\204\246\327^h\3348\350\254\301\022 ~\010C\316\342/\007{\264\302\364\320b\366E\240\230\217\325\001\"\035\277\031\305\235\372\260l\230\232:: M\366\301aH\223\357\347KN\342\0338PE-\223*z,/\035\264\250\202\204J\0056\267l\207B\031\022\010\022\006\010\001\022\002\010\000\032\r\022\013\n\007Org1MSP\020\001" txid:"984a58ad2309e56ab128dfb77edf139a892dd2df44d53216c4fb3f4b26206a54" channel_id:"mychannel"
	//type:COMPLETED payload:"\010\310\001\032\002OK" txid:"0e3c9725098f1cd17bfeefbe4a69be61fe44af6962d9c7981fd01a7f209b40c9"
	//=======resp===== type:COMPLETED payload:"\010\310\001\032\262\001\n\003acb\022\0010\032\004escc\"\004vscc*\031\022\010\022\006\010\001\022\002\010\000\032\r\022\013\n\007Org1MSP\020\0032D\n OO\267\000\357TF\034\372\002W\032\340\333\232\r\301\340\315\265Wt\204\246\327^h\3348\350\254\301\022 ~\010C\316\342/\007{\264\302\364\320b\366E\240\230\217\325\001\"\035\277\031\305\235\372\260l\230\232:: M\366\301aH\223\357\347KN\342\0338PE-\223*z,/\035\264\250\202\204J\0056\267l\207B\031\022\010\022\006\010\001\022\002\010\000\032\r\022\013\n\007Org1MSP\020\001" txid:"984a58ad2309e56ab128dfb77edf139a892dd2df44d53216c4fb3f4b26206a54" channel_id:"mychannel"
	chaincodeLogger.Info("=======err=====",err)
	return processChaincodeExecutionResult(txParams.TxID, cccid.Name, resp, err)
}

func processChaincodeExecutionResult(txid, ccName string, resp *pb.ChaincodeMessage, err error) (*pb.Response, *pb.ChaincodeEvent, error) {
	logger.Info("====processChaincodeExecutionResult=")
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
	logger.Info("====ChaincodeSupport=")
	h, err := cs.Launch(txParams.ChannelID, cccid.Name, cccid.Version, txParams.TXSimulator)
	if err != nil {
		return nil, err
	}

	return cs.execute(pb.ChaincodeMessage_INIT, txParams, cccid, input, h)
}

// Invoke will invoke chaincode and return the message containing the response.
// The chaincode will be launched if it is not already running.
func (cs *ChaincodeSupport) Invoke(txParams *ccprovider.TransactionParams, cccid *ccprovider.CCContext, input *pb.ChaincodeInput) (*pb.ChaincodeMessage, error) {
	chaincodeLogger.Info("====ChaincodeSupport=Invoke==")
	h, err := cs.Launch(txParams.ChannelID, cccid.Name, cccid.Version, txParams.TXSimulator)
	if err != nil {
		return nil, err
	}

	chaincodeLogger.Infof("===========%v, %v := cs.Launch(%v,%v,%v,%v)==================",h,err,txParams.ChannelID,cccid.Name,cccid.Version,txParams.TXSimulator)

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


	re,err := cs.execute(cctype, txParams, cccid, input, h)

	chaincodeLogger.Infof("========%v,%v := cs.execute(%v,%v, %v,%v,%v)================",re,err,cctype,txParams,cccid,input,h)
	return re,err
}

// execute executes a transaction and waits for it to complete until a timeout value.
func (cs *ChaincodeSupport) execute(cctyp pb.ChaincodeMessage_Type, txParams *ccprovider.TransactionParams, cccid *ccprovider.CCContext, input *pb.ChaincodeInput, h *Handler) (*pb.ChaincodeMessage, error) {
	logger.Info("====ChaincodeSupport=execute==")
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
