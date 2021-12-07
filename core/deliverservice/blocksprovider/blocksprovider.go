/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blocksprovider

import (
	"github.com/hyperledger/fabric/protos/msp"
	"github.com/hyperledger/fabric/protos/utils"
	"math"
	"os"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/gossip/api"
	gossipcommon "github.com/hyperledger/fabric/gossip/common"
	"github.com/hyperledger/fabric/gossip/discovery"
	"github.com/hyperledger/fabric/gossip/util"
	"github.com/hyperledger/fabric/protos/common"
	gossip_proto "github.com/hyperledger/fabric/protos/gossip"
	"github.com/hyperledger/fabric/protos/orderer"
)

// LedgerInfo an adapter to provide the interface to query
// the ledger committer for current ledger height
type LedgerInfo interface {
	// LedgerHeight returns current local ledger height
	LedgerHeight() (uint64, error)
}

// GossipServiceAdapter serves to provide basic functionality
// required from gossip service by delivery service
type GossipServiceAdapter interface {
	// PeersOfChannel returns slice with members of specified channel
	PeersOfChannel(gossipcommon.ChainID) []discovery.NetworkMember

	// AddPayload adds payload to the local state sync buffer
	AddPayload(chainID string, payload *gossip_proto.Payload) error

	// Gossip the message across the peers
	Gossip(msg *gossip_proto.GossipMessage)
}

// BlocksProvider used to read blocks from the ordering service
// for specified chain it subscribed to
type BlocksProvider interface {
	// DeliverBlocks starts delivering and disseminating blocks
	DeliverBlocks()

	// UpdateClientEndpoints update endpoints
	UpdateOrderingEndpoints(endpoints []string)

	// Stop shutdowns blocks provider and stops delivering new blocks
	Stop()
}

// BlocksDeliverer defines interface which actually helps
// to abstract the AtomicBroadcast_DeliverClient with only
// required method for blocks provider.
// This also decouples the production implementation of the gRPC stream
// from the code in order for the code to be more modular and testable.
type BlocksDeliverer interface {
	// Recv retrieves a response from the ordering service
	Recv() (*orderer.DeliverResponse, error)

	// Send sends an envelope to the ordering service
	Send(*common.Envelope) error
}

type streamClient interface {
	BlocksDeliverer

	// UpdateEndpoint update ordering service endpoints
	UpdateEndpoints(endpoints []string)

	// GetEndpoints
	GetEndpoints() []string

	// Close closes the stream and its underlying connection
	Close()

	// Disconnect disconnects from the remote node and disable reconnect to current endpoint for predefined period of time
	Disconnect(disableEndpoint bool)
}

// blocksProviderImpl the actual implementation for BlocksProvider interface
type blocksProviderImpl struct {
	chainID string

	client streamClient

	gossip GossipServiceAdapter

	mcs api.MessageCryptoService

	done int32

	wrongStatusThreshold int
}

const wrongStatusThreshold = 10

var maxRetryDelay = time.Second * 10
var logger = flogging.MustGetLogger("blocksProvider")

// NewBlocksProvider constructor function to create blocks deliverer instance
func NewBlocksProvider(chainID string, client streamClient, gossip GossipServiceAdapter, mcs api.MessageCryptoService) BlocksProvider {
	//logger.Info("==NewBlocksProvider==")
	return &blocksProviderImpl{
		chainID:              chainID,
		client:               client,
		gossip:               gossip,
		mcs:                  mcs,
		wrongStatusThreshold: wrongStatusThreshold,
	}

}

// DeliverBlocks used to pull out blocks from the ordering service to
// distributed them across peers

func (b *blocksProviderImpl) DeliverBlocks() {
	mspId := os.Getenv("CORE_PEER_LOCALMSPID")
	//要存到数据库中去
	logger.Infof("=============mspId:%v==========================", mspId)
	//=============mspId:Org1MSP==========================

	logger.Info("===========b.chainID==========",b.chainID)
	logger.Info("=================common.PolicyOrgName[b.chainID]====================",common.PolicyOrgName[b.chainID])

	//logger.Info("==blocksProviderImpl==DeliverBlocks==")
	/*
		peer0.org1.example.com自动接收的
	*/
	errorStatusCounter := 0
	statusCounter := 0
	defer b.client.Close()
	for !b.isDone() {
		msg, err := b.client.Recv()
		logger.Info("========msg============", msg)
		/*
		 block:<header:<number:3 previous_hash:"]\013\014\254\370K\214\327\341\355\275\223\231\231\014\327gO\310\0053\370m&\314\307~\216%l\313\022" data_hash:"MqT\216VK\2015\215\005R\216e\347\t\360\212A5\331\244\271\204\203\201\200#\3247\273z\222" > data:<data:"\n\313\026\n\276\007\nf\010\003\032\013\010\345\255\267\215\006\020\366\263\264)\"\tmychannel*@d62823023f73ae8d2495f3c5bbc47e1a3c42eeb3566bf3b1e412142c34ebdf74:\010\022\006\022\004mycc\022\323\006\n\266\006\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAdCgAwIBAgIQdoMtXcRmr8XdkhTXGAwFzDAKBggqhkjOPQQDAjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEcMBoGA1UEAxMTY2Eu\nb3JnMS5leGFtcGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUwMDBa\nMGwxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMQ8wDQYDVQQLEwZjbGllbnQxHzAdBgNVBAMMFkFkbWluQG9y\nZzEuZXhhbXBsZS5jb20wWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAR4lTOcEwH/\n7BA0PJHXxXCQWTvUHqLrw/m1noiFjGZa7wQWZ1NQ+AbOJ8tydWS3xu5YVw3KegfZ\naXcQ2VMAAYsIo00wSzAOBgNVHQ8BAf8EBAMCB4AwDAYDVR0TAQH/BAIwADArBgNV\nHSMEJDAigCBcpldgEIiaAL2L5u/dXT77TRUFw4c4xvgl33Q0ZMUW5jAKBggqhkjO\nPQQDAgNHADBEAiBKZAEOoM00vmTIR0pP7XFBp41hvVL4PGwISqi0v4AwTgIgUB5T\n6gaVVogvaUeMmCqljNGNupyBj1iY0jxDlvGteNA=\n-----END CERTIFICATE-----\n\022\030u\306\035\364C;\205\304l>c;\362]e|\306\362\334P\202\255q\340\022\207\017\n\204\017\n\323\006\n\266\006\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAdCgAwIBAgIQdoMtXcRmr8XdkhTXGAwFzDAKBggqhkjOPQQDAjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEcMBoGA1UEAxMTY2Eu\nb3JnMS5leGFtcGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUwMDBa\nMGwxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMQ8wDQYDVQQLEwZjbGllbnQxHzAdBgNVBAMMFkFkbWluQG9y\nZzEuZXhhbXBsZS5jb20wWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAR4lTOcEwH/\n7BA0PJHXxXCQWTvUHqLrw/m1noiFjGZa7wQWZ1NQ+AbOJ8tydWS3xu5YVw3KegfZ\naXcQ2VMAAYsIo00wSzAOBgNVHQ8BAf8EBAMCB4AwDAYDVR0TAQH/BAIwADArBgNV\nHSMEJDAigCBcpldgEIiaAL2L5u/dXT77TRUFw4c4xvgl33Q0ZMUW5jAKBggqhkjO\nPQQDAgNHADBEAiBKZAEOoM00vmTIR0pP7XFBp41hvVL4PGwISqi0v4AwTgIgUB5T\n6gaVVogvaUeMmCqljNGNupyBj1iY0jxDlvGteNA=\n-----END CERTIFICATE-----\n\022\030u\306\035\364C;\205\304l>c;\362]e|\306\362\334P\202\255q\340\022\253\010\n\"\n \n\036\010\001\022\006\022\004mycc\032\022\n\006invoke\n\001a\n\001b\n\00210\022\204\010\n}\n \232\272\314\r\\\324~\352\371\322\031mi\036\022\354E\236(\316\034\331\224\356\033\316B\023\272]\227\353\022Y\nE\022\024\n\004lscc\022\014\n\n\n\004mycc\022\002\010\001\022-\n\004mycc\022%\n\007\n\001a\022\002\010\002\n\007\n\001b\022\002\010\002\032\007\n\001a\032\00280\032\010\n\001b\032\003220\032\003\010\310\001\"\013\022\004mycc\032\0032.0\022\202\007\n\266\006\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKDCCAc+gAwIBAgIRAIXP/C0maAQHX5XjgtvGLLQwCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMjAyMDc1MDAwWhcNMzExMTMwMDc1MDAw\nWjBqMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzENMAsGA1UECxMEcGVlcjEfMB0GA1UEAxMWcGVlcjAub3Jn\nMS5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABLy3SNbSDT+T\nl4uVYEo2ctnKQbcyZ6mR6CgQq6UuDU3Yf4DxGNYiv3ubkpG7VU7cyC8EGJpcIdrH\nOdtoxO2ZEeujTTBLMA4GA1UdDwEB/wQEAwIHgDAMBgNVHRMBAf8EAjAAMCsGA1Ud\nIwQkMCKAIFymV2AQiJoAvYvm791dPvtNFQXDhzjG+CXfdDRkxRbmMAoGCCqGSM49\nBAMCA0cAMEQCIDqhk3Alg+FixH7YWhNWqqRzEE2Lqg7sdSWxGGOndidhAiAx3UN6\nHoo2YstH+JVP8r/6qjaAdkPTgUCLnpRlflNnxQ==\n-----END CERTIFICATE-----\n\022G0E\002!\000\306\255\225\214\026\351\337I\263\324*\300gJ\351\307\345F\361\270\332TB\265\314\030eS\216\234v\241\002 ^\355\245\237\220\271\221\263&I[\375\272-A\026d\253M\006\270.\026\315\221\226\t\264 W\0135\022G0E\002!\000\310!\204\346\253j\333\336\n\307\033d|\013p{!\306\274\267\200\273 \261\216{-\327\016\353\274\005\002 b\215L*U\312yc\351\275\224\263&\221\\,z\277[5~qG\266\260yW2\021A\315Y" > metadata:<metadata:"\022\370\006\n\255\006\n\220\006\n\nOrdererMSP\022\201\006-----BEGIN CERTIFICATE-----\nMIICDDCCAbKgAwIBAgIQO7f5H/GNWl4bCFrav0m09TAKBggqhkjOPQQDAjBpMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEUMBIGA1UEChMLZXhhbXBsZS5jb20xFzAVBgNVBAMTDmNhLmV4YW1w\nbGUuY29tMB4XDTIxMTIwMjA3NTAwMFoXDTMxMTEzMDA3NTAwMFowWDELMAkGA1UE\nBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBGcmFuY2lz\nY28xHDAaBgNVBAMTE29yZGVyZXIuZXhhbXBsZS5jb20wWTATBgcqhkjOPQIBBggq\nhkjOPQMBBwNCAAQ50oX9JKkYpZg+CKMMHRBR9nWjdOLgD9yHfpiM01f+L64b1eEg\nBpwz0ci6KItxvX+j81Q4dRjV0/Tw/eSI7fllo00wSzAOBgNVHQ8BAf8EBAMCB4Aw\nDAYDVR0TAQH/BAIwADArBgNVHSMEJDAigCAiKUD447X2wODjavJYziGH+yguMn94\nPA285AcRrKlxzzAKBggqhkjOPQQDAgNIADBFAiEAvBTKhKJ1q9+Tto05f7a01B0V\n3hORQui4xrriz0G5Z1MCID6i4ksld0Nzhr+45XgZRcMxC+Q8SzEpbXWJDTTo+tIl\n-----END CERTIFICATE-----\n\022\030\334\"!En\264N[\324\302\310^\303\242\334\317\003\356\210{1~\362$\022F0D\002 FEo\301u\241\233 \321\233s6?\007\325\303\013\014\247%\227W\361\360\347\254\202z\t\274)\277\002 /\001\025}\327\324\323\325\027\307\3419h\312/\373[\342\277\017\373$Jih\244`\266!,\177\263" metadata:"\022\371\006\n\255\006\n\220\006\n\nOrdererMSP\022\201\006-----BEGIN CERTIFICATE-----\nMIICDDCCAbKgAwIBAgIQO7f5H/GNWl4bCFrav0m09TAKBggqhkjOPQQDAjBpMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEUMBIGA1UEChMLZXhhbXBsZS5jb20xFzAVBgNVBAMTDmNhLmV4YW1w\nbGUuY29tMB4XDTIxMTIwMjA3NTAwMFoXDTMxMTEzMDA3NTAwMFowWDELMAkGA1UE\nBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBGcmFuY2lz\nY28xHDAaBgNVBAMTE29yZGVyZXIuZXhhbXBsZS5jb20wWTATBgcqhkjOPQIBBggq\nhkjOPQMBBwNCAAQ50oX9JKkYpZg+CKMMHRBR9nWjdOLgD9yHfpiM01f+L64b1eEg\nBpwz0ci6KItxvX+j81Q4dRjV0/Tw/eSI7fllo00wSzAOBgNVHQ8BAf8EBAMCB4Aw\nDAYDVR0TAQH/BAIwADArBgNVHSMEJDAigCAiKUD447X2wODjavJYziGH+yguMn94\nPA285AcRrKlxzzAKBggqhkjOPQQDAgNIADBFAiEAvBTKhKJ1q9+Tto05f7a01B0V\n3hORQui4xrriz0G5Z1MCID6i4ksld0Nzhr+45XgZRcMxC+Q8SzEpbXWJDTTo+tIl\n-----END CERTIFICATE-----\n\022\030\235\235\377\271\336\344\t\037[y\023\372\350\207\304\275:D\363\372\360\243'-\022G0E\002!\000\355\250\2674Ho\351\251\372\274\337\013\334\325\254\021\206\304\256Y\362\307\014\374\017\\\331\330\341D\215]\002 P\\J\021\r\3626k\312\201s\030Q\356&@\"\327o\321\233c\207S<Gx\034\016\020?\213" metadata:"" metadata:"" > >
		*/

		if err != nil {
			logger.Warningf("[%s] Receive error: %s", b.chainID, err.Error())
			return
		}
		switch t := msg.Type.(type) {
		case *orderer.DeliverResponse_Status:
			logger.Info("=========case *orderer.DeliverResponse_Status========================")
			if t.Status == common.Status_SUCCESS {
				logger.Infof("[%s] ERROR! Received success for a seek that should never complete", b.chainID)
				return
			}
			if t.Status == common.Status_BAD_REQUEST || t.Status == common.Status_FORBIDDEN {
				logger.Errorf("[%s] Got error %v", b.chainID, t)
				errorStatusCounter++
				if errorStatusCounter > b.wrongStatusThreshold {
					logger.Criticalf("[%s] Wrong statuses threshold passed, stopping block provider", b.chainID)
					return
				}
			} else {
				errorStatusCounter = 0
				logger.Warningf("[%s] Got error %v", b.chainID, t)
			}
			maxDelay := float64(maxRetryDelay)
			currDelay := float64(time.Duration(math.Pow(2, float64(statusCounter))) * 100 * time.Millisecond)
			time.Sleep(time.Duration(math.Min(maxDelay, currDelay)))
			if currDelay < maxDelay {
				statusCounter++
			}
			if t.Status == common.Status_BAD_REQUEST {
				b.client.Disconnect(false)
			} else {
				b.client.Disconnect(true)
			}
			continue
		case *orderer.DeliverResponse_Block:
			logger.Info("=========case *orderer.DeliverResponse_Block========================")
			errorStatusCounter = 0
			statusCounter = 0
			blockNum := t.Block.Header.Number

			logger.Infof("=============t.Block.Header.Number:%v===================", t.Block.Header.Number)
			marshaledBlock, err := proto.Marshal(t.Block)
			if err != nil {
				logger.Errorf("[%s] Error serializing block with sequence number %d, due to %s", b.chainID, blockNum, err)
				continue
			}
			if err := b.mcs.VerifyBlock(gossipcommon.ChainID(b.chainID), blockNum, marshaledBlock); err != nil {
				logger.Errorf("[%s] Error verifying block with sequnce number %d, due to %s", b.chainID, blockNum, err)
				continue
			}

			/*
			number = 1
			 */

			//if blockNum == 0 {
			//	logger.Info("============blockNum==0==================")
			//	//遵循规则
			//	for index, _ := range t.Block.Data.Data {
			//		en, err := utils.ExtractEnvelope(t.Block, index)
			//		if err != nil {
			//			logger.Error(err)
			//			return
			//		}
			//		pa, err := utils.ExtractPayload(en)
			//		if err != nil {
			//			logger.Error(err)
			//			return
			//		}
			//
			//		a, err := utils.UnmarshalChannelHeader(pa.Header.ChannelHeader)
			//		if a == nil {
			//			logger.Error(err)
			//			return
			//		}
			//		logger.Infof("====channelHeader:%v===========================", *a)
			//		/*
			//			====channelHeader:{1 0 seconds:1638759177  mychannel  0 [] [] Org1MSP Org1MSP {} [] 0}
			//		*/
			//		logger.Infof("=================channelHeader.Type:%v=========", a.Type)      //1
			//		logger.Infof("=================channelHeader.OrgPki:%v=======", a.OrgPki)    //Org1MSP
			//		logger.Infof("=================channelHeader.OrgName:%v======", a.OrgName)   //Org1MSP
			//		logger.Infof("=================channelHeader.ChannelId:%v====", a.ChannelId) //mychannel
			//		logger.Infof("=================channelHeader.Txid:%v=========", a.TxId)
			//		logger.Infof("=================channelHeader.Timestamp:%v====", a.Timestamp)
			//		//1638759177 second
			//		//2021-12-06 10:52:57
			//		logger.Info("===========主节点名===========", a.OrgName)
			//		common.PolicyOrgName[a.ChannelId] = a.OrgName
			//		////1638759177 second
			//		////2021-12-06 10:52:57
			//	}
			//	//block.Data.Data = bd.Data
			//}

			logger.Info("================len(t.Block.Data.Data):%v=============",len(t.Block.Data.Data))
			logger.Infof("======common.PolicyOrgName:%v==========",common.PolicyOrgName)//map[]

			if blockNum > 1 {
				logger.Infof("=================blockNun:%v==================", blockNum)
				//遵循规则
				var bd common.BlockData
				for index, envelopBytes := range t.Block.Data.Data {
					en, err := utils.ExtractEnvelope(t.Block, index)
					if err != nil {
						logger.Error(err)
						return
					}
					pa, err := utils.ExtractPayload(en)
					if err != nil {
						logger.Error(err)
						return
					}

					a, err := utils.UnmarshalChannelHeader(pa.Header.ChannelHeader)
					if a == nil {
						logger.Error(err)
						return
					}
					logger.Infof("===========channelHeader:%v======================", *a)
					/*
						====channelHeader:{1 0 seconds:1638759177  mychannel  0 [] [] Org1MSP Org1MSP {} [] 0}
					*/
					logger.Infof("===================channelHeader.Type:%v=========", a.Type)      //1
					logger.Infof("===================channelHeader.OrgPki:%v=======", a.OrgPki)    //Org1MSP
					logger.Infof("===================channelHeader.OrgName:%v======", a.OrgName)   //Org1MSP
					logger.Infof("===================channelHeader.ChannelId:%v====", a.ChannelId) //mychannel
					logger.Infof("===================channelHeader.Txid:%v=========", a.TxId)
					logger.Infof("===================channelHeader.Timestamp:%v====", a.Timestamp)
					//1638759177 second
					//2021-12-06 10:52:57
					//common.PolicyOrgName[a.ChannelId] = a.OrgName
					payloadSignatureHeader := &common.SignatureHeader{}
					err = proto.Unmarshal(pa.Header.SignatureHeader, payloadSignatureHeader)
					creator := payloadSignatureHeader.Creator

					sid := &msp.SerializedIdentity{}
					err = proto.Unmarshal(creator, sid)
					logger.Infof("==============发送者组织名:=========================", sid.Mspid)

					master := common.PolicyOrgName[a.ChannelId]
					request_singer := mspId
					txid_signed := sid.Mspid
					logger.Info("===========主节点名===========", master)
					logger.Info("==========request_singer==========", mspId)
					logger.Info("==========txid_signed==========", sid.Mspid)

					if request_singer != master {
						if request_singer != txid_signed {
							logger.Infof("===========3.%v区块交易没有获取权限==================", request_singer)
							continue
						}
					}
					bd.Data = append(bd.Data, envelopBytes)
				}
				t.Block.Data.Data = bd.Data
			}
			logger.Infof("================len(t.Block.Data.Data):%v=============",len(t.Block.Data.Data))//0

			/*
				遍历区块，比较creator.MSP和mspId的关系
				如果不是自己的包就丢弃
			*/

			numberOfPeers := len(b.gossip.PeersOfChannel(gossipcommon.ChainID(b.chainID)))
			// Create payload with a block received

			newBlockBytes, err := proto.Marshal(t.Block)
			if err != nil{
				return
			}
			payload := createPayload(blockNum, newBlockBytes)


			//payload := createPayload(blockNum, marshaledBlock)
			// Use payload to create gossip message
			gossipMsg := createGossipMsg(b.chainID, payload)

			logger.Debugf("[%s] Adding payload to local buffer, blockNum = [%d]", b.chainID, blockNum)
			// Add payload to local state payloads buffer
			if err := b.gossip.AddPayload(b.chainID, payload); err != nil {
				logger.Warningf("Block [%d] received from ordering service wasn't added to payload buffer: %v", blockNum, err)
			}

			// Gossip messages with other nodes
			logger.Debugf("[%s] Gossiping block [%d], peers number [%d]", b.chainID, blockNum, numberOfPeers)
			if !b.isDone() {
				b.gossip.Gossip(gossipMsg)
			}
		default:
			logger.Warningf("[%s] Received unknown: %v", b.chainID, t)
			return
		}
	}
}

// Stop stops blocks delivery provider
func (b *blocksProviderImpl) Stop() {
	//logger.Info("==blocksProviderImpl==Stop==")
	atomic.StoreInt32(&b.done, 1)
	b.client.Close()
}

// UpdateOrderingEndpoints update endpoints of ordering service
func (b *blocksProviderImpl) UpdateOrderingEndpoints(endpoints []string) {
	//logger.Info("==blocksProviderImpl==UpdateOrderingEndpoints==")
	if !b.isEndpointsUpdated(endpoints) {
		// No new endpoints for ordering service were provided
		return
	}
	// We have got new set of endpoints, updating client
	logger.Infof("Updating endpoint, to %s", endpoints)
	b.client.UpdateEndpoints(endpoints)
	logger.Debug("Disconnecting so endpoints update will take effect")
	// We need to disconnect the client to make it reconnect back
	// to newly updated endpoints
	b.client.Disconnect(false)
}
func (b *blocksProviderImpl) isEndpointsUpdated(endpoints []string) bool {
	//logger.Info("==blocksProviderImpl==isEndpointsUpdated==")
	if len(endpoints) != len(b.client.GetEndpoints()) {
		return true
	}
	// Check that endpoints was actually updated
	for _, endpoint := range endpoints {
		//logger.Info("=====endpoint=========")
		//logger.Info("=========b.client.GetEndpoints==========",b.client.GetEndpoints())
		if !util.Contains(endpoint, b.client.GetEndpoints()) {
			// Found new endpoint
			return true
		}
	}
	// Nothing has changed
	return false
}

// Check whenever provider is stopped
func (b *blocksProviderImpl) isDone() bool {
	//logger.Info("==blocksProviderImpl==isDone==")
	return atomic.LoadInt32(&b.done) == 1
}

func createGossipMsg(chainID string, payload *gossip_proto.Payload) *gossip_proto.GossipMessage {
	//logger.Info("==createGossipMsg==")
	gossipMsg := &gossip_proto.GossipMessage{
		Nonce:   0,
		Tag:     gossip_proto.GossipMessage_CHAN_AND_ORG,
		Channel: []byte(chainID),
		Content: &gossip_proto.GossipMessage_DataMsg{
			DataMsg: &gossip_proto.DataMessage{
				Payload: payload,
			},
		},
	}
	return gossipMsg
}

func createPayload(seqNum uint64, marshaledBlock []byte) *gossip_proto.Payload {
	//logger.Info("==createPayload==")
	return &gossip_proto.Payload{
		Data:   marshaledBlock,
		SeqNum: seqNum,
	}
}
