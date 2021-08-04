package producer

import (
	"fmt"
	"io"
	"time"

	"github.com/hyperledger/fabric/common/flogging"
	pb "github.com/hyperledger/fabric/protos/peer"
)

var logger = flogging.MustGetLogger("eventhub_producer")

// EventsServer implementation of the Peer service
type EventsServer struct {
}

//singleton - if we want to create multiple servers, we need to subsume events.gEventConsumers into EventsServer
var globalEventsServer *EventsServer

// NewEventsServer returns a EventsServer
func NewEventsServer(bufferSize uint, timeout time.Duration) *EventsServer {
	if globalEventsServer != nil {
		panic("Cannot create multiple event hub servers")
	}
	globalEventsServer = new(EventsServer)
	initializeEvents(bufferSize, timeout)
	//initializeCCEventProcessor(bufferSize, timeout)
	return globalEventsServer
}

// Chat implementation of the Chat bidi streaming RPC function
func (p *EventsServer) Chat(stream pb.Events_ChatServer) error {
	handler, err := newEventHandler(stream)//
	if err != nil {
		return fmt.Errorf("error creating handler during handleChat initiation: %s", err)
	}
	//根据服务端流接口stream，创建一个handler，用于处理接收客户端发送的SignedEvent类型数据
	defer handler.Stop()
	for {
		//循环接收数据，然后处理数据
		in, err := stream.Recv()//接收signedEvent类型的数据，这也是grpc双向流的标准用法，
		if err == io.EOF {
			logger.Debug("Received EOF, ending Chat")
			return nil
		}
		if err != nil {
			e := fmt.Errorf("error during Chat, stopping handler: %s", err)
			logger.Error(e.Error())
			return e
		}
		err = handler.HandleMessage(in)//处理数据
		if err != nil {
			logger.Errorf("Error handling message: %s", err)
			return err
		}
	}
}
