/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package gossip

import (
	"bytes"
	"sync"
	"sync/atomic"

	"github.com/hyperledger/fabric/gossip/api"
	"github.com/hyperledger/fabric/gossip/comm"
	"github.com/hyperledger/fabric/gossip/common"
	"github.com/hyperledger/fabric/gossip/discovery"
	"github.com/hyperledger/fabric/gossip/gossip/channel"
	proto "github.com/hyperledger/fabric/protos/gossip"
)

type channelState struct {
	stopping int32
	sync.RWMutex
	channels map[string]channel.GossipChannel
	g        *gossipServiceImpl
}

func (cs *channelState) stop() {
	//logger.Info("====channelState===stop==")
	if cs.isStopping() {
		return
	}
	atomic.StoreInt32(&cs.stopping, int32(1))
	cs.Lock()
	defer cs.Unlock()
	for _, gc := range cs.channels {
		gc.Stop()
	}
}

func (cs *channelState) isStopping() bool {
	//logger.Info("====channelState===isStopping==")
	return atomic.LoadInt32(&cs.stopping) == int32(1)
}

func (cs *channelState) lookupChannelForMsg(msg proto.ReceivedMessage) channel.GossipChannel {
	//logger.Info("====channelState===lookupChannelForMsg==")
	if msg.GetGossipMessage().IsStateInfoPullRequestMsg() {
		sipr := msg.GetGossipMessage().GetStateInfoPullReq()
		mac := sipr.Channel_MAC
		pkiID := msg.GetConnectionInfo().ID
		return cs.getGossipChannelByMAC(mac, pkiID)
	}
	return cs.lookupChannelForGossipMsg(msg.GetGossipMessage().GossipMessage)
}

func (cs *channelState) lookupChannelForGossipMsg(msg *proto.GossipMessage) channel.GossipChannel {
	//logger.Info("====channelState===lookupChannelForGossipMsg==")
	if !msg.IsStateInfoMsg() {
		// If we reached here then the message isn't:
		// 1) StateInfoPullRequest
		// 2) StateInfo
		// Hence, it was already sent to a peer (us) that has proved it knows the channel name, by
		// sending StateInfo messages in the past.
		// Therefore- we use the channel name from the message itself.
		return cs.getGossipChannelByChainID(msg.Channel)
	}

	// Else, it's a StateInfo message.
	stateInfMsg := msg.GetStateInfo()
	return cs.getGossipChannelByMAC(stateInfMsg.Channel_MAC, stateInfMsg.PkiId)
}

func (cs *channelState) getGossipChannelByMAC(receivedMAC []byte, pkiID common.PKIidType) channel.GossipChannel {
	//logger.Info("====channelState===getGossipChannelByMAC==")
	// Iterate over the channels, and try to find a channel that the computation
	// of the MAC is equal to the MAC on the message.
	// If it is, then the peer that signed the message knows the name of the channel
	// because its PKI-ID was checked when the message was verified.
	cs.RLock()
	defer cs.RUnlock()
	for chanName, gc := range cs.channels {
		//logger.Info("======chanName=====",chanName)//mychannel
		//logger.Info("======gc=====",gc)
		/*
		&{0xc002b5efa0 {{0 0} 0 0 0 0} 0 0xc000399e90 [111 36 162 133 16 26 222 168 198 53 13 236 188 162 199 49 240 121 120 163 42 92 168 118 195 242 204 167 181 179 224 144] [79 114 103 49 77 83 80] 0xc002b50540 0xc002789940 [[79 114 103 49 77 83 80]] 0xc00259ef10 0xc002b54500 0xc002b63a10 0xc002b54680 [109 121 99 104 97 110 110 101 108] 0xc000228a20 0xc000304c40 0xc002b5d0e0 0xc002b5d180 0xc002b5f060 1 1637216990210960351 0 0xc002b5f8e0}
		*/
		mac := channel.GenerateMAC(pkiID, common.ChainID(chanName))
		if bytes.Equal(mac, receivedMAC) {
			return gc
		}
	}
	return nil
}

func (cs *channelState) getGossipChannelByChainID(chainID common.ChainID) channel.GossipChannel {
	//logger.Info("====channelState===getGossipChannelByChainID==")
	if cs.isStopping() {
		return nil
	}
	cs.RLock()
	defer cs.RUnlock()
	a := string(chainID)
	//logger.Info("==========string(chainID)=====",string(chainID))//mychannel
	b := cs.channels[a]
	//logger.Info("=======b",b)
	/*
	&{0xc002446320 {{0 0} 0 0 0 0} 0 0xc0002869c0 [111 36 162 133 16 26 222 168 198 53 13 236 188 162 199 49 240 121 120 163 42 92 168 118 195 242 204 167 181 179 224 144] [79 114 103 49 77 83 80] 0xc00262e600 0xc00024b520 [[79 114 103 49 77 83 80]] 0xc0005a0d00 0xc002480400 0xc0021f8ff0 0xc002480580 [109 121 99 104 97 110 110 101 108] 0xc0002b0bd0 0xc002260860 0xc00246ee60 0xc00246ef00 0xc0024463e0 1 1637298370970474877 0 0xc00032f5e0}
	*/
	return b
}

func (cs *channelState) joinChannel(joinMsg api.JoinChannelMessage, chainID common.ChainID) {
	//logger.Info("====channelState===joinChannel==")
	if cs.isStopping() {
		return
	}
	cs.Lock()
	defer cs.Unlock()
	 gc, exists := cs.channels[string(chainID)]
	//logger.Info("========string(chainID)===============",string(chainID))
	//logger.Info("=======exists========",exists)
	if !exists {
		//logger.Info("=========gc==========",gc)
		//logger.Info("=========gc==========",gc)
		pkiID := cs.g.comm.GetPKIid()
		//logger.Info("========pkiID======",pkiID)
		ga := &gossipAdapterImpl{gossipServiceImpl: cs.g, Discovery: cs.g.disc}
		gc := channel.NewGossipChannel(pkiID, cs.g.selfOrg, cs.g.mcs, chainID, ga, joinMsg)
		cs.channels[string(chainID)] = gc
	} else {
		gc.ConfigureChannel(joinMsg)
	}
}

type gossipAdapterImpl struct {
	*gossipServiceImpl
	discovery.Discovery
}

func (ga *gossipAdapterImpl) GetConf() channel.Config {
	//logger.Info("====gossipAdapterImpl===GetConf==")
	a  := channel.Config{
		ID:                          ga.conf.ID, //peer0.org1.example.com:7051
		MaxBlockCountToStore:        ga.conf.MaxBlockCountToStore,// 4s
		PublishStateInfoInterval:    ga.conf.PublishStateInfoInterval,//100
		PullInterval:                ga.conf.PullInterval,//3
		PullPeerNum:                 ga.conf.PullPeerNum,//4s
		RequestStateInfoInterval:    ga.conf.RequestStateInfoInterval,//4s
		BlockExpirationInterval:     ga.conf.PullInterval * 100,
		StateInfoCacheSweepInterval: ga.conf.PullInterval * 5,
		TimeForMembershipTracker:    ga.conf.TimeForMembershipTracker,
	}
	//logger.Info("============channel.Config=====================",a)
	/*
	{peer0.org1.example.com:7051 4s 100 3 4s 4s 6m40s 20s 5s}
	 */
	return a
}

func (ga *gossipAdapterImpl) Sign(msg *proto.GossipMessage) (*proto.SignedGossipMessage, error) {
	//logger.Info("====gossipAdapterImpl===Sign==")
	signer := func(msg []byte) ([]byte, error) {
		return ga.mcs.Sign(msg)
	}
	sMsg := &proto.SignedGossipMessage{
		GossipMessage: msg,
	}
	e, err := sMsg.Sign(signer)
	if err != nil {
		return nil, err
	}
	return &proto.SignedGossipMessage{
		Envelope:      e,
		GossipMessage: msg,
	}, nil
}

// Gossip gossips a message
func (ga *gossipAdapterImpl) Gossip(msg *proto.SignedGossipMessage) {
	//logger.Info("====gossipAdapterImpl===Gossip==")
	ga.gossipServiceImpl.emitter.Add(&emittedGossipMessage{
		SignedGossipMessage: msg,
		filter: func(_ common.PKIidType) bool {
			return true
		},
	})
}

// Forward sends message to the next hops
func (ga *gossipAdapterImpl) Forward(msg proto.ReceivedMessage) {
	//logger.Info("====gossipAdapterImpl===Forward==")
	ga.gossipServiceImpl.emitter.Add(&emittedGossipMessage{
		SignedGossipMessage: msg.GetGossipMessage(),
		filter:              msg.GetConnectionInfo().ID.IsNotSameFilter,
	})
}

func (ga *gossipAdapterImpl) Send(msg *proto.SignedGossipMessage, peers ...*comm.RemotePeer) {
	//logger.Info("====gossipAdapterImpl===Send==")
	ga.gossipServiceImpl.comm.Send(msg, peers...)
}

// ValidateStateInfoMessage returns error if a message isn't valid
// nil otherwise
func (ga *gossipAdapterImpl) ValidateStateInfoMessage(msg *proto.SignedGossipMessage) error {
	//logger.Info("====gossipAdapterImpl===ValidateStateInfoMessage==")
	return ga.gossipServiceImpl.validateStateInfoMsg(msg)
}

// GetOrgOfPeer returns the organization identifier of a certain peer
func (ga *gossipAdapterImpl) GetOrgOfPeer(PKIID common.PKIidType) api.OrgIdentityType {
	//logger.Info("====gossipAdapterImpl===GetOrgOfPeer==")
	return ga.gossipServiceImpl.getOrgOfPeer(PKIID)
}

// GetIdentityByPKIID returns an identity of a peer with a certain
// pkiID, or nil if not found
func (ga *gossipAdapterImpl) GetIdentityByPKIID(pkiID common.PKIidType) api.PeerIdentityType {
	//logger.Info("====gossipAdapterImpl===GetIdentityByPKIID==")
	identity, err := ga.idMapper.Get(pkiID)
	if err != nil {
		return nil
	}
	return identity
}
