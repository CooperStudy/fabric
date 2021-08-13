package gossip

import (
	"fmt"
	"time"

	"github.com/hyperledger/fabric/gossip/api"
	"github.com/hyperledger/fabric/gossip/comm"
	"github.com/hyperledger/fabric/gossip/common"
	"github.com/hyperledger/fabric/gossip/discovery"
	"github.com/hyperledger/fabric/gossip/filter"
	proto "github.com/hyperledger/fabric/protos/gossip"
)

// Gossip is the interface of the gossip component
// gossipSvc是gossip_service.go文件下定义的变量类型，具体是type Gossip interface
type Gossip interface {

	// Send sends a message to remote peers
	//将某一个message 发送到remote peers
	Send(msg *proto.GossipMessage, peers ...*comm.RemotePeer)

	// SendByCriteria sends a given message to all peers that match the given SendCriteria
	// 将某一个message发送到所有满足SendCriteria的peers
	SendByCriteria(*proto.SignedGossipMessage, SendCriteria) error

	// GetPeers returns the NetworkMembers considered alive
	//返回被认为还alive netWorkMembers
	Peers() []discovery.NetworkMember

	// PeersOfChannel returns the NetworkMembers considered alive
	// and also subscribed to the channel given
	//返回被认为还alive并且订阅了给定channel的NetworkMembers
	PeersOfChannel(common.ChainID) []discovery.NetworkMember

	// UpdateMetadata updates the self metadata of the discovery layer
	// the peer publishes to other peers
	// 更新peer发布给其他peers的discovery layer的self metadata
	UpdateMetadata(metadata []byte)

	// UpdateChannelMetadata updates the self metadata the peer
	// publishes to other peers about its channel-related state
	//更新peer发布给其他peers的与其channel-related state的self-metadata
	UpdateChannelMetadata(metadata []byte, chainID common.ChainID)

	// Gossip sends a message to other peers to the network
	//发送一个message到网络中的其他peers
	Gossip(msg *proto.GossipMessage)

	// PeerFilter receives a SubChannelSelectionCriteria and returns a RoutingFilter that selects
	// only peer identities that match the given criteria, and that they published their channel participation
	/*
	   接收一个SubChannelSelectionCriteria并且返回一个RoutingFilter
	   RoutingFilter选择那些满足给定的criteria（准则）,并且发布了他们的channel participation的peer Identity
	 */
	PeerFilter(channel common.ChainID, messagePredicate api.SubChannelSelectionCriteria) (filter.RoutingFilter, error)

	// Accept returns a dedicated read-only channel for messages sent by other nodes that match a certain predicate.
	// If passThrough is false, the messages are processed by the gossip layer beforehand.
	// If passThrough is true, the gossip layer doesn't intervene and the messages
	// can be used to send a reply back to the sender
	/*
	  Accept返回一个明确只读的通道，其中存储的是其他节点发送过来的符合某个谓词的messaged，
	  如果passThrough是false，这些message先有gossip layer处理
	  如果passThrouth是true，gossip layer不会介入，并且这些message可以被发送回对应的sender
	 */
	Accept(acceptor common.MessageAcceptor, passThrough bool) (<-chan *proto.GossipMessage, <-chan proto.ReceivedMessage)

	// JoinChan makes the Gossip instance join a channel
	//使得一个gossip实例加入channel
	JoinChan(joinMsg api.JoinChannelMessage, chainID common.ChainID)

	// LeaveChan makes the Gossip instance leave a channel.
	// It still disseminates(传播) stateInfo message, but doesn't participate
	// in block pulling anymore, and can't return anymore a list of peers
	// in the channel.
	//LeaveChan使得一个Gossip实例离开一个channel
	//该Gossip实例仍旧可以传播stateInfo message
	//但是不能再参与进block的拉去，并且不能再返回一个channel中所含peers的列表
	LeaveChan(chainID common.ChainID)

	// SuspectPeers makes the gossip instance validate identities of suspected peers, and close
	// any connections to peers with identities that are found invalid
	/*
	   SuspectPeers使得一个Gossip实例验证被怀疑的peers的identities，并且关掉那些被发现的identities不合法的peers之间
	   的connections
	 */
	SuspectPeers(s api.PeerSuspector)

	// Stop stops the gossip component
	// 停止该goss component
	Stop()
}

// emittedGossipMessage encapsulates signed gossip message to compose
// with routing filter to be used while message is forwarded
type emittedGossipMessage struct {
	*proto.SignedGossipMessage
	filter func(id common.PKIidType) bool
}

// SendCriteria defines how to send a specific message
type SendCriteria struct {
	Timeout    time.Duration        // Timeout defines the time to wait for acknowledgements
	MinAck     int                  // MinAck defines the amount of peers to collect acknowledgements from
	MaxPeers   int                  // MaxPeers defines the maximum number of peers to send the message to
	IsEligible filter.RoutingFilter // IsEligible defines whether a specific peer is eligible of receiving the message
	Channel    common.ChainID       // Channel specifies a channel to send this message on. \
	// Only peers that joined the channel would receive this message
}

// String returns a string representation of this SendCriteria
func (sc SendCriteria) String() string {
	return fmt.Sprintf("channel: %s, tout: %v, minAck: %d, maxPeers: %d", sc.Channel, sc.Timeout, sc.MinAck, sc.MaxPeers)
}

// Config is the configuration of the gossip component
type Config struct {
	BindPort            int      // Port we bind to, used only for tests
	ID                  string   // ID of this instance
	BootstrapPeers      []string // Peers we connect to at startup
	PropagateIterations int      // Number of times a message is pushed to remote peers
	PropagatePeerNum    int      // Number of peers selected to push messages to

	MaxBlockCountToStore int // Maximum count of blocks we store in memory

	MaxPropagationBurstSize    int           // Max number of messages stored until it triggers a push to remote peers
	MaxPropagationBurstLatency time.Duration // Max time between consecutive message pushes

	PullInterval time.Duration // Determines frequency of pull phases
	PullPeerNum  int           // Number of peers to pull from

	SkipBlockVerification bool // Should we skip verifying block messages or not

	PublishCertPeriod        time.Duration // Time from startup certificates are included in Alive messages
	PublishStateInfoInterval time.Duration // Determines frequency of pushing state info messages to peers
	RequestStateInfoInterval time.Duration // Determines frequency of pulling state info messages from peers

	TLSCerts *common.TLSCertificates // TLS certificates of the peer

	InternalEndpoint string // Endpoint we publish to peers in our organization
	ExternalEndpoint string // Peer publishes this endpoint instead of SelfEndpoint to foreign organizations
}
