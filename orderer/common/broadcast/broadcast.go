/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package broadcast

import (
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/protos/msp"
	"github.com/hyperledger/fabric/protos/utils"
	"io"
	"time"

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/orderer/common/msgprocessor"
	cb "github.com/hyperledger/fabric/protos/common"
	ab "github.com/hyperledger/fabric/protos/orderer"
	"github.com/pkg/errors"
)



var logger = flogging.MustGetLogger("orderer.common.broadcast")

//go:generate counterfeiter -o mock/channel_support_registrar.go --fake-name ChannelSupportRegistrar . ChannelSupportRegistrar

// ChannelSupportRegistrar provides a way for the Handler to look up the Support for a channel
type ChannelSupportRegistrar interface {
	// BroadcastChannelSupport returns the message channel header, whether the message is a config update
	// and the channel resources for a message or an error if the message is not a message which can
	// be processed directly (like CONFIG and ORDERER_TRANSACTION messages)
	BroadcastChannelSupport(msg *cb.Envelope) (*cb.ChannelHeader, bool, ChannelSupport, error)
}

//go:generate counterfeiter -o mock/channel_support.go --fake-name ChannelSupport . ChannelSupport

// ChannelSupport provides the backing resources needed to support broadcast on a channel
type ChannelSupport interface {
	msgprocessor.Processor
	Consenter
}

// Consenter provides methods to send messages through consensus
type Consenter interface {
	// Order accepts a message or returns an error indicating the cause of failure
	// It ultimately passes through to the consensus.Chain interface
	Order(env *cb.Envelope, configSeq uint64) error

	// Configure accepts a reconfiguration or returns an error indicating the cause of failure
	// It ultimately passes through to the consensus.Chain interface
	Configure(config *cb.Envelope, configSeq uint64) error

	// WaitReady blocks waiting for consenter to be ready for accepting new messages.
	// This is useful when consenter needs to temporarily block ingress messages so
	// that in-flight messages can be consumed. It could return error if consenter is
	// in erroneous states. If this blocking behavior is not desired, consenter could
	// simply return nil.
	WaitReady() error
}

// Handler is designed to handle connections from Broadcast AB gRPC service
type Handler struct {
	SupportRegistrar ChannelSupportRegistrar
	Metrics          *Metrics
}

// Handle reads requests from a Broadcast stream, processes them, and returns the responses to the stream
func (bh *Handler) Handle(srv ab.AtomicBroadcast_BroadcastServer) error {
	logger.Info("=======Handler=====Handle===============")
	/*
	cli -＞
	 */
	addr := util.ExtractRemoteAddress(srv.Context())
	logger.Debugf("Starting new broadcast loop for %s", addr)
	for {
		msg, err := srv.Recv()
		logger.Info("=========1=msg========")
		if msg != nil{
			pa,err := utils.ExtractPayload(msg)
			logger.Info("======err",err)
			a,err := utils.UnmarshalChannelHeader(pa.Header.ChannelHeader)
			logger.Infof("=================channelHeader:%v",*a)
			/*
			channelHeader:{2 0 seconds:1638657982  mychannel  0 [] [] org1MSP 1234 {} [] 0}
			channelHeader:{5 0 seconds:1638657983  mychannel  0 [] []   {} [] 0}

			channelHeader:{3 0 seconds:1638762592 nanos:833025942  mychannel 7a2bc5f7bbb888c097c10a5b71b9b6ee28363eb3ae48c997c1a85b72ce7f364e 0 [18 6 18 4 108 115 99 99] []   {} [] 0}
			a.Extension:[18 6 18 4 108 115 99 99] //lscc
			*/
			logger.Infof("=================a.OrgName:%v===",a.OrgName)
			logger.Infof("=================a.OrgPki:%v===",a.OrgPki)
			logger.Infof("=================a.Type:%v==",a.Type)

			logger.Infof("=================a.Version:%v====",a.Version)
			logger.Infof("=================a.ChannelId:%v=====",a.ChannelId)
			logger.Infof("=================a.Extension:%v======",a.Extension)
			logger.Infof("=================a.TxId:%v=========",a.TxId)
			payloadSignatureHeader := &cb.SignatureHeader{}
			err = proto.Unmarshal(pa.Header.SignatureHeader,payloadSignatureHeader)
			creator := payloadSignatureHeader.Creator

			sid := &msp.SerializedIdentity{}
			err = proto.Unmarshal(creator, sid)


			//type==2过来的master
			logger.Info("==========a.OrgName",a.OrgName)//Org1MSP
			if a.Type == 2 && a.OrgName == sid.Mspid{
				logger.Info("========master标记=====")
				//configUpdate
				cb.PolicyOrgName[a.ChannelId] = a.OrgName
				cb.PolicyOrgPKI[a.ChannelId] = a.OrgPki
				//pki
			}




			//type ==5 区块请求者

			if a.Type == 5 {
				//请求获取区块者
				logger.Info("========请求获取区块者标记=====")
				logger.Info("=========请求获取区块者=======",sid.Mspid)
			}else{
				logger.Info("==========交易发送者名字===",sid.Mspid)
				//Org1MSP
			}


			logger.Infof("====cb.PolicyOrgName:%v==========",cb.PolicyOrgName)
			/*
			map[mychannel:Org1MSP]
			 */


			//logger.Info("==============envelope.Payload===============",pa.Data)
			//ee,err := utils.UnmarshalEnvelope(pa.Data)
			//eee,err := utils.ExtractPayload(ee)
			//eeee,err := utils.UnmarshalChannelHeader(eee.Header.ChannelHeader)
			//
			//logger.Infof("=================channelHeader:%v",*eeee)
			//logger.Infof("=================eeee.OrgName:%v",eeee.OrgName)
			//logger.Infof("=================eeee.OrgPki:%v",eeee.OrgPki)
			//logger.Infof("=================eeee.Type:%v",eeee.Type)//2
			//logger.Infof("=================eeee.Type:%T",eeee.Type)//
			//logger.Infof("=================eeee.Version:%v",eeee.Version)//0
			//logger.Infof("=================eeee.ChannelId:%v",eeee.ChannelId)//mychannel
			//logger.Infof("================eeee.Extension:%v",eeee.Extension)//[]
			//logger.Infof("=================eeee.TxId:%v",eeee.TxId)
		}

		/*
			HeaderType_MESSAGE              HeaderType = 0
			HeaderType_CONFIG               HeaderType = 1
			HeaderType_CONFIG_UPDATE        HeaderType = 2
			HeaderType_ENDORSER_TRANSACTION HeaderType = 3
			HeaderType_ORDERER_TRANSACTION  HeaderType = 4
			HeaderType_DELIVER_SEEK_INFO    HeaderType = 5
			HeaderType_CHAINCODE_PACKAGE    HeaderType = 6
			HeaderType_PEER_ADMIN_OPERATION HeaderType = 8
			HeaderType_TOKEN_TRANSACTION    HeaderType = 9
		*/





		if err == io.EOF {
			logger.Debugf("Received EOF from %s, hangup", addr)
			return nil
		}
		if err != nil {
			logger.Warningf("Error reading from %s: %s", addr, err)
			return err
		}

		resp := bh.ProcessMessage(msg, addr)
		err = srv.Send(resp)
		if resp.Status != cb.Status_SUCCESS {
			return err
		}

		if err != nil {
			logger.Warningf("Error sending to %s: %s", addr, err)
			return err
		}
	}

}

type MetricsTracker struct {
	ValidateStartTime time.Time
	EnqueueStartTime  time.Time
	ValidateDuration  time.Duration
	ChannelID         string
	TxType            string
	Metrics           *Metrics
}

func (mt *MetricsTracker) Record(resp *ab.BroadcastResponse) {
	logger.Info("=======MetricsTracker=====Record===============")
	labels := []string{
		"status", resp.Status.String(),
		"channel", mt.ChannelID,
		"type", mt.TxType,
	}

	if mt.ValidateDuration == 0 {
		mt.EndValidate()
	}
	mt.Metrics.ValidateDuration.With(labels...).Observe(mt.ValidateDuration.Seconds())

	if mt.EnqueueStartTime != (time.Time{}) {
		enqueueDuration := time.Since(mt.EnqueueStartTime)
		mt.Metrics.EnqueueDuration.With(labels...).Observe(enqueueDuration.Seconds())
	}

	mt.Metrics.ProcessedCount.With(labels...).Add(1)
}

func (mt *MetricsTracker) BeginValidate() {
	logger.Info("=======MetricsTracker=====BeginValidate===============")
	mt.ValidateStartTime = time.Now()
}

func (mt *MetricsTracker) EndValidate() {
	logger.Info("=======MetricsTracker=====EndValidate===============")
	mt.ValidateDuration = time.Since(mt.ValidateStartTime)
}

func (mt *MetricsTracker) BeginEnqueue() {
	logger.Info("=======MetricsTracker=====BeginEnqueue===============")
	mt.EnqueueStartTime = time.Now()
}

// ProcessMessage validates and enqueues a single message
func (bh *Handler) ProcessMessage(msg *cb.Envelope, addr string) (resp *ab.BroadcastResponse) {
	logger.Info("=======Handler=====ProcessMessage===============")
	tracker := &MetricsTracker{
		ChannelID: "unknown",
		TxType:    "unknown",
		Metrics:   bh.Metrics,
	}
	defer func() {
		// This looks a little unnecessary, but if done directly as
		// a defer, resp gets the (always nil) current state of resp
		// and not the return value
		tracker.Record(resp)
	}()
	tracker.BeginValidate()




	chdr, isConfig, processor, err := bh.SupportRegistrar.BroadcastChannelSupport(msg)


	if chdr != nil {
		tracker.ChannelID = chdr.ChannelId
		tracker.TxType = cb.HeaderType(chdr.Type).String()
	}
	if err != nil {
		logger.Warningf("[channel: %s] Could not get message processor for serving %s: %s", tracker.ChannelID, addr, err)
		return &ab.BroadcastResponse{Status: cb.Status_BAD_REQUEST, Info: err.Error()}
	}

	if !isConfig {
		logger.Info("=============if !isConfig=================")
		logger.Debugf("[channel: %s] Broadcast is processing normal message from %s with txid '%s' of type %s", chdr.ChannelId, addr, chdr.TxId, cb.HeaderType_name[chdr.Type])

		configSeq, err := processor.ProcessNormalMsg(msg)
		if err != nil {
			logger.Warningf("[channel: %s] Rejecting broadcast of normal message from %s because of error: %s", chdr.ChannelId, addr, err)
			return &ab.BroadcastResponse{Status: ClassifyError(err), Info: err.Error()}
		}
		tracker.EndValidate()

		tracker.BeginEnqueue()
		if err = processor.WaitReady(); err != nil {
			logger.Warningf("[channel: %s] Rejecting broadcast of message from %s with SERVICE_UNAVAILABLE: rejected by Consenter: %s", chdr.ChannelId, addr, err)
			return &ab.BroadcastResponse{Status: cb.Status_SERVICE_UNAVAILABLE, Info: err.Error()}
		}
		//改了头部信息会导致包交易验证失败

		//收到包的人知道这个是mas
		//for channel,_:= range cb.PolicyOrgName{
		//	if channel == chdr.ChannelId {
		//		logger.Info("==============master================")
		//		pa,err := utils.ExtractPayload(msg)
		//		logger.Info("======err",err)
		//		logger.Infof("==========1.pa.Header.ChannelHeader:%v===========",pa.Header.ChannelHeader)
		//		a,err := utils.UnmarshalChannelHeader(pa.Header.ChannelHeader)
		//		if a == nil{
		//			return &ab.BroadcastResponse{Status: ClassifyError(err), Info: err.Error()}
		//		}
		//
		//		payloadSignatureHeader := &cb.SignatureHeader{}
		//		err = proto.Unmarshal(pa.Header.SignatureHeader,payloadSignatureHeader)
		//		creator := payloadSignatureHeader.Creator
		//
		//		sid := &msp.SerializedIdentity{}
		//		err = proto.Unmarshal(creator, sid)
		//		a.OrgName = cb.PolicyOrgName[a.ChannelId]
		//		a.OrgPki = sid.Mspid
		//		logger.Infof("========通道:%v,master组织:%v========",a.ChannelId,a.OrgName)
		//		logger.Info("============交易发送者组织名===========================",a.OrgPki)
		//		/*
		//		type=3 a.OrgName=Org1MSP
		//		 */
		//
		//		logger.Infof("==========2.pa.Header.ChannelHeader:%v===========",pa.Header.ChannelHeader)
		//
		//
		//		channelChannelBytes,err := utils.Marshal(a)
		//		pa.Header.ChannelHeader  = channelChannelBytes
		//		//copy(pa.Header.ChannelHeader,channelChannelBytes) //copy复制长度不一样的值时，会有问题
		//		payloadBytes,err := utils.Marshal(pa)
		//		//copy(msg.Payload,payloadBytes)
		//		msg.Payload = payloadBytes
		//		pa,err = utils.ExtractPayload(msg)
		//		logger.Info("======err",err)//nil
		//		a,err = utils.UnmarshalChannelHeader(pa.Header.ChannelHeader)
		//		if a != nil{
		//			logger.Infof("======channelHeader:%v=====",*a)
		//			/*
		//			{3 0 seconds:1638762592 nanos:833025942  mychannel 7a2bc5f7bbb888c097c10a5b71b9b6ee28363eb3ae48c997c1a85b72ce7f364e 0 [18 6 18 4 108 115 99 99] []   {} [] 0})
		//			 */
		//			logger.Infof("==========a.OrgName:%v=====",a.OrgName)
		//			logger.Infof("==========a.OrgPki:%v=====",a.OrgPki)
		//		}
		//	}
		//}
		//共识
		err = processor.Order(msg, configSeq)
		if err != nil {
			logger.Warningf("[channel: %s] Rejecting broadcast of normal message from %s with SERVICE_UNAVAILABLE: rejected by Order: %s", chdr.ChannelId, addr, err)
			return &ab.BroadcastResponse{Status: cb.Status_SERVICE_UNAVAILABLE, Info: err.Error()}
		}
	} else { // isConfig
		logger.Info("=============configUpdate=====")
		logger.Info("[channel: %s] Broadcast is processing config update message from %s", chdr.ChannelId, addr)

		config, configSeq, err := processor.ProcessConfigUpdateMsg(msg)
		if err != nil {
			logger.Warningf("[channel: %s] Rejecting broadcast of config message from %s because of error: %s", chdr.ChannelId, addr, err)
			return &ab.BroadcastResponse{Status: ClassifyError(err), Info: err.Error()}
		}


		tracker.EndValidate()

		tracker.BeginEnqueue()
		if err = processor.WaitReady(); err != nil {
			logger.Warningf("[channel: %s] Rejecting broadcast of message from %s with SERVICE_UNAVAILABLE: rejected by Consenter: %s", chdr.ChannelId, addr, err)
			return &ab.BroadcastResponse{Status: cb.Status_SERVICE_UNAVAILABLE, Info: err.Error()}
		}

		err = processor.Configure(config, configSeq)
		if err != nil {
			logger.Warningf("[channel: %s] Rejecting broadcast of config message from %s with SERVICE_UNAVAILABLE: rejected by Configure: %s", chdr.ChannelId, addr, err)
			return &ab.BroadcastResponse{Status: cb.Status_SERVICE_UNAVAILABLE, Info: err.Error()}
		}
	}

	logger.Debugf("[channel: %s] Broadcast has successfully enqueued message of type %s from %s", chdr.ChannelId, cb.HeaderType_name[chdr.Type], addr)

	return &ab.BroadcastResponse{Status: cb.Status_SUCCESS}
}

// ClassifyError converts an error type into a status code.
func ClassifyError(err error) cb.Status {
	logger.Info("=======ClassifyError===============")
	switch errors.Cause(err) {
	case msgprocessor.ErrChannelDoesNotExist:
		return cb.Status_NOT_FOUND
	case msgprocessor.ErrPermissionDenied:
		return cb.Status_FORBIDDEN
	default:
		return cb.Status_BAD_REQUEST
	}
}
