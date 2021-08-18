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
	//startupGracePeriod整个判断过程N+15秒，或leader已经出现（即收到declaration消息），则停止判断，
	//这里需要深入理解，在一个新初始化的chaiin中，新的节点陆续加入到网络中，被gossip服务的discovery模块发现并记录，这个过程是很快的
	//可能几毫秒内就会发现若干个新节点，所系咋判断成员关系是否稳定时，若在等待1秒后新获取的成员数据量还是等于原来的成员数量，那么
	//election模块就认为在1s这么长的时间内都没有新节点加入，则自认为当前网络中所有活着的界定啊都已经发现了，即成员关系固定了。另外一个
	//这个等待进成员关系固定化的过程是一个辅助election模块进行选举的leader的，不能无限期等待下去而耽误了选举的正事儿，所以规定了15s
	//要是15s还没固定，那就不等了。自然的，要是这个期间已经有了leader，那也就没有选举的必要了，没选举的必要，更没有再等待固定化的必要，
	//所以这种情况下就会停止判断。
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
			//如果此时模块正在休眠且interruptChan是空，则向iinterruptChan唤醒模块
			//这里的interruptChan可以理解为模块的中断休眠频道，模块成员sleeping标志者模块当前是否在休眠，
			//而waitForINttrupt（time)函数通过select{}让模块进入休眠 ，等待time后，或者declaration消息一旦到来，在置sleeping为false，即唤醒模块。
			le.interruptChan <- struct{}{}
		}
		if bytes.Compare(msg.SenderID(), le.id) < 0 && le.IsLeader() {
			//若自己当前是leader而declaration中的新leader不是自己，则调用stopBeingLeader，停止当leader。
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
	/*
	   现在可以election任务了。1）如果现在还没有leader，则调用leaderElection（）进行选举，这里说
	   选举，不如说进行自荐，
	 */
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
	le.propose() //向其他节点传播自己的自荐信，即包含自己身份的proposal消息，其他节点收到后
	//会在他们的handleMessage（）中存储这消息，
	// Collect other proposals
	le.waitForInterrupt(getLeaderElectionDuration())//让模块进入休眠，time为5s
	//由peer.gossip.election.leaderElectionDuration决定，在这个休眠过程中，既接收其他节点发来的自荐信并存储在proposals中，
	//也可以等待可能出现的declaration消息，在模块被唤醒后，判断此时是否有leader存在，若存在（休眠期间接收到了declaration信息，
	//则自己放弃成为leader，若不存在，则拿自己的身份与proposals中已经收到的自荐信中其他节点身份进行一一对比，看看自己是否比它们更
	//有资格当leader。这里所说的身份是一个节点pki-ID，而判断谁更有资格当leader的标准是bytes.compare(peerID(id),le.id)，即
	//谁的身份的二进制值更小，谁更有资格。不过gossip中的leader工作量大，并且没有其他的特权和优待，若proposals中所有的身份都没有
	//自己的身份小，则自己当选leader，，通过调用le.beLeader(),真正成为一名leader，把“已经有leader"和"自己是leader的标识"，即成员
	//leaderExists和isLeader,如果自己是follower，则执行le.follower(),都是自己成为的角色应该做的事情。leader的任务：循环地
	//每休眠5秒，因为自身是leader，所有不可能再从其他节点中接收到declaration消息而使休眠中中断，
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


	le.proposals.Clear() //清空“信箱” proposals以备下一轮选举，置leaderExists为0（用以接待收到leader发来的declaration消息后再置为1），
	//如果一个peer发现，当前已经存在leader，且自己不是leader，则首先将leader是否存在的标志位设为leaderExists设为0
	atomic.StoreInt32(&le.leaderExists, int32(0))
	select {
	//如果没有收到leadership declaration 5表内，设置值leader known false
	case <-time.After(getLeaderAliveThreshold())://然后等待10秒，由peer.gossip.election.leaderAliveThreshold配置决定），在这期间，如果收到了其他
	//leader发来的leadership declaration消息，则将leaderExists设为1，这样做的目的在于，探测网络分区或者leader节点是否失效

	//10s结束后还没有收到leader发来的declaration消息，即leaderExists还为0，则节点将再发起新一轮的选举。这里仍然是，follower认为10秒这么长的时间，
	//足够leader发送一条declaration消息给自己，若10s还没有收到，则follower认为这个leader已死，则自己发起新一轮的选举
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
	/*
	此函数等待成员关系固定化，这里的成员关系指的是网络中存在的所有或者的节点数量，固化指成员关系稳定的，
	这个等待过程是：确定当前时间为N后开始判断，使用适配器获取当前成员变量viewSize（追溯适配器可知，获取成员数量
	使用的还是discovery模块）
	然后进入for循环，不断地判断成员关系是否稳定，判断的标准是，每隔一秒重弄获取最新的成员数量newSize
	若newSize == viewSize,则称当前成员关系稳定，否则更新viewSize最新发现的成员 数量，即viewSize = newSize，进行下一轮判断

	 */
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
/*
  当一个节点调用le.beLeader(）成为一名leader时，调用le.callback(true)
  当一个节点调用le.stopBeingLeader()停止leader时，调用了le.callback(false),
  两者只是参数不同，“倒钩函数”callback有election模块初始化的时候传入，具体的赋值
  service/gossip_service.go中onStatusChangeFactory(),返回的函数func（isLeader bool）
  即根据是否是leader而分别执行g.deliveryService.StartDeliverForChannel,和g.deliveryService.StopDeliverForChannel
  从这点就可以看出，leader多干扰了DeliverForChanel向其它节点分发数据相关的一些事情，这与ordering服务的数据分啊客户端有关
 */
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
