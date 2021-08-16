/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package deliverclient

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/comm"
	"github.com/hyperledger/fabric/core/deliverservice/blocksprovider"
	"github.com/hyperledger/fabric/gossip/api"
	"github.com/hyperledger/fabric/gossip/util"
	"github.com/hyperledger/fabric/protos/orderer"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var logger *logging.Logger // package-level logger

func init() {
	logger = flogging.MustGetLogger("deliveryClient")
}

const (
	defaultReConnectTotalTimeThreshold = time.Second * 60 * 60
)

var (
	connTimeout               = time.Second * 3
	reConnectBackoffThreshold = float64(time.Hour)
)

func getReConnectTotalTimeThreshold() time.Duration {
	return util.GetDurationOrDefault("peer.deliveryclient.reconnectTotalTimeThreshold", defaultReConnectTotalTimeThreshold)
}

// DeliverService used to communicate with orderers to obtain
// new blocks and send them to the committer service
/*
    DeliverService用来与orderers交流以获得新的区块，并且将新的区块发送到committer service
 */
type DeliverService interface {
	// StartDeliverForChannel dynamically starts delivery of new blocks from ordering service
	// to channel peers.
	// When the delivery finishes, the finalizer func is called
	//StartDeliverForChannel 动态地将从Ordering Service获取的区块发送到channel peers，当delivery结束，finalizer函数被调用
	StartDeliverForChannel(chainID string, ledgerInfo blocksprovider.LedgerInfo, finalizer func()) error

	// StopDeliverForChannel dynamically stops delivery of new blocks from ordering service
	// to channel peers.
	//StopDeliverForChannel 停止channel的block provider来停止传递区块
	StopDeliverForChannel(chainID string) error

	// UpdateEndpoints
	//更新所连接的Ordering Service的端点
	UpdateEndpoints(chainID string, endpoints []string) error

	// Stop terminates delivery service and closes the connection
	//Stop 终止delivery服务并关闭 connection
	Stop()
}

// deliverServiceImpl the implementation of the delivery service
// maintains connection to the ordering service and maps of
// blocks providers
type deliverServiceImpl struct {
	conf           *Config
	blockProviders map[string]blocksprovider.BlocksProvider
	lock           sync.RWMutex
	stopping       bool
}

// Config dictates the DeliveryService's properties,
// namely how it connects to an ordering service endpoint,
// how it verifies messages received from it,
// and how it disseminates the messages to other peers
type Config struct {
	// ConnFactory returns a function that creates a connection to an endpoint
	ConnFactory func(channelID string) func(endpoint string) (*grpc.ClientConn, error)
	// ABCFactory creates an AtomicBroadcastClient out of a connection
	ABCFactory func(*grpc.ClientConn) orderer.AtomicBroadcastClient
	// CryptoSvc performs cryptographic actions like message verification and signing
	// and identity validation
	CryptoSvc api.MessageCryptoService
	// Gossip enables to enumerate peers in the channel, send a message to peers,
	// and add a block to the gossip state transfer layer
	Gossip blocksprovider.GossipServiceAdapter
	// Endpoints specifies the endpoints of the ordering service
	Endpoints []string
}

// NewDeliverService construction function to create and initialize
// delivery service instance. It tries to establish connection to
// the specified in the configuration ordering service, in case it
// fails to dial to it, return nil
func NewDeliverService(conf *Config) (DeliverService, error) {
	ds := &deliverServiceImpl{
		conf:           conf,
		blockProviders: make(map[string]blocksprovider.BlocksProvider),
	}
	if err := ds.validateConfiguration(); err != nil {
		return nil, err
	}
	return ds, nil
}

func (d *deliverServiceImpl) UpdateEndpoints(chainID string, endpoints []string) error {
	// Use chainID to obtain blocks provider and pass endpoints
	// for update
	if bp, ok := d.blockProviders[chainID]; ok {
		// We have found specified channel so we can safely update it
		bp.UpdateOrderingEndpoints(endpoints)
		return nil
	}
	return errors.New(fmt.Sprintf("Channel with %s id was not found", chainID))
}

func (d *deliverServiceImpl) validateConfiguration() error {
	conf := d.conf
	if len(conf.Endpoints) == 0 {
		return errors.New("No endpoints specified")
	}
	if conf.Gossip == nil {
		return errors.New("No gossip provider specified")
	}
	if conf.ABCFactory == nil {
		return errors.New("No AtomicBroadcast factory specified")
	}
	if conf.ConnFactory == nil {
		return errors.New("No connection factory specified")
	}
	if conf.CryptoSvc == nil {
		return errors.New("No crypto service specified")
	}
	return nil
}

// StartDeliverForChannel starts blocks delivery for channel
// initializes the grpc stream for given chainID, creates blocks provider instance
// that spawns in go routine to read new blocks starting from the position provided by ledger
// info instance.
func (d *deliverServiceImpl) StartDeliverForChannel(chainID string, ledgerInfo blocksprovider.LedgerInfo, finalizer func()) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	/*
	首先判断deliverService实例是否已经停止（stopping）
	 */
	if d.stopping {
		errMsg := fmt.Sprintf("Delivery service is stopping cannot join a new channel %s", chainID)
		logger.Errorf(errMsg)
		return errors.New(errMsg)
	}
	//或者对于某个channel已经存在blockProvider,若停止或者对于某个channel已经存在blockProvider怎么返回相应的错误，若既没有停止也不存在相应的blockProvider，则

	if _, exist := d.blockProviders[chainID]; exist {
		errMsg := fmt.Sprintf("Delivery service - block provider already exists for %s found, can't start delivery", chainID)
		logger.Errorf(errMsg)
		return errors.New(errMsg)
	} else {
		//构建新的blockProvider实例
		client := d.newClient(chainID, ledgerInfo)
		logger.Debug("This peer will pass blocks from orderer service to other peers for channel", chainID)
		d.blockProviders[chainID] = blocksprovider.NewBlocksProvider(chainID, client, d.conf.Gossip, d.conf.CryptoSvc)
		//开启一个goroutine调用该实例的DeliverBlocks方法，并开始从Ordering Service拉取区块
		go func() {
			d.blockProviders[chainID].DeliverBlocks()
			finalizer()
		}()
	}
	return nil
}

// StopDeliverForChannel stops blocks delivery for channel by stopping channel block provider
func (d *deliverServiceImpl) StopDeliverForChannel(chainID string) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	if d.stopping {
		errMsg := fmt.Sprintf("Delivery service is stopping, cannot stop delivery for channel %s", chainID)
		logger.Errorf(errMsg)
		return errors.New(errMsg)
	}
	if client, exist := d.blockProviders[chainID]; exist {
		client.Stop()
		delete(d.blockProviders, chainID)
		logger.Debug("This peer will stop pass blocks from orderer service to other peers")
	} else {
		errMsg := fmt.Sprintf("Delivery service - no block provider for %s found, can't stop delivery", chainID)
		logger.Errorf(errMsg)
		return errors.New(errMsg)
	}
	return nil
}

// Stop all service and release resources
func (d *deliverServiceImpl) Stop() {
	d.lock.Lock()
	defer d.lock.Unlock()
	// Marking flag to indicate the shutdown of the delivery service
	d.stopping = true

	for _, client := range d.blockProviders {
		client.Stop()
	}
}

func (d *deliverServiceImpl) newClient(chainID string, ledgerInfoProvider blocksprovider.LedgerInfo) *broadcastClient {
	requester := &blocksRequester{
		tls:     comm.TLSEnabled(),
		chainID: chainID,
	}
	broadcastSetup := func(bd blocksprovider.BlocksDeliverer) error {
		return requester.RequestBlocks(ledgerInfoProvider)
	}
	backoffPolicy := func(attemptNum int, elapsedTime time.Duration) (time.Duration, bool) {
		if elapsedTime.Nanoseconds() > getReConnectTotalTimeThreshold().Nanoseconds() {
			return 0, false
		}
		sleepIncrement := float64(time.Millisecond * 500)
		attempt := float64(attemptNum)
		return time.Duration(math.Min(math.Pow(2, attempt)*sleepIncrement, reConnectBackoffThreshold)), true
	}
	connProd := comm.NewConnectionProducer(d.conf.ConnFactory(chainID), d.conf.Endpoints)
	bClient := NewBroadcastClient(connProd, d.conf.ABCFactory, broadcastSetup, backoffPolicy)
	requester.client = bClient
	return bClient
}

func DefaultConnectionFactory(channelID string) func(endpoint string) (*grpc.ClientConn, error) {
	return func(endpoint string) (*grpc.ClientConn, error) {
		dialOpts := []grpc.DialOption{grpc.WithBlock()}
		// set max send/recv msg sizes
		dialOpts = append(dialOpts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(comm.MaxRecvMsgSize()),
			grpc.MaxCallSendMsgSize(comm.MaxSendMsgSize())))
		// set the keepalive options
		kaOpts := comm.DefaultKeepaliveOptions()
		if viper.IsSet("peer.keepalive.deliveryClient.interval") {
			kaOpts.ClientInterval = viper.GetDuration(
				"peer.keepalive.deliveryClient.interval")
		}
		if viper.IsSet("peer.keepalive.deliveryClient.timeout") {
			kaOpts.ClientTimeout = viper.GetDuration(
				"peer.keepalive.deliveryClient.timeout")
		}
		dialOpts = append(dialOpts, comm.ClientKeepaliveOptions(kaOpts)...)

		if comm.TLSEnabled() {
			creds, err := comm.GetCredentialSupport().GetDeliverServiceCredentials(channelID)
			if err != nil {
				return nil, fmt.Errorf("Failed obtaining credentials for channel %s: %v", channelID, err)
			}
			dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
		} else {
			dialOpts = append(dialOpts, grpc.WithInsecure())
		}
		grpc.EnableTracing = true
		ctx := context.Background()
		ctx, _ = context.WithTimeout(ctx, connTimeout)
		return grpc.DialContext(ctx, endpoint, dialOpts...)
	}
}

func DefaultABCFactory(conn *grpc.ClientConn) orderer.AtomicBroadcastClient {
	return orderer.NewAtomicBroadcastClient(conn)
}
