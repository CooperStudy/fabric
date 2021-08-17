/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package election

import (
	"bytes"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hyperledger/fabric/gossip/util"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
)

// Gossip leader election module
// Algorithm properties:
// - Peers break symmetry by comparing IDs
// - Each peer is either a leader or a follower,
//   and the aim is to have exactly 1 leader if the membership view
//   is the same for all peers
// - If the network is partitioned into 2 or more sets, the number of leaders
//   is the number of network partitions, but when the partition heals,
//   only 1 leader should be left eventually
// - Peers communicate by gossiping leadership proposal or declaration messages

// The Algorithm, in pseudo code:
//
//
// variables:
// 	leaderKnown = false
//
// Invariant:
//	Peer listens for messages from remote peers
//	and whenever it receives a leadership declaration,
//	leaderKnown is set to true
//
// Startup():
// 	wait for membership view to stabilize, or for a leadership declaration is received
//      or the startup timeout expires.
//	goto SteadyState()
//
// SteadyState():
// 	while true:
//		If leaderKnown is false:
// 			LeaderElection()
//		If you are the leader:
//			Broadcast leadership declaration
//			If a leadership declaration was received from
// 			a peer with a lower ID,
//			become a follower
//		Else, you're a follower:
//			If haven't received a leadership declaration within
// 			a time threshold:
//				set leaderKnown to false
//
// LeaderElection():
// 	Gossip leadership proposal message
//	Collect messages from other peers sent within a time period
//	If received a leadership declaration:
//		return
//	Iterate over all proposal messages collected.
// 	If a proposal message from a peer with an ID lower
// 	than yourself was received, return.
//	Else, declare yourself a leader

// LeaderElectionAdapter is used by the leader election module
// to send and receive messages and to get membership information
type LeaderElectionAdapter interface {
	// Gossip gossips a message to other peers
	Gossip(Msg)

	// Accept returns a channel that emits messages
	Accept() <-chan Msg

	// CreateProposalMessage
	CreateMessage(isDeclaration bool) Msg

	// Peers returns a list of peers considered alive
	Peers() []Peer
}

type leadershipCallback func(isLeader bool)

// LeaderElectionService is the object that runs the leader election algorithm
/*
    由type  leaderElectionSvcImpl struct,LeaderElection模块的初始化和上一部分中GossipSateProvider的初始化一样，都是在InitializeChannel方法中进行的
 */
type LeaderElectionService interface {
	// IsLeader returns whether this peer is a leader or not
	IsLeader() bool

	// Stop stops the LeaderElectionService
	Stop()

	// Yield relinquishes the leadership until a new leader is elected,
	// or a timeout expires
	Yield()
}

type peerID []byte

// Peer describes a remote peer
type Peer interface {
	// ID returns the ID of the peer
	ID() peerID
}

// Msg describes a message sent from a remote peer
type Msg interface {
	// SenderID returns the ID of the peer sent the message
	SenderID() peerID
	// IsProposal returns whether this message is a leadership proposal
	IsProposal() bool
	// IsDeclaration returns whether this message is a leadership declaration
	IsDeclaration() bool
}

func noopCallback(_ bool) {
}

// NewLeaderElectionService returns a new LeaderElectionService
/*
   根据传入参数初始化一个leader选举服务的实例
   leaderElection组件用于领导节点的选举，将收到的信息在节点间扩散。
   leader选举的算法性质：1） peer之间靠pkiID分辨各自的身份， 2）每个peer不是leader就是follow，算法的目的在于对于所有具有相同成员视图的peers，只选图一个leader
   3）如果网络分区成2个或多个子集，则leader的数目等于分区的数目，但是当网络又归为一个时，有一个leader可以留下来
 */
func NewLeaderElectionService(adapter LeaderElectionAdapter, id string, callback leadershipCallback) LeaderElectionService {
	if len(id) == 0 {
		panic("Empty id")
	}
	/*
	  初始化选服务的实例
	 */
	le := &leaderElectionSvcImpl{
		id:            peerID(id),
		proposals:     util.NewSet(),
		adapter:       adapter,
		stopChan:      make(chan struct{}, 1),
		interruptChan: make(chan struct{}, 1),
		logger:        util.GetLogger(util.LoggingElectionModule, ""),
		callback:      noopCallback,
	}

	if callback != nil {
		le.callback = callback
	}

	//开启一个go程
	go le.start()
	return le
}

// leaderElectionSvcImpl is an implementation of a LeaderElectionService
type leaderElectionSvcImpl struct {
	id        peerID
	proposals *util.Set
	sync.Mutex
	stopChan      chan struct{}
	interruptChan chan struct{}
	stopWG        sync.WaitGroup
	isLeader      int32
	toDie         int32
	leaderExists  int32
	yield         int32
	sleeping      bool
	adapter       LeaderElectionAdapter
	logger        *logging.Logger
	callback      leadershipCallback
	yieldTimer    *time.Timer
}

func (le *leaderElectionSvcImpl) start() {

	le.stopWG.Add(2)
	go le.handleMessages()//监听leader选举相关消息
	le.waitForMembershipStabilization(getStartupGracePeriod())//等待成员视图稳定，时间长度15秒，core.yaml peer.gossip.election.membershipSampleInterval配置项决定
	go le.run()//执行leader选举并广播相关的Proposal或declaration消息
}
/*
 主要开启处理消息服务，首先
 */
func (le *leaderElectionSvcImpl) handleMessages() {
	le.logger.Debug(le.id, ": Entering")
	defer le.logger.Debug(le.id, ": Exiting")
	defer le.stopWG.Done()
	//首先用适配器adapter获取接收消息的频道
	msgChan := le.adapter.Accept()//msgChan是专门处理election模块中特定消息类型msgImpl的
	for {
		//在for循环中持续接收msgChan中到来的一条条消息，交由handleMessage函数处理，
		select {
		case <-le.stopChan:
			le.stopChan <- struct{}{}
			return
		case msg := <-msgChan:

			if !le.isAlive(msg.SenderID()) {
				le.logger.Debug(le.id, ": Got message from", msg.SenderID(), "but it is not in the view")
				break
			}
			le.handleMessage(msg)
		}
	}
}

func (le *leaderElectionSvcImpl) handleMessage(msg Msg) {
	msgType := "proposal"
	if msg.IsDeclaration() {
		msgType = "declaration"
	}
	le.logger.Debug(le.id, ":", msg.SenderID(), "sent us", msgType)
	le.Lock()
	defer le.Unlock()
	//如果消息是proposal，则记录到election模块成员proposals中，即把所有其他节点发来的想要成为leader的自荐信先放起来
	if msg.IsProposal() {
		le.proposals.Add(string(msg.SenderID()))
	} else if msg.IsDeclaration() {
		//如果消息是declaration,则把已经存在leader的标志leaderExists置为1
		atomic.StoreInt32(&le.leaderExists, int32(1))
		if le.sleeping && len(le.interruptChan) == 0 {
			//如果
			le.interruptChan <- struct{}{}
		}
		if bytes.Compare(msg.SenderID(), le.id) < 0 && le.IsLeader() {
			le.stopBeingLeader()
		}
	} else {
		// We shouldn't get here
		le.logger.Error("Got a message that's not a proposal and not a declaration")
	}
}

// waitForInterrupt sleeps until the interrupt channel is triggered
// or given timeout expires
func (le *leaderElectionSvcImpl) waitForInterrupt(timeout time.Duration) {
	le.logger.Debug(le.id, ": Entering")
	defer le.logger.Debug(le.id, ": Exiting")
	le.Lock()
	le.sleeping = true
	le.Unlock()

	select {
	case <-le.interruptChan:
	case <-le.stopChan:
		le.stopChan <- struct{}{}
	case <-time.After(timeout):
	}

	le.Lock()
	le.sleeping = false
	// We drain the interrupt channel
	// because we might get 2 leadership declarations messages
	// while sleeping, but we would only read 1 of them in the select block above
	le.drainInterruptChannel()
	le.Unlock()
}

func (le *leaderElectionSvcImpl) run() {
	defer le.stopWG.Done()
	for !le.shouldStop() {
		if !le.isLeaderExists() {
			//leader不存在，就进入选举
			le.leaderElection()
		}
		// If we are yielding and some leader has been elected,
		// stop yielding
		if le.isLeaderExists() && le.isYielding() {
			le.stopYielding()
		}
		if le.shouldStop() {
			return
		}
		if le.IsLeader() {
			//如果是leader就发送 leadership declaration 消息
			le.leader()
		} else {
			le.follower()
		}
	}
}

func (le *leaderElectionSvcImpl) leaderElection() {
	le.logger.Debug(le.id, ": Entering")
	defer le.logger.Debug(le.id, ": Exiting")
	// If we're yielding to other peers, do not participate
	// in leader election
	if le.isYielding() {
		return
	}
	// Propose ourselves as a leader
	le.propose()
	// Collect other proposals
	le.waitForInterrupt(getLeaderElectionDuration())
	// If someone declared itself as a leader, give up
	// on trying to become a leader too
	if le.isLeaderExists() {
		le.logger.Info(le.id, ": Some peer is already a leader")
		return
	}

	if le.isYielding() {
		le.logger.Debug(le.id, ": Aborting leader election because yielding")
		return
	}
	// Leader doesn't exist, let's see if there is a better candidate than us
	// for being a leader
	for _, o := range le.proposals.ToArray() {
		id := o.(string)
		if bytes.Compare(peerID(id), le.id) < 0 {
			return
		}
	}
	//if a proposal mesage form peer with an ID lower than yourself was received,return

	// If we got here, there is no one that proposed being a leader
	// that's a better candidate than us.
	le.beLeader()//声明自己是领导
	atomic.StoreInt32(&le.leaderExists, int32(1))
}

// propose sends a leadership proposal message to remote peers
func (le *leaderElectionSvcImpl) propose() {
	le.logger.Debug(le.id, ": Entering")
	le.logger.Debug(le.id, ": Exiting")
	leadershipProposal := le.adapter.CreateMessage(false)
	le.adapter.Gossip(leadershipProposal)
}

func (le *leaderElectionSvcImpl) follower() {
	le.logger.Debug(le.id, ": Entering")
	defer le.logger.Debug(le.id, ": Exiting")


	le.proposals.Clear()
	//如果一个peer发现，当前已经存在leader，且自己不是leader，则首先将leader是否存在的标志位设为leaderExists设为0
	atomic.StoreInt32(&le.leaderExists, int32(0))
	select {
	//如果没有收到leadership declaration 5表内，设置值leader known false
	case <-time.After(getLeaderAliveThreshold())://然后等待10秒，由peer.gossip.election.leaderAliveThreshold配置决定），在这期间，如果收到了其他
	//leader发来的leadership declaration消息，则将leaderExists设为1，这样做的目的在于，探测网络分区或者leader节点是否失效
	case <-le.stopChan:
		le.stopChan <- struct{}{}
	}
}

func (le *leaderElectionSvcImpl) leader() {

	leaderDeclaration := le.adapter.CreateMessage(true)
	le.adapter.Gossip(leaderDeclaration) //如果一个peer被选为leader，则它每5s向外发送一此leadership declaration
	//peer.gossip.election.leaderAliveThreshold的时间（10s）/2 = 5s
	le.waitForInterrupt(getLeadershipDeclarationInterval())
}

// waitForMembershipStabilization waits for membership view to stabilize
// or until a time limit expires, or until a peer declares itself as a leader
/*
   1.首先设置一个定时器，事件妆度15秒。在定时器没到期之前，每隔1s,peer.gossip.election.membersipSampleInterval配置项决定
    做如下检查，1)成员视图是否稳定，即所掌握的remote peers的数目和航一次检查时的数目是否一致
     2）计时器是否到期，
　　　３）是否已经存在leader，若有一个满足条件，则结束对成员视图的等待
 */
func (le *leaderElectionSvcImpl) waitForMembershipStabilization(timeLimit time.Duration) {


	le.logger.Debug(le.id, ": Entering")
	defer le.logger.Debug(le.id, ": Exiting, peers found", len(le.adapter.Peers()))
	endTime := time.Now().Add(timeLimit)
	viewSize := len(le.adapter.Peers())
	for !le.shouldStop() {
		time.Sleep(getMembershipSampleInterval())//peer.gossip.election.membershipSampleInterval 1秒
		newSize := len(le.adapter.Peers())
		if newSize == viewSize || time.Now().After(endTime) || le.isLeaderExists() {
			//1)所掌握的remote peers的数目和航一次检查时的数目是否一致
			//2)计时器是否到期，
			//3)是否已经存在leader，若有一个满足条件，则结束对成员视图的等待
			return
		}
		viewSize = newSize
	}
}

// drainInterruptChannel clears the interruptChannel
// if needed
func (le *leaderElectionSvcImpl) drainInterruptChannel() {
	if len(le.interruptChan) == 1 {
		<-le.interruptChan
	}
}

// isAlive returns whether peer of given id is considered alive
func (le *leaderElectionSvcImpl) isAlive(id peerID) bool {
	for _, p := range le.adapter.Peers() {
		if bytes.Equal(p.ID(), id) {
			return true
		}
	}
	return false
}

func (le *leaderElectionSvcImpl) isLeaderExists() bool {
	return atomic.LoadInt32(&le.leaderExists) == int32(1)
}

// IsLeader returns whether this peer is a leader
func (le *leaderElectionSvcImpl) IsLeader() bool {
	isLeader := atomic.LoadInt32(&le.isLeader) == int32(1)
	le.logger.Debug(le.id, ": Returning", isLeader)
	return isLeader
}

func (le *leaderElectionSvcImpl) beLeader() {
	le.logger.Info(le.id, ": Becoming a leader")
	atomic.StoreInt32(&le.isLeader, int32(1))
	le.callback(true)
}

func (le *leaderElectionSvcImpl) stopBeingLeader() {
	le.logger.Info(le.id, "Stopped being a leader")
	atomic.StoreInt32(&le.isLeader, int32(0))
	le.callback(false)
}

func (le *leaderElectionSvcImpl) shouldStop() bool {
	return atomic.LoadInt32(&le.toDie) == int32(1)
}

func (le *leaderElectionSvcImpl) isYielding() bool {
	return atomic.LoadInt32(&le.yield) == int32(1)
}

func (le *leaderElectionSvcImpl) stopYielding() {
	le.logger.Debug("Stopped yielding")
	le.Lock()
	defer le.Unlock()
	atomic.StoreInt32(&le.yield, int32(0))
	le.yieldTimer.Stop()
}

// Yield relinquishes the leadership until a new leader is elected,
// or a timeout expires
func (le *leaderElectionSvcImpl) Yield() {
	le.Lock()
	defer le.Unlock()
	if !le.IsLeader() || le.isYielding() {
		return
	}
	// Turn on the yield flag
	atomic.StoreInt32(&le.yield, int32(1))
	// Stop being a leader
	le.stopBeingLeader()
	// Clear the leader exists flag since it could be that we are the leader
	atomic.StoreInt32(&le.leaderExists, int32(0))
	// Clear the yield flag in any case afterwards
	le.yieldTimer = time.AfterFunc(getLeaderAliveThreshold()*6, func() {
		atomic.StoreInt32(&le.yield, int32(0))
	})
}

// Stop stops the LeaderElectionService
func (le *leaderElectionSvcImpl) Stop() {
	le.logger.Debug(le.id, ": Entering")
	defer le.logger.Debug(le.id, ": Exiting")
	atomic.StoreInt32(&le.toDie, int32(1))
	le.stopChan <- struct{}{}
	le.stopWG.Wait()
}

// SetStartupGracePeriod configures startup grace period interval,
// the period of time to wait until election algorithm will start
func SetStartupGracePeriod(t time.Duration) {
	viper.Set("peer.gossip.election.startupGracePeriod", t)
}

// SetMembershipSampleInterval setups/initializes the frequency the
// membership view should be checked
func SetMembershipSampleInterval(t time.Duration) {
	viper.Set("peer.gossip.election.membershipSampleInterval", t)
}

// SetLeaderAliveThreshold configures leader election alive threshold
func SetLeaderAliveThreshold(t time.Duration) {
	viper.Set("peer.gossip.election.leaderAliveThreshold", t)
}

// SetLeaderElectionDuration configures expected leadership election duration,
// interval to wait until leader election will be completed
func SetLeaderElectionDuration(t time.Duration) {
	viper.Set("peer.gossip.election.leaderElectionDuration", t)
}

func getStartupGracePeriod() time.Duration {
	return util.GetDurationOrDefault("peer.gossip.election.startupGracePeriod", time.Second*15)
}

func getMembershipSampleInterval() time.Duration {
	return util.GetDurationOrDefault("peer.gossip.election.membershipSampleInterval", time.Second)
}

func getLeaderAliveThreshold() time.Duration {
	return util.GetDurationOrDefault("peer.gossip.election.leaderAliveThreshold", time.Second*10)
}

func getLeadershipDeclarationInterval() time.Duration {
	return time.Duration(getLeaderAliveThreshold() / 2)
}

func getLeaderElectionDuration() time.Duration {
	return util.GetDurationOrDefault("peer.gossip.election.leaderElectionDuration", time.Second*5)
}

// GetMsgExpirationTimeout return leadership message expiration timeout
func GetMsgExpirationTimeout() time.Duration {
	return getLeaderAliveThreshold() * 10
}
