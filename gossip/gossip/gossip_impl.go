/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package gossip

import (
	"bytes"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hyperledger/fabric/gossip/api"
	"github.com/hyperledger/fabric/gossip/comm"
	"github.com/hyperledger/fabric/gossip/common"
	"github.com/hyperledger/fabric/gossip/discovery"
	"github.com/hyperledger/fabric/gossip/filter"
	"github.com/hyperledger/fabric/gossip/gossip/channel"
	"github.com/hyperledger/fabric/gossip/gossip/msgstore"
	"github.com/hyperledger/fabric/gossip/gossip/pull"
	"github.com/hyperledger/fabric/gossip/identity"
	"github.com/hyperledger/fabric/gossip/util"
	proto "github.com/hyperledger/fabric/protos/gossip"
	"github.com/op/go-logging"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

const (
	presumedDeadChanSize = 100
	acceptChanSize       = 100
)

type channelRoutingFilterFactory func(channel.GossipChannel) filter.RoutingFilter

/*
  type Gossip interface的实现
 */
type gossipServiceImpl struct {
	selfIdentity          api.PeerIdentityType //该peer自己的Identity
	includeIdentityPeriod time.Time  //自peer启动时将自己的certificate包含在Alive message之中的持续时间，默认为10s
	certStore             *certStore //用来处理与peer identity有关的消息
	idMapper              identity.Mapper //维护peers的pkiID与certificate之间的映射
	presumedDead          chan common.PKIidType //存储疑似dead的peer的pikID的通道，默认大小为100
	disc                  discovery.Discovery   //discovery模块，用来维护当前网络视图

	comm                  comm.Comm //Comm模块，用来与同样嵌入了Comm模块的其他peer通信
	incTime               time.Time
	selfOrg               api.OrgIdentityType //peer 自己所属Org的identity
	*comm.ChannelDeMultiplexer //一个用来你接收channel注册（AddChannel）与发布DeMultiplex的结构体
	logger            *logging.Logger
	stopSignal        *sync.WaitGroup
	conf              *Config    //Gossip服务的相关配置
	toDieChan         chan struct{} //用来标识Gossip服务是否停止通道，默认大小为1
	stopFlag          int32         //Gossip服务是否停止的标识
	emitter           batchingEmitter //emitter模块，用来发送message
	discAdapter       *discoveryAdapter //discovery适配器，用来为discovery模块中声明的comm接口的所需功能
	secAdvisor        api.SecurityAdvisor //SecurityAdvisor定义了一个外部的辅助对象，提供安全和审核相关
	chanState         *channelState //维护peer所加入的channel消息
	disSecAdap        *discoverySecurityAdapter  //discovery安全适配器，用来验证Alive message等
	mcs               api.MessageCryptoService //MessageCryptoService是在gossip组件与peer加密层之间的协议（contract）;messageCryptoService是被用来验证与授权远程peer与他们所发送的数据，也用来验证从
	//从Ordering Service收到的区块
	stateInfoMsgStore msgstore.MessageStore //消息存储，当接收到一个message的时候，将message放入一内部缓存
	certPuller        pull.Mediator //是一个包装PullEngine的组件，它提供了执行pull同步所需的方法
}

// NewGossipService creates a gossip instance attached to a gRPC server
//gossipSvc
func NewGossipService(conf *Config, s *grpc.Server, secAdvisor api.SecurityAdvisor,
	mcs api.MessageCryptoService, selfIdentity api.PeerIdentityType,
	secureDialOpts api.PeerSecureDialOpts) Gossip {
	var err error

	lgr := util.GetLogger(util.LoggingGossipModule, conf.ID)

	g := &gossipServiceImpl{
		selfOrg:               secAdvisor.OrgByPeerIdentity(selfIdentity),
		secAdvisor:            secAdvisor,
		selfIdentity:          selfIdentity,
		presumedDead:          make(chan common.PKIidType, presumedDeadChanSize),
		disc:                  nil,
		mcs:                   mcs,
		conf:                  conf,
		ChannelDeMultiplexer:  comm.NewChannelDemultiplexer(),
		logger:                lgr,
		toDieChan:             make(chan struct{}, 1),
		stopFlag:              int32(0),
		stopSignal:            &sync.WaitGroup{},
		includeIdentityPeriod: time.Now().Add(conf.PublishCertPeriod),
	}


	g.stateInfoMsgStore = g.newStateInfoMsgStore()

	g.idMapper = identity.NewIdentityMapper(mcs, selfIdentity, func(pkiID common.PKIidType, identity api.PeerIdentityType) {
		g.comm.CloseConn(&comm.RemotePeer{PKIID: pkiID})
		g.certPuller.Remove(string(pkiID))
	})

	if s == nil {
		g.comm, err = createCommWithServer(conf.BindPort, g.idMapper, selfIdentity, secureDialOpts)
	} else {
		g.comm, err = createCommWithoutServer(s, conf.TLSCerts, g.idMapper, selfIdentity, secureDialOpts)
	}

	if err != nil {
		lgr.Error("Failed instntiating communication layer:", err)
		return nil
	}

	g.chanState = newChannelState(g)
	g.emitter = newBatchingEmitter(conf.PropagateIterations,
		conf.MaxPropagationBurstSize, conf.MaxPropagationBurstLatency,
		g.sendGossipBatch)

	g.discAdapter = g.newDiscoveryAdapter()
	g.disSecAdap = g.newDiscoverySecurityAdapter()
	/*
	    周期性地将打包好的message传给回调函数gossipBatch以发送
	 */
	g.disc = discovery.NewDiscoveryService(g.selfNetworkMember(), g.discAdapter, g.disSecAdap, g.disclosurePolicy)



	g.logger.Info("Creating gossip service with self membership of", g.selfNetworkMember())

	/*

	 */
	g.certPuller = g.createCertStorePuller()
	/*
	  这个模块的名称可以看出模块的功能，即该模块可以同步peers之间各个peer（所持有自己或者其他peer）的identity（certificate），gossipSvc的certPuller
	  模块与certStore模块是搭配使用的，刚刚初始化的certPuller模块是certStore模块的一部分，
	 */
	g.certStore = newCertStore(g.certPuller, g.idMapper, selfIdentity, mcs)

	if g.conf.ExternalEndpoint == "" {
		g.logger.Warning("External endpoint is empty, peer will not be accessible outside of its organization")
	}

	go g.start()

	go g.connect2BootstrapPeers()

	return g
}

func (g *gossipServiceImpl) newStateInfoMsgStore() msgstore.MessageStore {
	pol := proto.NewGossipMessageComparator(0)
	return msgstore.NewMessageStoreExpirable(pol,
		msgstore.Noop,
		g.conf.PublishStateInfoInterval*100,
		nil,
		nil,
		msgstore.Noop)
}

func (g *gossipServiceImpl) selfNetworkMember() discovery.NetworkMember {
	self := discovery.NetworkMember{
		Endpoint:         g.conf.ExternalEndpoint,
		PKIid:            g.comm.GetPKIid(),
		Metadata:         []byte{},
		InternalEndpoint: g.conf.InternalEndpoint,
	}
	if g.disc != nil {
		self.Metadata = g.disc.Self().Metadata
	}
	return self
}

func newChannelState(g *gossipServiceImpl) *channelState {
	return &channelState{
		stopping: int32(0),
		channels: make(map[string]channel.GossipChannel),
		g:        g,
	}
}

func createCommWithoutServer(s *grpc.Server, certs *common.TLSCertificates, idStore identity.Mapper,
	identity api.PeerIdentityType, secureDialOpts api.PeerSecureDialOpts) (comm.Comm, error) {
	return comm.NewCommInstance(s, certs, idStore, identity, secureDialOpts)
}

// NewGossipServiceWithServer creates a new gossip instance with a gRPC server
func NewGossipServiceWithServer(conf *Config, secAdvisor api.SecurityAdvisor, mcs api.MessageCryptoService,
	identity api.PeerIdentityType, secureDialOpts api.PeerSecureDialOpts) Gossip {
	return NewGossipService(conf, nil, secAdvisor, mcs, identity, secureDialOpts)
}

func createCommWithServer(port int, idStore identity.Mapper, identity api.PeerIdentityType,
	secureDialOpts api.PeerSecureDialOpts) (comm.Comm, error) {
	return comm.NewCommInstanceWithServer(port, idStore, identity, secureDialOpts)
}

func (g *gossipServiceImpl) toDie() bool {
	return atomic.LoadInt32(&g.stopFlag) == int32(1)
}

func (g *gossipServiceImpl) JoinChan(joinMsg api.JoinChannelMessage, chainID common.ChainID) {
	// joinMsg is supposed to have been already verified
	g.chanState.joinChannel(joinMsg, chainID)

	for _, org := range joinMsg.Members() {
		g.learnAnchorPeers(org, joinMsg.AnchorPeersOf(org))
	}
}

func (g *gossipServiceImpl) LeaveChan(chainID common.ChainID) {
	gc := g.chanState.getGossipChannelByChainID(chainID)
	if gc == nil {
		g.logger.Debug("No such channel", chainID)
		return
	}
	b, _ := (&common.NodeMetastate{}).Bytes()
	stateInfMsg, err := g.createStateInfoMsg(b, chainID, true)
	if err != nil {
		g.logger.Errorf("Failed creating StateInfo message: %+v", errors.WithStack(err))
		return
	}
	gc.UpdateStateInfo(stateInfMsg)
	gc.LeaveChannel()
}

// SuspectPeers makes the gossip instance validate identities of suspected peers, and close
// any connections to peers with identities that are found invalid
func (g *gossipServiceImpl) SuspectPeers(isSuspected api.PeerSuspector) {
	g.certStore.suspectPeers(isSuspected)
}

func (g *gossipServiceImpl) periodicalIdentityValidation(suspectFunc api.PeerSuspector, interval time.Duration) {
	for {
		select {
		case s := <-g.toDieChan:
			g.toDieChan <- s
			return
		case <-time.After(interval):
			g.SuspectPeers(suspectFunc)
		}
	}
}

func (g *gossipServiceImpl) learnAnchorPeers(orgOfAnchorPeers api.OrgIdentityType, anchorPeers []api.AnchorPeer) {
	for _, ap := range anchorPeers {
		if ap.Host == "" {
			g.logger.Warning("Got empty hostname, skipping connecting to anchor peer", ap)
			continue
		}
		if ap.Port == 0 {
			g.logger.Warning("Got invalid port (0), skipping connecting to anchor peer", ap)
			continue
		}
		endpoint := fmt.Sprintf("%s:%d", ap.Host, ap.Port)
		// Skip connecting to self
		if g.selfNetworkMember().Endpoint == endpoint || g.selfNetworkMember().InternalEndpoint == endpoint {
			g.logger.Info("Anchor peer with same endpoint, skipping connecting to myself")
			continue
		}

		inOurOrg := bytes.Equal(g.selfOrg, orgOfAnchorPeers)
		if !inOurOrg && g.selfNetworkMember().Endpoint == "" {
			g.logger.Infof("Anchor peer %s:%d isn't in our org(%v) and we have no external endpoint, skipping", ap.Host, ap.Port, string(orgOfAnchorPeers))
			continue
		}
		identifier := func() (*discovery.PeerIdentification, error) {
			remotePeerIdentity, err := g.comm.Handshake(&comm.RemotePeer{Endpoint: endpoint})
			if err != nil {
				err = errors.WithStack(err)
				g.logger.Warningf("Deep probe of %s failed: %+v", endpoint, err)
				return nil, err
			}
			isAnchorPeerInMyOrg := bytes.Equal(g.selfOrg, g.secAdvisor.OrgByPeerIdentity(remotePeerIdentity))
			if bytes.Equal(orgOfAnchorPeers, g.selfOrg) && !isAnchorPeerInMyOrg {
				err := errors.Errorf("Anchor peer %s isn't in our org, but is claimed to be", endpoint)
				g.logger.Warningf("%+v", err)
				return nil, err
			}
			pkiID := g.mcs.GetPKIidOfCert(remotePeerIdentity)
			if len(pkiID) == 0 {
				return nil, errors.Errorf("Wasn't able to extract PKI-ID of remote peer with identity of %v", remotePeerIdentity)
			}
			return &discovery.PeerIdentification{
				ID:      pkiID,
				SelfOrg: isAnchorPeerInMyOrg,
			}, nil
		}

		g.disc.Connect(discovery.NetworkMember{
			InternalEndpoint: endpoint, Endpoint: endpoint}, identifier)
	}
}

func (g *gossipServiceImpl) handlePresumedDead() {
	defer g.logger.Debug("Exiting")
	g.stopSignal.Add(1)
	defer g.stopSignal.Done()
	for {
		select {
		case s := <-g.toDieChan:
			g.toDieChan <- s
			return
		case deadEndpoint := <-g.comm.PresumedDead():
			g.presumedDead <- deadEndpoint
		}
	}
}

func (g *gossipServiceImpl) syncDiscovery() {
	g.logger.Debug("Entering discovery sync with interval", g.conf.PullInterval)
	defer g.logger.Debug("Exiting discovery sync loop")
	for !g.toDie() {
		g.disc.InitiateSync(g.conf.PullPeerNum)
		time.Sleep(g.conf.PullInterval)
	}
}

func (g *gossipServiceImpl) start() {
	go g.syncDiscovery() //peers间成员视图
	go g.handlePresumedDead() //处理dead的peer

	msgSelector := func(msg interface{}) bool {
		gMsg, isGossipMsg := msg.(proto.ReceivedMessage)
		if !isGossipMsg {
			return false
		}

		isConn := gMsg.GetGossipMessage().GetConn() != nil
		isEmpty := gMsg.GetGossipMessage().GetEmpty() != nil
		isPrivateData := gMsg.GetGossipMessage().IsPrivateDataMsg()

		return !(isConn || isEmpty || isPrivateData)
	}

	//
	incMsgs := g.comm.Accept(msgSelector)

	//处理收到的messages
	go g.acceptMessages(incMsgs)

	g.logger.Info("Gossip instance", g.conf.ID, "started")
}

func (g *gossipServiceImpl) acceptMessages(incMsgs <-chan proto.ReceivedMessage) {
	defer g.logger.Debug("Exiting")
	g.stopSignal.Add(1)
	defer g.stopSignal.Done()
	for {
		select {
		case s := <-g.toDieChan:
			g.toDieChan <- s
			return
		case msg := <-incMsgs:
			g.handleMessage(msg)
		}
	}
}

func (g *gossipServiceImpl) handleMessage(m proto.ReceivedMessage) {
	if g.toDie() {
		return
	}

	if m == nil || m.GetGossipMessage() == nil {
		return
	}

	msg := m.GetGossipMessage()

	g.logger.Debug("Entering,", m.GetConnectionInfo(), "sent us", msg)
	defer g.logger.Debug("Exiting")

	if !g.validateMsg(m) {
		g.logger.Warning("Message", msg, "isn't valid")
		return
	}

	if msg.IsChannelRestricted() {
		if gc := g.chanState.lookupChannelForMsg(m); gc == nil {
			// If we're not in the channel, we should still forward to peers of our org
			// in case it's a StateInfo message
			if g.isInMyorg(discovery.NetworkMember{PKIid: m.GetConnectionInfo().ID}) && msg.IsStateInfoMsg() {
				if g.stateInfoMsgStore.Add(msg) {
					g.emitter.Add(&emittedGossipMessage{
						SignedGossipMessage: msg,
						filter:              m.GetConnectionInfo().ID.IsNotSameFilter,
					})
				}
			}
			if !g.toDie() {
				g.logger.Debug("No such channel", msg.Channel, "discarding message", msg)
			}
		} else {
			if m.GetGossipMessage().IsLeadershipMsg() {
				if err := g.validateLeadershipMessage(m.GetGossipMessage()); err != nil {
					g.logger.Warningf("Failed validating LeaderElection message: %+v", errors.WithStack(err))
					return
				}
			}
			gc.HandleMessage(m)
		}
		return
	}

	if selectOnlyDiscoveryMessages(m) {
		// It's a membership request, check its self information
		// matches the sender
		if m.GetGossipMessage().GetMemReq() != nil {
			sMsg, err := m.GetGossipMessage().GetMemReq().SelfInformation.ToGossipMessage()
			if err != nil {
				g.logger.Warningf("Got membership request with invalid selfInfo: %+v", errors.WithStack(err))
				return
			}
			if !sMsg.IsAliveMsg() {
				g.logger.Warning("Got membership request with selfInfo that isn't an AliveMessage")
				return
			}
			if !bytes.Equal(sMsg.GetAliveMsg().Membership.PkiId, m.GetConnectionInfo().ID) {
				g.logger.Warning("Got membership request with selfInfo that doesn't match the handshake")
				return
			}
		}
		g.forwardDiscoveryMsg(m)
	}

	if msg.IsPullMsg() && msg.GetPullMsgType() == proto.PullMsgType_IDENTITY_MSG {
		g.certStore.handleMessage(m)
	}
}

func (g *gossipServiceImpl) forwardDiscoveryMsg(msg proto.ReceivedMessage) {
	defer func() { // can be closed while shutting down
		recover()
	}()

	g.discAdapter.incChan <- msg
}

// validateMsg checks the signature of the message if exists,
// and also checks that the tag matches the message type
func (g *gossipServiceImpl) validateMsg(msg proto.ReceivedMessage) bool {
	if err := msg.GetGossipMessage().IsTagLegal(); err != nil {
		g.logger.Warningf("Tag of %v isn't legal:", msg.GetGossipMessage(), errors.WithStack(err))
		return false
	}

	if msg.GetGossipMessage().IsAliveMsg() {
		if !g.disSecAdap.ValidateAliveMsg(msg.GetGossipMessage()) {
			return false
		}
	}

	if msg.GetGossipMessage().IsStateInfoMsg() {
		if err := g.validateStateInfoMsg(msg.GetGossipMessage()); err != nil {
			g.logger.Warningf("StateInfo message %v is found invalid: %v", msg, err)
			return false
		}
	}
	return true
}

func (g *gossipServiceImpl) sendGossipBatch(a []interface{}) {
	msgs2Gossip := make([]*emittedGossipMessage, len(a))
	for i, e := range a {
		msgs2Gossip[i] = e.(*emittedGossipMessage)
	}
	g.gossipBatch(msgs2Gossip)
}

// gossipBatch - This is the method that actually decides to which peers to gossip the message
// batch we possess.
// For efficiency, we first isolate all the messages that have the same routing policy
// and send them together, and only after that move to the next group of messages.
// i.e: we send all blocks of channel C to the same group of peers,
// and send all StateInfo messages to the same group of peers, etc. etc.
// When we send blocks, we send only to peers that advertised themselves in the channel.
// When we send StateInfo messages, we send to peers in the channel.
// When we send messages that are marked to be sent only within the org, we send all of these messages
// to the same set of peers.
// The rest of the messages that have no restrictions on their destinations can be sent
// to any group of peers.
/*
   1.blocks
   2.stateInfoMsg
   3.orgMsgs
   4.leadershipMsgs
   5.others 其他
 */
func (g *gossipServiceImpl) gossipBatch(msgs []*emittedGossipMessage) {
	if g.disc == nil {
		g.logger.Error("Discovery has not been initialized yet, aborting!")
		return
	}

	//定义的谓词
	var blocks []*emittedGossipMessage  //判断是否为DataMesage
	var stateInfoMsgs []*emittedGossipMessage //stateInfoMsgs的处理，首先要遍历stateInfoMsgs。对于每一条stateInfoMsg，首先要判给出的peer是否属于本Org
	//然后通过过去发送的stateInfoMsgs查看stateInfo所在的channel，如果这个channel存在且存在external endpoint，则需要哦判断给出的peer是否是这个channel的成员
	//然后通过remote peers过滤器刷选出要发送的peers，将stateInfoMsg进行发送。三个基本过滤器分别是：
	/*
	   1.g.conf.PropagatePeerNum 要发送的节点个数，
	   2.g.disc.GetMembership() 网络存活的成员
	   3.peerSelector 判断节点是否在channel上
	 */
	var orgMsgs []*emittedGossipMessage //对于stateInfoMsgs的处理过程，首先需要遍历stateInfoMsg，首先会判断给出的peer是否属于本Org，然后通过过去发送的
	/*
	   通过remote peers过滤器刷选出要发送的peers，三个基本过滤器分别是：
			g.conf.PropagatePeerNum 要发送的节点个数（有由core.yaml文件下的peer.gossip.PropagatePeerNum配置项决定
	        g.disc.GetMembership() 网络中存活的成员
	        g.isInMyorg 判断节点是否在组织内
	    然后对orgMsgs进行遍历，逐条发送给刷选出的peer节点
	 */
	//stateInfoMsgs查看stateInfo所在的channel。如果这个channel存在且有
	var leadershipMsgs []*emittedGossipMessage //对于leadershipMsgs的处理过程与blocks消息基本一致，区别在于，blocks消息最多只会发送3个remote peers
	//但是对于LeadershipMsgs消息，gossipInChan会将消息发送给所有满足过滤条件的remote peers。这也容易理解，因为leader选举与声明需要Org中的全体成员参与的.
	isABlock := func(o interface{}) bool {
		return o.(*emittedGossipMessage).IsDataMsg() //判断消息类型是否为DataMessage
	}
	isAStateInfoMsg := func(o interface{}) bool {
		return o.(*emittedGossipMessage).IsStateInfoMsg()  //判断是否是stateInfo
	}
	aliveMsgsWithNoEndpointAndInOurOrg := func(o interface{}) bool {
		msg := o.(*emittedGossipMessage)
		if !msg.IsAliveMsg() {
			return false
		}
		member := msg.GetAliveMsg().Membership
		return member.Endpoint == "" && g.isInMyorg(discovery.NetworkMember{PKIid: member.PkiId})
	} //判断该消息是否是aliveMsg，判断发送该aliveMsg的remote peer会否有
	//external endpoint ，判断该remote peer是否属于当前组织，
	isOrgRestricted := func(o interface{}) bool {
		return aliveMsgsWithNoEndpointAndInOurOrg(o) || o.(*emittedGossipMessage).IsOrgRestricted()
	}//判断消息是否满足aliveMsgsWithNoEndpointAndInOurOrg或者IsOrgRestricted判断消息是否只应该在组织内传播
	isLeadershipMsg := func(o interface{}) bool {
		return o.(*emittedGossipMessage).IsLeadershipMsg()  //判断消息或否为LeadershipMsg,故消息类别的解析也会有顺序限制的：
		//blocks ->　leadershipMsg -> stateInfoMsgs => orgMsg -> others
	}

	// Gossip blocks
	//定以了五中消息谓词，通过定义的谓词，结合通文件下的partitionMessages函数实现对消息类别的解析，定义的谓词：
	blocks, msgs = partitionMessages(isABlock, msgs)
	//从要发送的msgs切片中，解析出几类消息，然后通过设定的过滤条件，从当前视图中刷选出有资格节后欧特定种类的消息remote peers,然后将消息发送

	//调用gossipInChan方法来发送blocks消息。blocks消息切片 和remote peers--过滤器。remote peers过滤器是通过利用
	//gossip/fiilter/filter.go文件下的CombineRoutingFilters函数，将三个基本过滤网通过逻辑组合而成的，三个基本过滤器分别是：
	/*
	 1.gc.EligibleForChannel 。判断一个给定的peer是否有资格获得该channel下的区块
	 2.gc.IsMemberInChan 判断一个给定的peer是否是该channel的成员
	 3.g.isInMyorg. 判断一个给定的peer是否属于本Org
	 */
	g.gossipInChan(blocks, func(gc channel.GossipChannel) filter.RoutingFilter {
		return filter.CombineRoutingFilters(gc.EligibleForChannel, gc.IsMemberInChan, g.isInMyorg)
	})

	// Gossip Leadership messages
	leadershipMsgs, msgs = partitionMessages(isLeadershipMsg, msgs)
	g.gossipInChan(leadershipMsgs, func(gc channel.GossipChannel) filter.RoutingFilter {
		return filter.CombineRoutingFilters(gc.EligibleForChannel, gc.IsMemberInChan, g.isInMyorg)
	})

	// Gossip StateInfo messages
	stateInfoMsgs, msgs = partitionMessages(isAStateInfoMsg, msgs)
	for _, stateInfMsg := range stateInfoMsgs {
		peerSelector := g.isInMyorg
		gc := g.chanState.lookupChannelForGossipMsg(stateInfMsg.GossipMessage)
		if gc != nil && g.hasExternalEndpoint(stateInfMsg.GossipMessage.GetStateInfo().PkiId) {
			peerSelector = gc.IsMemberInChan
		}

		peerSelector = filter.CombineRoutingFilters(peerSelector, func(member discovery.NetworkMember) bool {
			return stateInfMsg.filter(member.PKIid)
		})

		peers2Send := filter.SelectPeers(g.conf.PropagatePeerNum, g.disc.GetMembership(), peerSelector)
		g.comm.Send(stateInfMsg.SignedGossipMessage, peers2Send...)
	}

	// Gossip messages restricted to our org
	orgMsgs, msgs = partitionMessages(isOrgRestricted, msgs)
	peers2Send := filter.SelectPeers(g.conf.PropagatePeerNum, g.disc.GetMembership(), g.isInMyorg)
	for _, msg := range orgMsgs {
		g.comm.Send(msg.SignedGossipMessage, g.removeSelfLoop(msg, peers2Send)...)
	}

	// Finally, gossip the remaining messages
	for _, msg := range msgs {
		if !msg.IsAliveMsg() {
			g.logger.Error("Unknown message type", msg)
			continue
		}
		selectByOriginOrg := g.peersByOriginOrgPolicy(discovery.NetworkMember{PKIid: msg.GetAliveMsg().Membership.PkiId})
		selector := filter.CombineRoutingFilters(selectByOriginOrg, func(member discovery.NetworkMember) bool {
			return msg.filter(member.PKIid)
		})
		peers2Send := filter.SelectPeers(g.conf.PropagatePeerNum, g.disc.GetMembership(), selector)
		g.sendAndFilterSecrets(msg.SignedGossipMessage, peers2Send...)
	}
}

func (g *gossipServiceImpl) sendAndFilterSecrets(msg *proto.SignedGossipMessage, peers ...*comm.RemotePeer) {
	for _, peer := range peers {
		// Prevent forwarding alive messages of external organizations
		// to peers that have no external endpoints
		aliveMsgFromDiffOrg := msg.IsAliveMsg() && !g.isInMyorg(discovery.NetworkMember{PKIid: msg.GetAliveMsg().Membership.PkiId})
		if aliveMsgFromDiffOrg && !g.hasExternalEndpoint(peer.PKIID) {
			continue
		}
		// Don't gossip secrets
		if !g.isInMyorg(discovery.NetworkMember{PKIid: peer.PKIID}) {
			msg.Envelope.SecretEnvelope = nil
		}

		g.comm.Send(msg, peer)
	}
}

// gossipInChan gossips a given GossipMessage slice according to a channel's routing policy.
func (g *gossipServiceImpl) gossipInChan(messages []*emittedGossipMessage, chanRoutingFactory channelRoutingFilterFactory) {
	if len(messages) == 0 {
		return
	}
	//对于blocks切片中的各个信息，提取器所属channel信息，将blocks按所属channel归类，对于每一类block，刷选出有资格接收该block的remote peers，
	//并从刷选出的remote peers中随机选出最多3个（core.yaml peer.gossip.PropagatePeerNum配置选项决定），将block发送给这些peers,
	//特别地，还会将某一msg的发送者sender，从peers的待发送列表中过滤掉，以免造成消息发送的循环。
	totalChannels := extractChannels(messages)
	var channel common.ChainID
	var messagesOfChannel []*emittedGossipMessage
	for len(totalChannels) > 0 {
		// Take first channel
		channel, totalChannels = totalChannels[0], totalChannels[1:]
		// Extract all messages of that channel
		grabMsgs := func(o interface{}) bool {
			return bytes.Equal(o.(*emittedGossipMessage).Channel, channel)
		}
		messagesOfChannel, messages = partitionMessages(grabMsgs, messages)
		if len(messagesOfChannel) == 0 {
			continue
		}
		// Grab channel object for that channel
		gc := g.chanState.getGossipChannelByChainID(channel)
		if gc == nil {
			g.logger.Warning("Channel", channel, "wasn't found")
			continue
		}
		// Select the peers to send the messages to
		// For leadership messages we will select all peers that pass routing factory - e.g. all peers in channel and org
		membership := g.disc.GetMembership()
		var peers2Send []*comm.RemotePeer
		if messagesOfChannel[0].IsLeadershipMsg() {
			peers2Send = filter.SelectPeers(len(membership), membership, chanRoutingFactory(gc))
		} else {
			peers2Send = filter.SelectPeers(g.conf.PropagatePeerNum, membership, chanRoutingFactory(gc))
		}

		// Send the messages to the remote peers
		for _, msg := range messagesOfChannel {
			filteredPeers := g.removeSelfLoop(msg, peers2Send)
			g.comm.Send(msg.SignedGossipMessage, filteredPeers...)
		}
	}
}

// removeSelfLoop deletes from the list of peers peer which has sent the message
func (g *gossipServiceImpl) removeSelfLoop(msg *emittedGossipMessage, peers []*comm.RemotePeer) []*comm.RemotePeer {
	var result []*comm.RemotePeer
	for _, peer := range peers {
		if msg.filter(peer.PKIID) {
			result = append(result, peer)
		}
	}
	return result
}

// SendByCriteria sends a given message to all peers that match the given SendCriteria
func (g *gossipServiceImpl) SendByCriteria(msg *proto.SignedGossipMessage, criteria SendCriteria) error {
	if criteria.Timeout == 0 {
		return errors.New("Timeout should be specified")
	}

	if criteria.IsEligible == nil {
		criteria.IsEligible = filter.SelectAllPolicy
	}

	membership := g.disc.GetMembership()
	if criteria.MaxPeers == 0 {
		criteria.MaxPeers = len(membership)
	}

	if len(criteria.Channel) > 0 {
		gc := g.chanState.getGossipChannelByChainID(criteria.Channel)
		if gc == nil {
			return fmt.Errorf("Requested to Send for channel %s, but no such channel exists", string(criteria.Channel))
		}
		membership = gc.GetPeers()
	}

	peers2send := filter.SelectPeers(criteria.MaxPeers, membership, criteria.IsEligible)
	if len(peers2send) < criteria.MinAck {
		return fmt.Errorf("Requested to send to at least %d peers, but know only of %d suitable peers", criteria.MinAck, len(peers2send))
	}

	results := g.comm.SendWithAck(msg, criteria.Timeout, criteria.MinAck, peers2send...)

	for _, res := range results {
		if res.Error() == "" {
			continue
		}
		g.logger.Warning("Failed sending to", res.Endpoint, "error:", res.Error())
	}

	if results.AckCount() < criteria.MinAck {
		return errors.New(results.String())
	}
	return nil
}

// Gossip sends a message to other peers to the network
func (g *gossipServiceImpl) Gossip(msg *proto.GossipMessage) {
	// Educate developers to Gossip messages with the right tags.
	// See IsTagLegal() for wanted behavior.
	if err := msg.IsTagLegal(); err != nil {
		panic(errors.WithStack(err))
	}

	sMsg := &proto.SignedGossipMessage{
		GossipMessage: msg,
	}

	var err error
	if sMsg.IsDataMsg() {
		sMsg, err = sMsg.NoopSign()
	} else {
		_, err = sMsg.Sign(func(msg []byte) ([]byte, error) {
			return g.mcs.Sign(msg)
		})
	}

	if err != nil {
		g.logger.Warningf("Failed signing message: %+v", errors.WithStack(err))
		return
	}

	if msg.IsChannelRestricted() {
		gc := g.chanState.getGossipChannelByChainID(msg.Channel)
		if gc == nil {
			g.logger.Warning("Failed obtaining gossipChannel of", msg.Channel, "aborting")
			return
		}
		if msg.IsDataMsg() {
			gc.AddToMsgStore(sMsg)
		}
	}

	if g.conf.PropagateIterations == 0 {
		return
	}
	g.emitter.Add(&emittedGossipMessage{
		SignedGossipMessage: sMsg,
		filter: func(_ common.PKIidType) bool {
			return true
		},
	})
}

// Send sends a message to remote peers
func (g *gossipServiceImpl) Send(msg *proto.GossipMessage, peers ...*comm.RemotePeer) {
	m, err := msg.NoopSign()
	if err != nil {
		g.logger.Warningf("Failed creating SignedGossipMessage: %+v", errors.WithStack(err))
		return
	}
	g.comm.Send(m, peers...)
}

// GetPeers returns a mapping of endpoint --> []discovery.NetworkMember
func (g *gossipServiceImpl) Peers() []discovery.NetworkMember {
	return g.disc.GetMembership()
}

// PeersOfChannel returns the NetworkMembers considered alive
// and also subscribed to the channel given
func (g *gossipServiceImpl) PeersOfChannel(channel common.ChainID) []discovery.NetworkMember {
	gc := g.chanState.getGossipChannelByChainID(channel)
	if gc == nil {
		g.logger.Debug("No such channel", channel)
		return nil
	}

	return gc.GetPeers()
}

// PeerFilter receives a SubChannelSelectionCriteria and returns a RoutingFilter that selects
// only peer identities that match the given criteria, and that they published their channel participation
func (g *gossipServiceImpl) PeerFilter(channel common.ChainID, messagePredicate api.SubChannelSelectionCriteria) (filter.RoutingFilter, error) {
	gc := g.chanState.getGossipChannelByChainID(channel)
	if gc == nil {
		return nil, errors.Errorf("Channel %s doesn't exist", string(channel))
	}
	return gc.PeerFilter(messagePredicate), nil
}

// Stop stops the gossip component
func (g *gossipServiceImpl) Stop() {
	if g.toDie() {
		return
	}
	atomic.StoreInt32(&g.stopFlag, int32(1))
	g.logger.Info("Stopping gossip")
	comWG := sync.WaitGroup{}
	comWG.Add(1)
	go func() {
		defer comWG.Done()
		g.comm.Stop()
	}()
	g.chanState.stop()
	g.discAdapter.close()
	g.disc.Stop()
	g.certStore.stop()
	g.toDieChan <- struct{}{}
	g.emitter.Stop()
	g.ChannelDeMultiplexer.Close()
	g.stateInfoMsgStore.Stop()
	g.stopSignal.Wait()
	comWG.Wait()
}

func (g *gossipServiceImpl) UpdateMetadata(md []byte) {
	g.disc.UpdateMetadata(md)
}

// UpdateChannelMetadata updates the self metadata the peer
// publishes to other peers about its channel-related state
func (g *gossipServiceImpl) UpdateChannelMetadata(md []byte, chainID common.ChainID) {
	gc := g.chanState.getGossipChannelByChainID(chainID)
	if gc == nil {
		g.logger.Debug("No such channel", chainID)
		return
	}
	stateInfMsg, err := g.createStateInfoMsg(md, chainID, false)
	if err != nil {
		g.logger.Errorf("Failed creating StateInfo message: %+v", errors.WithStack(err))
		return
	}
	gc.UpdateStateInfo(stateInfMsg)
}

// Accept returns a dedicated read-only channel for messages sent by other nodes that match a certain predicate.
// If passThrough is false, the messages are processed by the gossip layer beforehand.
// If passThrough is true, the gossip layer doesn't intervene and the messages
// can be used to send a reply back to the sender
func (g *gossipServiceImpl) Accept(acceptor common.MessageAcceptor, passThrough bool) (<-chan *proto.GossipMessage, <-chan proto.ReceivedMessage) {
	if passThrough {
		return nil, g.comm.Accept(acceptor)
	}
	acceptByType := func(o interface{}) bool {
		if o, isGossipMsg := o.(*proto.GossipMessage); isGossipMsg {
			return acceptor(o)
		}
		if o, isSignedMsg := o.(*proto.SignedGossipMessage); isSignedMsg {
			sMsg := o
			return acceptor(sMsg.GossipMessage)
		}
		g.logger.Warning("Message type:", reflect.TypeOf(o), "cannot be evaluated")
		return false
	}
	inCh := g.AddChannel(acceptByType)
	outCh := make(chan *proto.GossipMessage, acceptChanSize)
	go func() {
		for {
			select {
			case s := <-g.toDieChan:
				g.toDieChan <- s
				return
			case m := <-inCh:
				if m == nil {
					return
				}
				outCh <- m.(*proto.SignedGossipMessage).GossipMessage
			}
		}
	}()
	return outCh, nil
}

func selectOnlyDiscoveryMessages(m interface{}) bool {
	msg, isGossipMsg := m.(proto.ReceivedMessage)
	if !isGossipMsg {
		return false
	}
	alive := msg.GetGossipMessage().GetAliveMsg()
	memRes := msg.GetGossipMessage().GetMemRes()
	memReq := msg.GetGossipMessage().GetMemReq()

	selected := alive != nil || memReq != nil || memRes != nil

	return selected
}

func (g *gossipServiceImpl) newDiscoveryAdapter() *discoveryAdapter {
	return &discoveryAdapter{
		c:        g.comm,
		stopping: int32(0),
		gossipFunc: func(msg *proto.SignedGossipMessage) {
			if g.conf.PropagateIterations == 0 {
				return
			}
			g.emitter.Add(&emittedGossipMessage{
				SignedGossipMessage: msg,
				filter: func(_ common.PKIidType) bool {
					return true
				},
			})
		},
		forwardFunc: func(message proto.ReceivedMessage) {
			if g.conf.PropagateIterations == 0 {
				return
			}
			g.emitter.Add(&emittedGossipMessage{
				SignedGossipMessage: message.GetGossipMessage(),
				filter:              message.GetConnectionInfo().ID.IsNotSameFilter,
			})
		},
		incChan:          make(chan proto.ReceivedMessage),
		presumedDead:     g.presumedDead,
		disclosurePolicy: g.disclosurePolicy,
	}
}

// discoveryAdapter is used to supply the discovery module with needed abilities
// that the comm interface in the discovery module declares
type discoveryAdapter struct {
	stopping         int32
	c                comm.Comm
	presumedDead     chan common.PKIidType
	incChan          chan proto.ReceivedMessage
	gossipFunc       func(message *proto.SignedGossipMessage)
	forwardFunc      func(message proto.ReceivedMessage)
	disclosurePolicy discovery.DisclosurePolicy
}

func (da *discoveryAdapter) close() {
	atomic.StoreInt32(&da.stopping, int32(1))
	close(da.incChan)
}

func (da *discoveryAdapter) toDie() bool {
	return atomic.LoadInt32(&da.stopping) == int32(1)
}

func (da *discoveryAdapter) Gossip(msg *proto.SignedGossipMessage) {
	if da.toDie() {
		return
	}

	da.gossipFunc(msg)
}

func (da *discoveryAdapter) Forward(msg proto.ReceivedMessage) {
	if da.toDie() {
		return
	}

	da.forwardFunc(msg)
}

func (da *discoveryAdapter) SendToPeer(peer *discovery.NetworkMember, msg *proto.SignedGossipMessage) {
	if da.toDie() {
		return
	}
	// Check membership requests for peers that we know of their PKI-ID.
	// The only peers we don't know about their PKI-IDs are bootstrap peers.
	if memReq := msg.GetMemReq(); memReq != nil && len(peer.PKIid) != 0 {
		selfMsg, err := memReq.SelfInformation.ToGossipMessage()
		if err != nil {
			// Shouldn't happen
			panic(errors.Wrapf(err, "Tried to send a membership request with a malformed AliveMessage"))
		}
		// Apply the EnvelopeFilter of the disclosure policy
		// on the alive message of the selfInfo field of the membership request
		_, omitConcealedFields := da.disclosurePolicy(peer)
		selfMsg.Envelope = omitConcealedFields(selfMsg)
		// Backup old known field
		oldKnown := memReq.Known
		// Override new SelfInfo message with updated envelope
		memReq = &proto.MembershipRequest{
			SelfInformation: selfMsg.Envelope,
			Known:           oldKnown,
		}
		// Update original message
		msg.Content = &proto.GossipMessage_MemReq{
			MemReq: memReq,
		}
		// Update the envelope of the outer message, no need to sign (point2point)
		msg, err = msg.NoopSign()
		if err != nil {
			return
		}
	}
	da.c.Send(msg, &comm.RemotePeer{PKIID: peer.PKIid, Endpoint: peer.PreferredEndpoint()})
}

func (da *discoveryAdapter) Ping(peer *discovery.NetworkMember) bool {
	err := da.c.Probe(&comm.RemotePeer{Endpoint: peer.PreferredEndpoint(), PKIID: peer.PKIid})
	return err == nil
}

func (da *discoveryAdapter) Accept() <-chan proto.ReceivedMessage {
	return da.incChan
}

func (da *discoveryAdapter) PresumedDead() <-chan common.PKIidType {
	return da.presumedDead
}

func (da *discoveryAdapter) CloseConn(peer *discovery.NetworkMember) {
	da.c.CloseConn(&comm.RemotePeer{PKIID: peer.PKIid})
}

type discoverySecurityAdapter struct {
	identity              api.PeerIdentityType
	includeIdentityPeriod time.Time
	idMapper              identity.Mapper
	sa                    api.SecurityAdvisor
	mcs                   api.MessageCryptoService
	c                     comm.Comm
	logger                *logging.Logger
}

func (g *gossipServiceImpl) newDiscoverySecurityAdapter() *discoverySecurityAdapter {
	return &discoverySecurityAdapter{
		sa:                    g.secAdvisor,
		idMapper:              g.idMapper,
		mcs:                   g.mcs,
		c:                     g.comm,
		logger:                g.logger,
		includeIdentityPeriod: g.includeIdentityPeriod,
		identity:              g.selfIdentity,
	}
}

// validateAliveMsg validates that an Alive message is authentic
func (sa *discoverySecurityAdapter) ValidateAliveMsg(m *proto.SignedGossipMessage) bool {
	am := m.GetAliveMsg()
	if am == nil || am.Membership == nil || am.Membership.PkiId == nil || !m.IsSigned() {
		sa.logger.Warning("Invalid alive message:", m)
		return false
	}

	var identity api.PeerIdentityType

	// If identity is included inside AliveMessage
	if am.Identity != nil {
		identity = api.PeerIdentityType(am.Identity)
		claimedPKIID := am.Membership.PkiId
		err := sa.idMapper.Put(claimedPKIID, identity)
		if err != nil {
			sa.logger.Warningf("Failed validating identity of %v reason: %+v", am, errors.WithStack(err))
			return false
		}
	} else {
		identity, _ = sa.idMapper.Get(am.Membership.PkiId)
		if identity != nil {
			sa.logger.Debug("Fetched identity of", am.Membership.PkiId, "from identity store")
		}
	}

	if identity == nil {
		sa.logger.Debug("Don't have certificate for", am)
		return false
	}

	return sa.validateAliveMsgSignature(m, identity)
}

// SignMessage signs an AliveMessage and updates its signature field
func (sa *discoverySecurityAdapter) SignMessage(m *proto.GossipMessage, internalEndpoint string) *proto.Envelope {
	signer := func(msg []byte) ([]byte, error) {
		return sa.mcs.Sign(msg)
	}
	if m.IsAliveMsg() && time.Now().Before(sa.includeIdentityPeriod) {
		m.GetAliveMsg().Identity = sa.identity
	}
	sMsg := &proto.SignedGossipMessage{
		GossipMessage: m,
	}
	e, err := sMsg.Sign(signer)
	if err != nil {
		sa.logger.Warningf("Failed signing message: %+v", errors.WithStack(err))
		return nil
	}

	if internalEndpoint == "" {
		return e
	}
	e.SignSecret(signer, &proto.Secret{
		Content: &proto.Secret_InternalEndpoint{
			InternalEndpoint: internalEndpoint,
		},
	})
	return e
}

func (sa *discoverySecurityAdapter) validateAliveMsgSignature(m *proto.SignedGossipMessage, identity api.PeerIdentityType) bool {
	am := m.GetAliveMsg()
	// At this point we got the certificate of the peer, proceed to verifying the AliveMessage
	verifier := func(peerIdentity []byte, signature, message []byte) error {
		return sa.mcs.Verify(api.PeerIdentityType(peerIdentity), signature, message)
	}

	// We verify the signature on the message
	err := m.Verify(identity, verifier)
	if err != nil {
		sa.logger.Warningf("Failed verifying: %v: %+v", am, errors.WithStack(err))
		return false
	}

	return true
}

func (g *gossipServiceImpl) createCertStorePuller() pull.Mediator {
	conf := pull.Config{
		MsgType:           proto.PullMsgType_IDENTITY_MSG,
		Channel:           []byte(""),
		ID:                g.conf.InternalEndpoint,
		PeerCountToSelect: g.conf.PullPeerNum,
		PullInterval:      g.conf.PullInterval,
		Tag:               proto.GossipMessage_EMPTY,
	}
	pkiIDFromMsg := func(msg *proto.SignedGossipMessage) string {
		identityMsg := msg.GetPeerIdentity()
		if identityMsg == nil || identityMsg.PkiId == nil {
			return ""
		}
		return fmt.Sprintf("%s", string(identityMsg.PkiId))
	}
	certConsumer := func(msg *proto.SignedGossipMessage) {
		idMsg := msg.GetPeerIdentity()
		if idMsg == nil || idMsg.Cert == nil || idMsg.PkiId == nil {
			g.logger.Warning("Invalid PeerIdentity:", idMsg)
			return
		}
		err := g.idMapper.Put(common.PKIidType(idMsg.PkiId), api.PeerIdentityType(idMsg.Cert))
		if err != nil {
			g.logger.Warningf("Failed associating PKI-ID with certificate: %+v", errors.WithStack(err))
		}
		g.logger.Info("Learned of a new certificate:", idMsg.Cert)
	}
	adapter := &pull.PullAdapter{
		Sndr:            g.comm,
		MemSvc:          g.disc,
		IdExtractor:     pkiIDFromMsg,
		MsgCons:         certConsumer,
		EgressDigFilter: g.sameOrgOrOurOrgPullFilter,
	}
	return pull.NewPullMediator(conf, adapter)
}

func (g *gossipServiceImpl) sameOrgOrOurOrgPullFilter(msg proto.ReceivedMessage) func(string) bool {
	peersOrg := g.secAdvisor.OrgByPeerIdentity(msg.GetConnectionInfo().Identity)
	if len(peersOrg) == 0 {
		g.logger.Warning("Failed determining organization of", msg.GetConnectionInfo())
		return func(_ string) bool {
			return false
		}
	}

	// If the peer is from our org, gossip all identities
	if bytes.Equal(g.selfOrg, peersOrg) {
		return func(_ string) bool {
			return true
		}
	}
	// Else, the peer is from a different org
	return func(item string) bool {
		pkiID := common.PKIidType(item)
		msgsOrg := g.getOrgOfPeer(pkiID)
		if len(msgsOrg) == 0 {
			g.logger.Warning("Failed determining organization of", pkiID)
			return false
		}
		// Don't gossip identities of dead peers or of peers
		// without external endpoints, to peers of foreign organizations.
		if !g.hasExternalEndpoint(pkiID) {
			return false
		}
		// Peer from our org or identity from our org or identity from peer's org
		return bytes.Equal(msgsOrg, g.selfOrg) || bytes.Equal(msgsOrg, peersOrg)
	}
}

func (g *gossipServiceImpl) connect2BootstrapPeers() {
	for _, endpoint := range g.conf.BootstrapPeers {
		endpoint := endpoint
		identifier := func() (*discovery.PeerIdentification, error) {
			remotePeerIdentity, err := g.comm.Handshake(&comm.RemotePeer{Endpoint: endpoint})
			if err != nil {
				return nil, errors.WithStack(err)
			}
			sameOrg := bytes.Equal(g.selfOrg, g.secAdvisor.OrgByPeerIdentity(remotePeerIdentity))
			if !sameOrg {
				return nil, errors.Errorf("%s isn't in our organization, cannot be a bootstrap peer", endpoint)
			}
			pkiID := g.mcs.GetPKIidOfCert(remotePeerIdentity)
			if len(pkiID) == 0 {
				return nil, errors.Errorf("Wasn't able to extract PKI-ID of remote peer with identity of %v", remotePeerIdentity)
			}
			return &discovery.PeerIdentification{ID: pkiID, SelfOrg: sameOrg}, nil
		}
		g.disc.Connect(discovery.NetworkMember{
			InternalEndpoint: endpoint,
			Endpoint:         endpoint,
		}, identifier)
	}

}

func (g *gossipServiceImpl) createStateInfoMsg(metadata []byte, chainID common.ChainID, leftChannel bool) (*proto.SignedGossipMessage, error) {
	metaState, err := common.FromBytes(metadata)
	if err != nil {
		return nil, err
	}
	pkiID := g.comm.GetPKIid()
	stateInfMsg := &proto.StateInfo{
		Channel_MAC: channel.GenerateMAC(pkiID, chainID),
		Metadata:    metadata,
		PkiId:       g.comm.GetPKIid(),
		Timestamp: &proto.PeerTime{
			IncNum: uint64(g.incTime.UnixNano()),
			SeqNum: uint64(time.Now().UnixNano()),
		},
		Properties: &proto.Properties{
			LedgerHeight: metaState.LedgerHeight,
		},
	}
	if leftChannel {
		stateInfMsg.Properties.LeftChannel = true
	}
	m := &proto.GossipMessage{
		Nonce: 0,
		Tag:   proto.GossipMessage_CHAN_OR_ORG,
		Content: &proto.GossipMessage_StateInfo{
			StateInfo: stateInfMsg,
		},
	}
	sMsg := &proto.SignedGossipMessage{
		GossipMessage: m,
	}
	signer := func(msg []byte) ([]byte, error) {
		return g.mcs.Sign(msg)
	}
	_, err = sMsg.Sign(signer)
	return sMsg, errors.WithStack(err)
}

func (g *gossipServiceImpl) hasExternalEndpoint(PKIID common.PKIidType) bool {
	if nm := g.disc.Lookup(PKIID); nm != nil {
		return nm.Endpoint != ""
	}
	return false
}

func (g *gossipServiceImpl) isInMyorg(member discovery.NetworkMember) bool {
	if member.PKIid == nil {
		return false
	}
	if org := g.getOrgOfPeer(member.PKIid); org != nil {
		return bytes.Equal(g.selfOrg, org)
	}
	return false
}

func (g *gossipServiceImpl) getOrgOfPeer(PKIID common.PKIidType) api.OrgIdentityType {
	cert, err := g.idMapper.Get(PKIID)
	if err != nil {
		return nil
	}

	return g.secAdvisor.OrgByPeerIdentity(cert)
}

func (g *gossipServiceImpl) validateLeadershipMessage(msg *proto.SignedGossipMessage) error {
	pkiID := msg.GetLeadershipMsg().PkiId
	if len(pkiID) == 0 {
		return errors.New("Empty PKI-ID")
	}
	identity, err := g.idMapper.Get(pkiID)
	if err != nil {
		return errors.Wrap(err, "Unable to fetch PKI-ID from id-mapper")
	}
	return msg.Verify(identity, func(peerIdentity []byte, signature, message []byte) error {
		return g.mcs.Verify(identity, signature, message)
	})
}

func (g *gossipServiceImpl) validateStateInfoMsg(msg *proto.SignedGossipMessage) error {
	verifier := func(identity []byte, signature, message []byte) error {
		pkiID := g.idMapper.GetPKIidOfCert(api.PeerIdentityType(identity))
		if pkiID == nil {
			return errors.New("PKI-ID not found in identity mapper")
		}
		return g.idMapper.Verify(pkiID, signature, message)
	}
	identity, err := g.idMapper.Get(msg.GetStateInfo().PkiId)
	if err != nil {
		return errors.WithStack(err)
	}
	return msg.Verify(identity, verifier)
}

func (g *gossipServiceImpl) disclosurePolicy(remotePeer *discovery.NetworkMember) (discovery.Sieve, discovery.EnvelopeFilter) {
	remotePeerOrg := g.getOrgOfPeer(remotePeer.PKIid)

	if len(remotePeerOrg) == 0 {
		g.logger.Warning("Cannot determine organization of", remotePeer)
		return func(msg *proto.SignedGossipMessage) bool {
				return false
			}, func(msg *proto.SignedGossipMessage) *proto.Envelope {
				return msg.Envelope
			}
	}

	return func(msg *proto.SignedGossipMessage) bool {
			if !msg.IsAliveMsg() {
				g.logger.Panic("Programming error, this should be used only on alive messages")
			}
			org := g.getOrgOfPeer(msg.GetAliveMsg().Membership.PkiId)
			if len(org) == 0 {
				g.logger.Warning("Unable to determine org of message", msg.GossipMessage)
				// Don't disseminate messages who's origin org is unknown
				return false
			}

			// Target org and the message are from the same org
			fromSameForeignOrg := bytes.Equal(remotePeerOrg, org)
			// The message is from my org
			fromMyOrg := bytes.Equal(g.selfOrg, org)
			// Forward to target org only messages from our org, or from the target org itself.
			if !(fromSameForeignOrg || fromMyOrg) {
				return false
			}

			// Pass the alive message only if the alive message is in the same org as the remote peer
			// or the message has an external endpoint, and the remote peer also has one
			return bytes.Equal(org, remotePeerOrg) || msg.GetAliveMsg().Membership.Endpoint != "" && remotePeer.Endpoint != ""
		}, func(msg *proto.SignedGossipMessage) *proto.Envelope {
			if !bytes.Equal(g.selfOrg, remotePeerOrg) {
				msg.SecretEnvelope = nil
			}
			return msg.Envelope
		}
}

func (g *gossipServiceImpl) peersByOriginOrgPolicy(peer discovery.NetworkMember) filter.RoutingFilter {
	peersOrg := g.getOrgOfPeer(peer.PKIid)
	if len(peersOrg) == 0 {
		g.logger.Warning("Unable to determine organization of peer", peer)
		// Don't disseminate messages who's origin org is undetermined
		return filter.SelectNonePolicy
	}

	if bytes.Equal(g.selfOrg, peersOrg) {
		// Disseminate messages from our org to all known organizations.
		// IMPORTANT: Currently a peer cannot un-join a channel, so the only way
		// of making gossip stop talking to an organization is by having the MSP
		// refuse validating messages from it.
		return filter.SelectAllPolicy
	}

	// Else, select peers from the origin's organization,
	// and also peers from our own organization
	return func(member discovery.NetworkMember) bool {
		memberOrg := g.getOrgOfPeer(member.PKIid)
		if len(memberOrg) == 0 {
			return false
		}
		isFromMyOrg := bytes.Equal(g.selfOrg, memberOrg)
		return isFromMyOrg || bytes.Equal(memberOrg, peersOrg)
	}
}

// partitionMessages receives a predicate and a slice of gossip messages
// and returns a tuple of two slices: the messages that hold for the predicate
// and the rest
/*
   接收两个输入参数，第一个是上面所说的谓词，第二个是一个消息切片，该函数最后输出的是两个消息切片，一个是满足谓词的消息切片
   另一个是剩余的消息切片，因为上面说的5个谓词，有些是包含关系的，比如说isOrgRestricted包含了isLeadershipMsg，故消息类别的解析也是有顺序限制的
   blocks－＞　leadershipMsgs -> stateInfoMsgs-> orgMsgs -> others
 */
func partitionMessages(pred common.MessageAcceptor, a []*emittedGossipMessage) ([]*emittedGossipMessage, []*emittedGossipMessage) {
	s1 := []*emittedGossipMessage{}
	s2 := []*emittedGossipMessage{}
	for _, m := range a {
		if pred(m) {
			s1 = append(s1, m)
		} else {
			s2 = append(s2, m)
		}
	}
	return s1, s2
}

// extractChannels returns a slice with all channels
// of all given GossipMessages
func extractChannels(a []*emittedGossipMessage) []common.ChainID {
	channels := []common.ChainID{}
	for _, m := range a {
		if len(m.Channel) == 0 {
			continue
		}
		sameChan := func(a interface{}, b interface{}) bool {
			return bytes.Equal(a.(common.ChainID), b.(common.ChainID))
		}
		if util.IndexInSlice(channels, common.ChainID(m.Channel), sameChan) == -1 {
			channels = append(channels, common.ChainID(m.Channel))
		}
	}
	return channels
}
