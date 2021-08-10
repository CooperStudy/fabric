/*
Copyright IBM Corp. 2016 All Rights Reserved.

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

package broadcast

import (
	"github.com/hyperledger/fabric/orderer/common/filter"
	cb "github.com/hyperledger/fabric/protos/common"
	ab "github.com/hyperledger/fabric/protos/orderer"
	"github.com/op/go-logging"

	"io"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/protos/utils"
)

var logger = logging.MustGetLogger("orderer/common/broadcast")

// ConfigUpdateProcessor is used to transform CONFIG_UPDATE transactions which are used to generate other envelope
// message types with preprocessing by the orderer
type ConfigUpdateProcessor interface {
	// Process takes in an envelope of type CONFIG_UPDATE and proceses it
	// to transform it either into another envelope type
	Process(envConfigUpdate *cb.Envelope) (*cb.Envelope, error)
}

// Handler defines an interface which handles broadcasts
type Handler interface {
	// Handle starts a service thread for a given gRPC connection and services the broadcast connection
	Handle(srv ab.AtomicBroadcast_BroadcastServer) error
}

// SupportManager provides a way for the Handler to look up the Support for a chain
type SupportManager interface {
	ConfigUpdateProcessor

	// GetChain gets the chain support for a given ChannelId
	GetChain(chainID string) (Support, bool)
}

// Support provides the backing resources needed to support broadcast on a chain
type Support interface {
	// Enqueue accepts a message and returns true on acceptance, or false on shutdown
	Enqueue(env *cb.Envelope) bool

	// Filters returns the set of broadcast filters for this chain
	Filters() *filter.RuleSet
}

type handlerImpl struct {
	sm SupportManager
}

// NewHandlerImpl constructs a new implementation of the Handler interface
func NewHandlerImpl(sm SupportManager) Handler {
	return &handlerImpl{
		sm: sm,
	}
}

// Handle starts a service thread for a given gRPC connection and services the broadcast connection
func (bh *handlerImpl) Handle(srv ab.AtomicBroadcast_BroadcastServer) error {
	logger.Debugf("Starting new broadcast loop")
	/*
	会在for循环中，不断接收来自peer节点的消息
	 */
	for {
		//接收消息
		msg, err := srv.Recv()
		if err == io.EOF {
			logger.Debugf("Received EOF, hangup")
			return nil
		}
		if err != nil {
			logger.Warningf("Error reading from stream: %s", err)
			return err
		}

		//检查Envelope消息中的一些字段（如ChannelId），
		payload, err := utils.UnmarshalPayload(msg.Payload)
		if err != nil {
			logger.Warningf("Received malformed message, dropping connection: %s", err)
			return srv.Send(&ab.BroadcastResponse{Status: cb.Status_BAD_REQUEST})
		}

		if payload.Header == nil {
			logger.Warningf("Received malformed message, with missing header, dropping connection")
			return srv.Send(&ab.BroadcastResponse{Status: cb.Status_BAD_REQUEST})
		}

		chdr, err := utils.UnmarshalChannelHeader(payload.Header.ChannelHeader)
		if err != nil {
			logger.Warningf("Received malformed message (bad channel header), dropping connection: %s", err)
			return srv.Send(&ab.BroadcastResponse{Status: cb.Status_BAD_REQUEST})
		}
		//如果是HeadType_CONFIG_UPDATE类型的消息，则会将消息经过
		if chdr.Type == int32(cb.HeaderType_CONFIG_UPDATE) {
			logger.Debugf("Preprocessing CONFIG_UPDATE")
			//实际上是对消息的进一步加工，将消息加工成一个orderer可以处理的交易(加工成创建新chain的配置或已存在的chain的更新配置)
			msg, err = bh.sm.Process(msg)
			if err != nil {
				logger.Warningf("Rejecting CONFIG_UPDATE because: %s", err)
				return srv.Send(&ab.BroadcastResponse{Status: cb.Status_BAD_REQUEST})
			}

			err = proto.Unmarshal(msg.Payload, payload)
			if err != nil || payload.Header == nil {
				logger.Criticalf("Generated bad transaction after CONFIG_UPDATE processing")
				return srv.Send(&ab.BroadcastResponse{Status: cb.Status_INTERNAL_SERVER_ERROR})
			}

			chdr, err = utils.UnmarshalChannelHeader(payload.Header.ChannelHeader)
			if err != nil {
				logger.Criticalf("Generated bad transaction after CONFIG_UPDATE processing (bad channel header): %s", err)
				return srv.Send(&ab.BroadcastResponse{Status: cb.Status_INTERNAL_SERVER_ERROR})
			}

			if chdr.ChannelId == "" {
				logger.Criticalf("Generated bad transaction after CONFIG_UPDATE processing (empty channel ID)")
				return srv.Send(&ab.BroadcastResponse{Status: cb.Status_INTERNAL_SERVER_ERROR})
			}
		}

		/*
		获取消息对应的channelID的链的chainSupport
		 */
		support, ok := bh.sm.GetChain(chdr.ChannelId)
		if !ok {
			logger.Warningf("Rejecting broadcast because channel %s was not found", chdr.ChannelId)
			return srv.Send(&ab.BroadcastResponse{Status: cb.Status_NOT_FOUND})
		}

		logger.Debugf("[channel: %s] Broadcast is filtering message of type %s", chdr.ChannelId, cb.HeaderType_name[chdr.Type])

		// Normal transaction for existing chain
		//使用该chainSupport的过滤器过滤该消息，以查看该消息是否达标，这米有没有使用过滤器一并返回committer集合，这是第一次过滤
		_, filterErr := support.Filters().Apply(msg)

		if filterErr != nil {
			logger.Warningf("[channel: %s] Rejecting broadcast message because of filter error: %s", chdr.ChannelId, filterErr)
			return srv.Send(&ab.BroadcastResponse{Status: cb.Status_BAD_REQUEST})
		}

		/*
		进而调用了support对应的chain的cs.chain.Enqueue(env)处理消息，这个chain是对应consenter（这里以kakfa为例进行介绍）的handleChain生成
		的（在创建chainsupport的时候在orderer/multichain/chainSupport.go的newChainSupport中，指定
		cs.chai, err :=  consenter.HandleChain(cs,metadata)

		消息进入orderer/kafka/chain.go的Enqueue(env)
		 */
		if !support.Enqueue(msg) {
			return srv.Send(&ab.BroadcastResponse{Status: cb.Status_SERVICE_UNAVAILABLE})
		}

		if logger.IsEnabledFor(logging.DEBUG) {
			logger.Debugf("[channel: %s] Broadcast has successfully enqueued message of type %s", chdr.ChannelId, cb.HeaderType_name[chdr.Type])
		}

		err = srv.Send(&ab.BroadcastResponse{Status: cb.Status_SUCCESS})
		if err != nil {
			logger.Warningf("[channel: %s] Error sending to stream: %s", chdr.ChannelId, err)
			return err
		}
	}
}
