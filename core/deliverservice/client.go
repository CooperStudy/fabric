/*
Copyright IBM Corp. 2017 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

                 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package deliverclient

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hyperledger/fabric/core/comm"
	"github.com/hyperledger/fabric/core/deliverservice/blocksprovider"
	"github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/orderer"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// broadcastSetup is a function that is called by the broadcastClient immediately after each
// successful connection to the ordering service
type broadcastSetup func(blocksprovider.BlocksDeliverer) error

// retryPolicy receives as parameters the number of times the attempt has failed
// and a duration that specifies the total elapsed time passed since the first attempt.
// If further attempts should be made, it returns:
// 	- a time duration after which the next attempt would be made, true
// Else, a zero duration, false
type retryPolicy func(attemptNum int, elapsedTime time.Duration) (time.Duration, bool)

// clientFactory creates a gRPC broadcast client out of a ClientConn
//用于生成Deliver服务实例
type clientFactory func(*grpc.ClientConn) orderer.AtomicBroadcastClient

type broadcastClient struct {
	stopFlag int32
	sync.Mutex
	stopChan     chan struct{}
	createClient clientFactory
	shouldRetry  retryPolicy
	onConnect    broadcastSetup
	prod         comm.ConnectionProducer
	blocksprovider.BlocksDeliverer
	conn *connection
}

// NewBroadcastClient returns a broadcastClient with the given params
func NewBroadcastClient(prod comm.ConnectionProducer, clFactory clientFactory, onConnect broadcastSetup, bos retryPolicy) *broadcastClient {
	//createClient被赋值的config的ABCfactory，生成的broadcastCliet是blocksProvideImpl中的成员client，即blocksProviderImpl中createClinet的值是DefualtABCFactory
	//DefaultABCFacotry使用了protos/orderer/ab.pb.go中生成的默认的AtomicBroadcastClient（包含默认的DeLiver客户端）
	return &broadcastClient{prod: prod, onConnect: onConnect, shouldRetry: bos, createClient: clFactory, stopChan: make(chan struct{}, 1)}
}

// Recv receives a message from the ordering service
// 启动client接收线程后，在client接受欧消息时，core/deliverservice/client.go中的recv（）会执行try
func (bc *broadcastClient) Recv() (*orderer.DeliverResponse, error) {
	//执行try函数，是一个执行接收动作的函数，
	o, err := bc.try(func() (interface{}, error) {
		if bc.shouldStop() {
			return nil, errors.New("closing")
		}
		//使用bc.BlocksDeliverer.Recv接收消息（此时BlocksDeliverer还没有赋值）
		return bc.BlocksDeliverer.Recv()
	})
	if err != nil {
		return nil, err
	}
	return o.(*orderer.DeliverResponse), nil
}

// Send sends a message to the ordering service
func (bc *broadcastClient) Send(msg *common.Envelope) error {
	_, err := bc.try(func() (interface{}, error) {
		if bc.shouldStop() {
			return nil, errors.New("closing")
		}
		return nil, bc.BlocksDeliverer.Send(msg)
	})
	return err
}

/*
  for循环不断的执行bc.doAction(action)
 */
func (bc *broadcastClient) try(action func() (interface{}, error)) (interface{}, error) {
	attempt := 0
	start := time.Now()
	var backoffDuration time.Duration
	retry := true
	for retry && !bc.shouldStop() {
		attempt++
		//进而执行bc.doAction函数,这个action为上一步传入的接收消息的动作
		resp, err := bc.doAction(action)
		if err != nil {
			backoffDuration, retry = bc.shouldRetry(attempt, time.Since(start))
			if !retry {
				break
			}
			bc.sleep(backoffDuration)
			continue
		}
		return resp, nil
	}
	if bc.shouldStop() {
		return nil, errors.New("Client is closing")
	}
	return nil, fmt.Errorf("Attempts (%d) or elapsed time (%v) exhausted", attempt, time.Since(start))
}
/*
   在doAction会查看到client是或否已经和orderer通过grpc连接上，若未连接怎会执行connect()进行grpc连接，

 */
func (bc *broadcastClient) doAction(action func() (interface{}, error)) (interface{}, error) {
	if bc.conn == nil {
		//先查看conn是否建立，如果没有建立则建立，
		err := bc.connect()//如果没有建立，则调用这个函数建立连接
		if err != nil {
			return nil, err
		}
	}

	//执行action动作，与orderer建立并发送索要请求后，继续开始执行action()，这个action即开始接收orderer节点发来的block数据（e),(j)

	/*
	   orderer的Deliever服务器端向peer节点的Deliver客服端发送block，block进入doAction
	   action接收到block，返回至func (bc *broadcastClient) try(action func() (interface{}, error)) (interface{}, error)
	   ,再返回至Recv(),探后返回core/delieverservice/blocksprovider/blockprovier.go的DeliverBlocks()中的msg，err：= b.client.Recv(),
	   最后进入case *orderer.DeliverResponse_block分支
	 */
	resp, err := action()
	if err != nil {
		bc.Disconnect()
		return nil, err
	}
	return resp, nil
}

func (bc *broadcastClient) sleep(duration time.Duration) {
	select {
	case <-time.After(duration):
	case <-bc.stopChan:
	}
}

func (bc *broadcastClient) connect() error {
	//建立一个连接orderer节点的grpc连接，
	conn, endpoint, err := bc.prod.NewConnection()
	if err != nil {
		logger.Error("Failed obtaining connection:", err)
		return err
	}
	ctx, cf := context.WithCancel(context.Background())
	//使用grpc建立一个Deliver服务客服端
	abc, err := bc.createClient(conn).Deliver(ctx)
	if err != nil {
		logger.Error("Connection to ", endpoint, "established but was unable to create gRPC stream:", err)
		conn.Close()
		return err
	}
	//随后调用了，afterConnect
	err = bc.afterConnect(conn, abc, cf)
	if err == nil {
		return nil
	}
	// If we reached here, lets make sure connection is closed
	// and nullified before we return
	bc.Disconnect()
	return err
}

func (bc *broadcastClient) afterConnect(conn *grpc.ClientConn, abc orderer.AtomicBroadcast_DeliverClient, cf context.CancelFunc) error {
	/*
	将创建与orderer节点的连接和Deliver客户端，分别赋值给
	 */
	bc.Lock()
	//broadcastClient的conn可使之后的所有doAction(...)中的connect()不在执行，
	bc.conn = &connection{ClientConn: conn, cancel: cf}
	//blocksDelierver(在（e)时BlocksDeliever还没有被赋值）
	bc.BlocksDeliverer = abc
	if bc.shouldStop() {
		bc.Unlock()
		return errors.New("closing")
	}
	bc.Unlock()
	// If the client is closed at this point- before onConnect,
	// any use of this object by onConnect would return an error.

	//然后调用bc.onConnect,这个onConnect就是（a)(b)中的BroadcastSetup,即在这里发生了索要行为，向orderer索要
	//所有的block
	err := bc.onConnect(bc)
	// If the client is closed right after onConnect, but before
	// the following lock- this method would return an error because
	// the client has been closed.
	bc.Lock()
	defer bc.Unlock()
	if bc.shouldStop() {
		return errors.New("closing")
	}
	// If the client is closed right after this method exits,
	// it's because this method returned nil and not an error.
	// So- connect() would return nil also, and the flow of the goroutine
	// is returned to doAction(), where action() is invoked - and is configured
	// to check whether the client has closed or not.
	if err == nil {
		return nil
	}
	logger.Error("Failed setting up broadcast:", err)
	//随后开始返回到doAction中
	return err
}

func (bc *broadcastClient) shouldStop() bool {
	return atomic.LoadInt32(&bc.stopFlag) == int32(1)
}

// Close makes the client close its connection and shut down
func (bc *broadcastClient) Close() {
	bc.Lock()
	defer bc.Unlock()
	if bc.shouldStop() {
		return
	}
	atomic.StoreInt32(&bc.stopFlag, int32(1))
	bc.stopChan <- struct{}{}
	if bc.conn == nil {
		return
	}
	bc.conn.Close()
}

// Disconnect makes the client close the existing connection
func (bc *broadcastClient) Disconnect() {
	bc.Lock()
	defer bc.Unlock()
	if bc.conn == nil {
		return
	}
	bc.conn.Close()
	bc.conn = nil
	bc.BlocksDeliverer = nil
}

type connection struct {
	sync.Once
	*grpc.ClientConn
	cancel context.CancelFunc
}

func (c *connection) Close() error {
	var err error
	c.Once.Do(func() {
		c.cancel()
		err = c.ClientConn.Close()
	})
	return err
}
