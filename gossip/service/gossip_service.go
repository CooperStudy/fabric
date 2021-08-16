/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package service

import (
	"sync"

	"github.com/hyperledger/fabric/core/committer"
	"github.com/hyperledger/fabric/core/committer/txvalidator"
	"github.com/hyperledger/fabric/core/common/privdata"
	"github.com/hyperledger/fabric/core/deliverservice"
	"github.com/hyperledger/fabric/core/deliverservice/blocksprovider"
	"github.com/hyperledger/fabric/gossip/api"
	gossipCommon "github.com/hyperledger/fabric/gossip/common"
	"github.com/hyperledger/fabric/gossip/election"
	"github.com/hyperledger/fabric/gossip/gossip"
	"github.com/hyperledger/fabric/gossip/integration"
	privdata2 "github.com/hyperledger/fabric/gossip/privdata"
	"github.com/hyperledger/fabric/gossip/state"
	"github.com/hyperledger/fabric/gossip/util"
	"github.com/hyperledger/fabric/protos/common"
	gproto "github.com/hyperledger/fabric/protos/gossip"
	"github.com/hyperledger/fabric/protos/ledger/rwset"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

var (
	gossipServiceInstance *gossipServiceImpl //全局实例，即代表了peer下的gossip服务，gossipServiceInstance是一gossipServiceImpl
	//类型的结构体，
	once                  sync.Once
)

type gossipSvc gossip.Gossip

// GossipService encapsulates gossip and state capabilities into single interface
type GossipService interface {
	gossip.Gossip

	// DistributePrivateData distributes private data to the peers in the collections
	// according to policies induced by the PolicyStore and PolicyParser
	DistributePrivateData(chainID string, txID string, privateData *rwset.TxPvtReadWriteSet) error
	// NewConfigEventer creates a ConfigProcessor which the channelconfig.BundleSource can ultimately route config updates to
	NewConfigEventer() ConfigProcessor
	// InitializeChannel allocates the state provider and should be invoked once per channel per execution
	InitializeChannel(chainID string, endpoints []string, support Support)
	// AddPayload appends message payload to for given chain
	AddPayload(chainID string, payload *gproto.Payload) error
}

// DeliveryServiceFactory factory to create and initialize delivery service instance
type DeliveryServiceFactory interface {
	// Returns an instance of delivery client
	//返回一个delivery client的实例，sevice方法调用
	Service(g GossipService, endpoints []string, msc api.MessageCryptoService) (deliverclient.DeliverService, error)
}

type deliveryFactoryImpl struct {
}

// Returns an instance of delivery client
func (*deliveryFactoryImpl) Service(g GossipService, endpoints []string, mcs api.MessageCryptoService) (deliverclient.DeliverService, error) {
	return deliverclient.NewDeliverService(&deliverclient.Config{
		CryptoSvc:   mcs,
		Gossip:      g,
		Endpoints:   endpoints,
		ConnFactory: deliverclient.DefaultConnectionFactory,
		ABCFactory:  deliverclient.DefaultABCFactory,
	})
}

type privateHandler struct {
	support     Support
	coordinator privdata2.Coordinator
	distributor privdata2.PvtDataDistributor
}

func (p privateHandler) close() {
	p.coordinator.Close()
}

type gossipServiceImpl struct {
	gossipSvc 	//变量类型，继承interface接口，
	privateHandlers map[string]privateHandler //维护了当前peer中每一个channel到其privateHandle结构体的映射
	chains          map[string]state.GossipStateProvider //维护了当前peer中，每一个channel到其GossipStateProvider的映射
	leaderElection  map[string]election.LeaderElectionService //如果哦一个peer的core.yaml文件中peer.gossip.useLeaderElection配置
	//项目为true，则该peer对每一个channel都会开启一个leaderElection模块，用于领导节点的选举。
	deliveryService map[string]deliverclient.DeliverService //gossip服务的deliveryService维护了当前peer中每一个channel到其DeliverService的映射，
	//在InitializeChannel中完成初始化
	deliveryFactory DeliveryServiceFactory //用于生成创建并且初始化delivery service实例的组件，类型为DeliveryServiceFactory，
	lock            sync.RWMutex //即Gossip服务实例的读写锁，在gossip_service.go文件下的gossipServiceImpl结构体的各个方法中，有的使用了写锁定，有的使用了读锁定
	//Initialize Channel写锁定，Stop方法 写锁定
	mcs             api.MessageCryptoService //MessageCryptoService是在Gossip服务组件与peer一同被Gossip服务组件验证与授权远程peer月peer们所发送的数据，
	//也用来验证从Ordering Service收到的区块
	peerIdentity    []byte
	secAdv          api.SecurityAdvisor
}

// This is an implementation of api.JoinChannelMessage.
type joinChannelMessage struct {
	seqNum              uint64
	members2AnchorPeers map[string][]api.AnchorPeer
}

func (jcm *joinChannelMessage) SequenceNumber() uint64 {
	return jcm.seqNum
}

// Members returns the organizations of the channel
func (jcm *joinChannelMessage) Members() []api.OrgIdentityType {
	members := make([]api.OrgIdentityType, len(jcm.members2AnchorPeers))
	i := 0
	for org := range jcm.members2AnchorPeers {
		members[i] = api.OrgIdentityType(org)
		i++
	}
	return members
}

// AnchorPeersOf returns the anchor peers of the given organization
func (jcm *joinChannelMessage) AnchorPeersOf(org api.OrgIdentityType) []api.AnchorPeer {
	return jcm.members2AnchorPeers[string(org)]
}

var logger = util.GetLogger(util.LoggingServiceModule, "")

// InitGossipService initialize gossip service
func InitGossipService(peerIdentity []byte, endpoint string, s *grpc.Server, certs *gossipCommon.TLSCertificates,
	mcs api.MessageCryptoService, secAdv api.SecurityAdvisor, secureDialOpts api.PeerSecureDialOpts, bootPeers ...string) error {
	// TODO: Remove this.
	// TODO: This is a temporary work-around to make the gossip leader election module load its logger at startup
	// TODO: in order for the flogging package to register this logger in time so it can set the log levels as requested in the config
	util.GetLogger(util.LoggingElectionModule, "")
	return InitGossipServiceCustomDeliveryFactory(peerIdentity, endpoint, s, certs, &deliveryFactoryImpl{},
		mcs, secAdv, secureDialOpts, bootPeers...)
}

// InitGossipServiceCustomDeliveryFactory initialize gossip service with customize delivery factory
// implementation, might be useful for testing and mocking purposes
func InitGossipServiceCustomDeliveryFactory(peerIdentity []byte, endpoint string, s *grpc.Server,
	certs *gossipCommon.TLSCertificates, factory DeliveryServiceFactory, mcs api.MessageCryptoService,
	secAdv api.SecurityAdvisor, secureDialOpts api.PeerSecureDialOpts, bootPeers ...string) error {
	var err error
	var gossip gossip.Gossip
	once.Do(func() {
		if overrideEndpoint := viper.GetString("peer.gossip.endpoint"); overrideEndpoint != "" {
			endpoint = overrideEndpoint
		}

		logger.Info("Initialize gossip with endpoint", endpoint, "and bootstrap set", bootPeers)

		gossip, err = integration.NewGossipComponent(peerIdentity, endpoint, s, secAdv,
			mcs, secureDialOpts, certs, bootPeers...)

		gossipServiceInstance = &gossipServiceImpl{
			mcs:             mcs,
			gossipSvc:       gossip,
			privateHandlers: make(map[string]privateHandler),
			chains:          make(map[string]state.GossipStateProvider),
			leaderElection:  make(map[string]election.LeaderElectionService),
			deliveryService: make(map[string]deliverclient.DeliverService),
			deliveryFactory: factory,
			peerIdentity:    peerIdentity,
			secAdv:          secAdv,
		}
	})
	return errors.WithStack(err)
}

// GetGossipService returns an instance of gossip service
func GetGossipService() GossipService {
	return gossipServiceInstance
}

// DistributePrivateData distribute private read write set inside the channel based on the collections policies
//用于在channel中分发私有读写集
func (g *gossipServiceImpl) DistributePrivateData(chainID string, txID string, privData *rwset.TxPvtReadWriteSet) error {
	g.lock.RLock()//读锁定
	handler, exists := g.privateHandlers[chainID]
	g.lock.RUnlock()
	if !exists {
		return errors.Errorf("No private data handler for %s", chainID)
	}

	if err := handler.distributor.Distribute(txID, privData, handler.support.Cs); err != nil {
		logger.Error("Failed to distributed private collection, txID", txID, "channel", chainID, "due to", err)
		return err
	}

	if err := handler.coordinator.StorePvtData(txID, privData); err != nil {
		logger.Error("Failed to store private data into transient store, txID",
			txID, "channel", chainID, "due to", err)
		return err
	}
	return nil
}

// NewConfigEventer creates a ConfigProcessor which the channelconfig.BundleSource can ultimately route config updates to
func (g *gossipServiceImpl) NewConfigEventer() ConfigProcessor {
	return newConfigEventer(g)
}

// Support aggregates functionality of several
// interfaces required by gossip service
type Support struct {
	Validator txvalidator.Validator
	Committer committer.Committer
	Store     privdata2.TransientStore
	Cs        privdata.CollectionStore
}

// DataStoreSupport aggregates interfaces capable
// of handling either incoming blocks or private data
type DataStoreSupport struct {
	committer.Committer
	privdata2.TransientStore
}

// InitializeChannel allocates the state provider and should be invoked once per channel per execution
/*
   每一个channel的gossipStateProvider都在InitializeChannel方法中被初始化，而InitializeChannel进一步调用了
 */
func (g *gossipServiceImpl) InitializeChannel(chainID string, endpoints []string, support Support) {
	g.lock.Lock() //写锁定
	defer g.lock.Unlock()
	// Initialize new state provider for given committer
	logger.Debug("Creating state provider for chainID", chainID)
	servicesAdapter := &state.ServicesMediator{GossipAdapter: g, MCSAdapter: g.mcs}

	// Embed transient store and committer APIs to fulfill
	// DataStore interface to capture ability of retrieving
	// private data
	storeSupport := &DataStoreSupport{
		TransientStore: support.Store,
		Committer:      support.Committer,
	}
	// Initialize private data fetcher
	dataRetriever := privdata2.NewDataRetriever(storeSupport)
	fetcher := privdata2.NewPuller(support.Cs, g.gossipSvc, dataRetriever, chainID)

	coordinator := privdata2.NewCoordinator(privdata2.Support{
		CollectionStore: support.Cs,
		Validator:       support.Validator,
		TransientStore:  support.Store,
		Committer:       support.Committer,
		Fetcher:         fetcher,
	}, g.createSelfSignedData())

	g.privateHandlers[chainID] = privateHandler{
		support:     support,
		coordinator: coordinator,
		distributor: privdata2.NewDistributor(chainID, g),
	}
	//
	g.chains[chainID] = state.NewGossipStateProvider(chainID, servicesAdapter, coordinator)
	//首先判断peer在该channel是否已经存在deliveryService实例。如果为空
	if g.deliveryService[chainID] == nil {
		var err error
		//如果是为空，则调用Gossip服务中deliveryFactory模块下Service方法，新建一个delivery client的实例
		g.deliveryService[chainID], err = g.deliveryFactory.Service(g, endpoints, g.mcs)
		if err != nil {
			logger.Warningf("Cannot create delivery client, due to %+v", errors.WithStack(err))
		}
	}

	// Delivery service might be nil only if it was not able to get connected
	// to the ordering service

	if g.deliveryService[chainID] != nil {
		// Parameters:
		//              - peer.gossip.useLeaderElection
		//              - peer.gossip.orgLeader
		//
		// are mutual exclusive, setting both to true is not defined, hence
		// peer will panic and terminate
		//首先读取core.yaml中的配置项。
		leaderElection := viper.GetBool("peer.gossip.useLeaderElection")
		//读取
		isStaticOrgLeader := viper.GetBool("peer.gossip.orgLeader")

		if leaderElection && isStaticOrgLeader {
			//同为true则抛出异常
			logger.Panic("Setting both orgLeader and useLeaderElection to true isn't supported, aborting execution")
		}

		//Leader初始化
		if leaderElection {
			//
			logger.Debug("Delivery uses dynamic leader election mechanism, channel", chainID)
			g.leaderElection[chainID] = g.newLeaderElectionComponent(chainID, g.onStatusChangeFactory(chainID, support.Committer))
		} else if isStaticOrgLeader {
			/*
			  如果orgLeader为TRUE，则该peer，即对于当前channel代表该Org与Ordering Service进行通信拉取区块的leader peer
			 */
			logger.Debug("This peer is configured to connect to ordering service for blocks delivery, channel", chainID)

			g.deliveryService[chainID].StartDeliverForChannel(chainID, support.Committer, func() {})
		} else {
			logger.Debug("This peer is not configured to connect to ordering service for blocks delivery, channel", chainID)
		}
	} else {
		logger.Warning("Delivery client is down won't be able to pull blocks for chain", chainID)
	}
}

func (g *gossipServiceImpl) createSelfSignedData() common.SignedData {
	msg := make([]byte, 32)
	sig, err := g.mcs.Sign(msg)
	if err != nil {
		logger.Panicf("Failed creating self signed data because message signing failed: %v", err)
	}
	return common.SignedData{
		Data:      msg,
		Signature: sig,
		Identity:  g.peerIdentity,
	}
}

// updateAnchors constructs a joinChannelMessage and sends it to the gossipSvc
func (g *gossipServiceImpl) updateAnchors(config Config) {
	myOrg := string(g.secAdv.OrgByPeerIdentity(api.PeerIdentityType(g.peerIdentity)))
	if !g.amIinChannel(myOrg, config) {
		logger.Error("Tried joining channel", config.ChainID(), "but our org(", myOrg, "), isn't "+
			"among the orgs of the channel:", orgListFromConfig(config), ", aborting.")
		return
	}
	jcm := &joinChannelMessage{seqNum: config.Sequence(), members2AnchorPeers: map[string][]api.AnchorPeer{}}
	for _, appOrg := range config.Organizations() {
		logger.Debug(appOrg.MSPID(), "anchor peers:", appOrg.AnchorPeers())
		jcm.members2AnchorPeers[appOrg.MSPID()] = []api.AnchorPeer{}
		for _, ap := range appOrg.AnchorPeers() {
			anchorPeer := api.AnchorPeer{
				Host: ap.Host,
				Port: int(ap.Port),
			}
			jcm.members2AnchorPeers[appOrg.MSPID()] = append(jcm.members2AnchorPeers[appOrg.MSPID()], anchorPeer)
		}
	}

	// Initialize new state provider for given committer
	logger.Debug("Creating state provider for chainID", config.ChainID())
	g.JoinChan(jcm, gossipCommon.ChainID(config.ChainID()))
}

func (g *gossipServiceImpl) updateEndpoints(chainID string, endpoints []string) {
	if ds, ok := g.deliveryService[chainID]; ok {
		logger.Debugf("Updating endpoints for chainID", chainID)
		if err := ds.UpdateEndpoints(chainID, endpoints); err != nil {
			// The only reason to fail is because of absence of block provider
			// for given channel id, hence printing a warning will be enough
			logger.Warningf("Failed to update ordering service endpoints, due to %s", err)
		}
	}
}

// AddPayload appends message payload to for given chain
/*
    用于将message payload添加到给定的链
 */
func (g *gossipServiceImpl) AddPayload(chainID string, payload *gproto.Payload) error {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.chains[chainID].AddPayload(payload)
}

// Stop stops the gossip component
func (g *gossipServiceImpl) Stop() {
	g.lock.Lock() //写锁定
	defer g.lock.Unlock()

	for chainID := range g.chains {
		logger.Info("Stopping chain", chainID)
		if le, exists := g.leaderElection[chainID]; exists {
			logger.Infof("Stopping leader election for %s", chainID)
			le.Stop()
		}
		g.chains[chainID].Stop()
		g.privateHandlers[chainID].close()

		if g.deliveryService[chainID] != nil {
			g.deliveryService[chainID].Stop()
		}
	}
	g.gossipSvc.Stop()
}

func (g *gossipServiceImpl) newLeaderElectionComponent(chainID string, callback func(bool)) election.LeaderElectionService {
	/*
	    获取当前peer的Identity并初始化一个leader选举适配器
	 */
	PKIid := g.mcs.GetPKIidOfCert(g.peerIdentity)
	adapter := election.NewAdapter(g, PKIid, gossipCommon.ChainID(chainID))
	/*
	  然后在进一步调用
	 */
	return election.NewLeaderElectionService(adapter, string(PKIid), callback)
}

func (g *gossipServiceImpl) amIinChannel(myOrg string, config Config) bool {
	for _, orgName := range orgListFromConfig(config) {
		if orgName == myOrg {
			return true
		}
	}
	return false
}

func (g *gossipServiceImpl) onStatusChangeFactory(chainID string, committer blocksprovider.LedgerInfo) func(bool) {
	return func(isLeader bool) {
		if isLeader {
			yield := func() {
				g.lock.RLock()
				le := g.leaderElection[chainID]
				g.lock.RUnlock()
				le.Yield()
			}
			logger.Info("Elected as a leader, starting delivery service for channel", chainID)
			if err := g.deliveryService[chainID].StartDeliverForChannel(chainID, committer, yield); err != nil {
				logger.Errorf("Delivery service is not able to start blocks delivery for chain, due to %+v", errors.WithStack(err))
			}
		} else {
			logger.Info("Renounced leadership, stopping delivery service for channel", chainID)
			if err := g.deliveryService[chainID].StopDeliverForChannel(chainID); err != nil {
				logger.Errorf("Delivery service is not able to stop blocks delivery for chain, due to %+v", errors.WithStack(err))
			}

		}

	}
}

func orgListFromConfig(config Config) []string {
	var orgList []string
	for _, appOrg := range config.Organizations() {
		orgList = append(orgList, appOrg.MSPID())
	}
	return orgList
}
