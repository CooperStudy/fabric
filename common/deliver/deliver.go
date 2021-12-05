/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package deliver

import (
	"context"
	"github.com/hyperledger/fabric/protos/msp"
	"io"
	"math"
	"strconv"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/crypto"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/common/ledger/blockledger"
	"github.com/hyperledger/fabric/common/policies"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/core/comm"
	cb "github.com/hyperledger/fabric/protos/common"
	ab "github.com/hyperledger/fabric/protos/orderer"
	"github.com/hyperledger/fabric/protos/utils"
	"github.com/pkg/errors"
)

var logger = flogging.MustGetLogger("common.deliver")

//go:generate counterfeiter -o mock/chain_manager.go -fake-name ChainManager . ChainManager

// ChainManager provides a way for the Handler to look up the Chain.
type ChainManager interface {
	GetChain(chainID string) Chain
}

//go:generate counterfeiter -o mock/chain.go -fake-name Chain . Chain

// Chain encapsulates chain operations and data.
type Chain interface {
	// Sequence returns the current config sequence number, can be used to detect config changes
	Sequence() uint64

	// PolicyManager returns the current policy manager as specified by the chain configuration
	PolicyManager() policies.Manager

	// Reader returns the chain Reader for the chain
	Reader() blockledger.Reader

	// Errored returns a channel which closes when the backing consenter has errored
	Errored() <-chan struct{}
}

//go:generate counterfeiter -o mock/policy_checker.go -fake-name PolicyChecker . PolicyChecker

// PolicyChecker checks the envelope against the policy logic supplied by the
// function.
type PolicyChecker interface {
	CheckPolicy(envelope *cb.Envelope, channelID string) error
}

// The PolicyCheckerFunc is an adapter that allows the use of an ordinary
// function as a PolicyChecker.
type PolicyCheckerFunc func(envelope *cb.Envelope, channelID string) error

// CheckPolicy calls pcf(envelope, channelID)
func (pcf PolicyCheckerFunc) CheckPolicy(envelope *cb.Envelope, channelID string) error {
	return pcf(envelope, channelID)
}

//go:generate counterfeiter -o mock/inspector.go -fake-name Inspector . Inspector

// Inspector verifies an appropriate binding between the message and the context.
type Inspector interface {
	Inspect(context.Context, proto.Message) error
}

// The InspectorFunc is an adapter that allows the use of an ordinary
// function as an Inspector.
type InspectorFunc func(context.Context, proto.Message) error

// Inspect calls inspector(ctx, p)
func (inspector InspectorFunc) Inspect(ctx context.Context, p proto.Message) error {
	logger.Info("======InspectorFunc===Inspect===")
	return inspector(ctx, p)
}

// Handler handles server requests.
type Handler struct {
	ChainManager     ChainManager
	TimeWindow       time.Duration
	BindingInspector Inspector
	Metrics          *Metrics
}

//go:generate counterfeiter -o mock/receiver.go -fake-name Receiver . Receiver

// Receiver is used to receive enveloped seek requests.
type Receiver interface {
	Recv() (*cb.Envelope, error)
}

//go:generate counterfeiter -o mock/response_sender.go -fake-name ResponseSender . ResponseSender

// ResponseSender defines the interface a handler must implement to send
// responses.
type ResponseSender interface {
	SendStatusResponse(status cb.Status) error
	SendBlockResponse(block *cb.Block) error
}

// Filtered is a marker interface that indicates a response sender
// is configured to send filtered blocks
type Filtered interface {
	IsFiltered() bool
}

// Server is a polymorphic structure to support generalization of this handler
// to be able to deliver different type of responses.
type Server struct {
	Receiver
	PolicyChecker
	ResponseSender
}

// ExtractChannelHeaderCertHash extracts the TLS cert hash from a channel header.
func ExtractChannelHeaderCertHash(msg proto.Message) []byte {
	logger.Info("======ExtractChannelHeaderCertHash===")
	chdr, isChannelHeader := msg.(*cb.ChannelHeader)
	if !isChannelHeader || chdr == nil {
		return nil
	}
	return chdr.TlsCertHash
}

// NewHandler creates an implementation of the Handler interface.
func NewHandler(cm ChainManager, timeWindow time.Duration, mutualTLS bool, metrics *Metrics) *Handler {
	logger.Info("======NewHandler===")
	return &Handler{
		ChainManager:     cm,
		TimeWindow:       timeWindow,
		BindingInspector: InspectorFunc(comm.NewBindingInspector(mutualTLS, ExtractChannelHeaderCertHash)),
		Metrics:          metrics,
	}
}

// Handle receives incoming deliver requests.
func (h *Handler) Handle(ctx context.Context, srv *Server) error {
	logger.Info("====Handler==Handle======收到区块处理=========")
	addr := util.ExtractRemoteAddress(ctx)
	logger.Infof("Starting new deliver loop for %s", addr)
	h.Metrics.StreamsOpened.Add(1)
	defer h.Metrics.StreamsClosed.Add(1)
	for {
		logger.Infof("Attempting to read seek info message from %s", addr)
		envelope, err := srv.Recv()
		if envelope != nil{
			logger.Info("=========2 envelope===================")
			pa,_:= utils.ExtractPayload(envelope)
			logger.Info("==============envelope.Payload===============")
			a,err := utils.UnmarshalChannelHeader(pa.Header.ChannelHeader)
			logger.Info("====err",err)
			logger.Infof("=================channelHeader:%v",*a)
			//channelHeader:{5 0 seconds:1638659036  mychannel  0 [] []   {} [] 0}
			logger.Infof("=================发送者:%v",a.OrgName)
			/*
			type= 5是请求者的creator
			 */
			logger.Infof("=================a.OrgPki:%v",a.OrgPki)
			logger.Infof("=================a.Type:%v",a.Type)//
			logger.Infof("=================a.Type:%T",a.Type)//
			logger.Infof("=================a.Version:%v",a.Version)//0
			logger.Infof("=================a.ChannelId:%v",a.ChannelId)//mychannel
			logger.Infof("=================a.Extension:%v",a.Extension)//[]
			logger.Infof("=================a.TxId",a.TxId)



			payloadSignatureHeader := &cb.SignatureHeader{}
			err = proto.Unmarshal(pa.Header.SignatureHeader,payloadSignatureHeader)
			creator := payloadSignatureHeader.Creator

			sid := &msp.SerializedIdentity{}
			err = proto.Unmarshal(creator, sid)
			/*
			请求消息的组织名id.Mspid，跟交易中的签名的组织名进行比较，如果一致就获取块
			 */
			logger.Info("==========creator.OrgName",sid.Mspid)//Org1MSP

			logger.Info("==============envelope.Payload===============",pa.Data)



			/*
			[10 2 26 0 18 2 26 0]
			 */
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
			//logger.Infof("=================eeee.TxId",eeee.TxId)


			//ppp := &cb.SignatureHeader{}
			//err = proto.Unmarshal(eee.Header.SignatureHeader,ppp)
			//creator1 := ppp.Creator
			//
			//sss:= &msp.SerializedIdentity{}
			//err = proto.Unmarshal(creator1, sss)
			//logger.Infof("==========creator.OrgName:%v===",sss.Mspid)//Org1MSP
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








		//logger.Infof("======srv.Recv=envelope=envelope.Payload:%v,envelope.Signature:%v===========",envelope.Payload,envelope.Signature)
		/*
		create Channel
		//收到请求
		[10 237 6 10 21 8 5 26 6 8 147 160 167 141 6 34 9 109 121 99 104 97 110 110 101 108 18 211 6 10 182 6 10 7 79 114 103 49 77 83 80 18 170 6 45 45 45 45 45 66 69 71 73 78 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10 77 73 73 67 75 84 67 67 65 100 67 103 65 119 73 66 65 103 73 81 100 111 77 116 88 99 82 109 114 56 88 100 107 104 84 88 71 65 119 70 122 68 65 75 66 103 103 113 104 107 106 79 80 81 81 68 65 106 66 122 77 81 115 119 10 67 81 89 68 86 81 81 71 69 119 74 86 85 122 69 84 77 66 69 71 65 49 85 69 67 66 77 75 81 50 70 115 97 87 90 118 99 109 53 112 89 84 69 87 77 66 81 71 65 49 85 69 66 120 77 78 85 50 70 117 73 69 90 121 10 89 87 53 106 97 88 78 106 98 122 69 90 77 66 99 71 65 49 85 69 67 104 77 81 98 51 74 110 77 83 53 108 101 71 70 116 99 71 120 108 76 109 78 118 98 84 69 99 77 66 111 71 65 49 85 69 65 120 77 84 89 50 69 117 10 98 51 74 110 77 83 53 108 101 71 70 116 99 71 120 108 76 109 78 118 98 84 65 101 70 119 48 121 77 84 69 121 77 68 73 119 78 122 85 119 77 68 66 97 70 119 48 122 77 84 69 120 77 122 65 119 78 122 85 119 77 68 66 97 10 77 71 119 120 67 122 65 74 66 103 78 86 66 65 89 84 65 108 86 84 77 82 77 119 69 81 89 68 86 81 81 73 69 119 112 68 89 87 120 112 90 109 57 121 98 109 108 104 77 82 89 119 70 65 89 68 86 81 81 72 69 119 49 84 10 89 87 52 103 82 110 74 104 98 109 78 112 99 50 78 118 77 81 56 119 68 81 89 68 86 81 81 76 69 119 90 106 98 71 108 108 98 110 81 120 72 122 65 100 66 103 78 86 66 65 77 77 70 107 70 107 98 87 108 117 81 71 57 121 10 90 122 69 117 90 88 104 104 98 88 66 115 90 83 53 106 98 50 48 119 87 84 65 84 66 103 99 113 104 107 106 79 80 81 73 66 66 103 103 113 104 107 106 79 80 81 77 66 66 119 78 67 65 65 82 52 108 84 79 99 69 119 72 47 10 55 66 65 48 80 74 72 88 120 88 67 81 87 84 118 85 72 113 76 114 119 47 109 49 110 111 105 70 106 71 90 97 55 119 81 87 90 49 78 81 43 65 98 79 74 56 116 121 100 87 83 51 120 117 53 89 86 119 51 75 101 103 102 90 10 97 88 99 81 50 86 77 65 65 89 115 73 111 48 48 119 83 122 65 79 66 103 78 86 72 81 56 66 65 102 56 69 66 65 77 67 66 52 65 119 68 65 89 68 86 82 48 84 65 81 72 47 66 65 73 119 65 68 65 114 66 103 78 86 10 72 83 77 69 74 68 65 105 103 67 66 99 112 108 100 103 69 73 105 97 65 76 50 76 53 117 47 100 88 84 55 55 84 82 85 70 119 52 99 52 120 118 103 108 51 51 81 48 90 77 85 87 53 106 65 75 66 103 103 113 104 107 106 79 10 80 81 81 68 65 103 78 72 65 68 66 69 65 105 66 75 90 65 69 79 111 77 48 48 118 109 84 73 82 48 112 80 55 88 70 66 112 52 49 104 118 86 76 52 80 71 119 73 83 113 105 48 118 52 65 119 84 103 73 103 85 66 53 84 10 54 103 97 86 86 111 103 118 97 85 101 77 109 67 113 108 106 78 71 78 117 112 121 66 106 49 105 89 48 106 120 68 108 118 71 116 101 78 65 61 10 45 45 45 45 45 69 78 68 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10 18 24 7 110 33 36 234 142 118 218 122 16 126 119 42 66 87 80 87 213 93 115 163 71 126 59 18 8 10 2 26 0 18 2 26 0]===========%!(EXTRA []uint8=[48 69 2 33 0 234 248 50 95 89 162 190 224 58 91 31 229 225 76 130 147 43 60 174 32 118 187 246 188 181 15 159 158 185 238 195 223 2 32 106 67 47 27 63 232 222 136 217 12 45 114 152 188 103 57 16 179 126 4 94 234 53 54 129 75 100 199 170 214 198 252])
		*/
		if err == io.EOF {
			logger.Debugf("Received EOF from %s, hangup", addr)
			return nil
		}
		if err != nil {
			logger.Warningf("Error reading from %s: %s", addr, err)
			return err
		}
		logger.Info("=====deliverBlocks==========")
		status, err := h.deliverBlocks(ctx, srv, envelope)

		if err != nil {
			return err
		}

		err = srv.SendStatusResponse(status)
		if status != cb.Status_SUCCESS {
			return err
		}
		if err != nil {
			logger.Warningf("Error sending to %s: %s", addr, err)
			return err
		}

		logger.Debugf("Waiting for new SeekInfo from %s", addr)
	}
}

func isFiltered(srv *Server) bool {
	logger.Info("====isFiltered==")
	if filtered, ok := srv.ResponseSender.(Filtered); ok {
		return filtered.IsFiltered()
	}
	return false
}

func (h *Handler) deliverBlocks(ctx context.Context, srv *Server, envelope *cb.Envelope) (status cb.Status, err error) {
	logger.Info("==Handler==deliverBlocks==")


	request_singer := ""
	txid_order := ""
	txid_signed := ""

	if envelope != nil{
		logger.Info("=========2 envelope===================")
		pa,_:= utils.ExtractPayload(envelope)
		logger.Info("==============envelope.Payload===============")
		a,err := utils.UnmarshalChannelHeader(pa.Header.ChannelHeader)
		logger.Info("====err",err)
		logger.Infof("=================channelHeader:%v",*a)
		//channelHeader:{5 0 seconds:1638659036  mychannel  0 [] []   {} [] 0}
		logger.Infof("=================a.OrgName:%v",a.OrgName)
		logger.Infof("=================a.OrgPki:%v",a.OrgPki)//
		logger.Infof("=================a.Type:%v",a.Type)//2
		logger.Infof("=================a.Type:%T",a.Type)//
		logger.Infof("=================a.Version:%v",a.Version)//0
		logger.Infof("=================a.ChannelId:%v",a.ChannelId)//mychannel
		logger.Infof("=================a.Extension:%v",a.Extension)//[]
		logger.Infof("=================a.TxId",a.TxId)




		payloadSignatureHeader := &cb.SignatureHeader{}
		err = proto.Unmarshal(pa.Header.SignatureHeader,payloadSignatureHeader)
		creator := payloadSignatureHeader.Creator

		sid := &msp.SerializedIdentity{}
		err = proto.Unmarshal(creator, sid)
		/*
			请求消息的组织名id.Mspid，跟交易中的签名的组织名进行比较，如果一致就获取块
		*/
		//logger.Info("==========creator.OrgName",sid.Mspid)//Org1MSP

		//logger.Info("==============envelope.Payload===============",pa.Data)


		if a.Type == 5{
			request_singer = sid.Mspid
		}
		logger.Info("=============request_singer=================",request_singer)

		/*
			[10 2 26 0 18 2 26 0]
		*/
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
		//logger.Infof("=================eeee.TxId",eeee.TxId)


		//ppp := &cb.SignatureHeader{}
		//err = proto.Unmarshal(eee.Header.SignatureHeader,ppp)
		//creator1 := ppp.Creator
		//
		//sss:= &msp.SerializedIdentity{}
		//err = proto.Unmarshal(creator1, sss)
		//logger.Infof("==========creator.OrgName:%v===",sss.Mspid)//Org1MSP
	}













	//logger.Info("=============1.获取配置变量=======")
	//运行在order上的不能获取具体的用户，以下变量是在cli下的
	//oname:=os.Getenv("CORE_PEER_LOCALMSPID")/
	//logger.Info("=======2.CORE_PEER_LOCALMSPID===========",oname)
	//
	//oname1 := viper.GetString("CORE.peer.LOCALMSPID")
	//logger.Info("=======3.CORE_PEER_LOCALMSPID===========",oname1)
	//oname2 := viper.GetString("core.peer.localmspid")
	//logger.Info("=======5.CORE_PEER_LOCALMSPID===========",oname2)
	//if oname == "" && oname1=="" && oname2 == ""{
	//	oname = "Org3MSP"
	//}
	//logger.Info("=======6.CORE_PEER_LOCALMSPID===========",oname)
	addr := util.ExtractRemoteAddress(ctx)
	logger.Info("=======addr============",addr)
	//172.20.0.7:41574
	//172.19.0.7:51014
	payload, err := utils.UnmarshalPayload(envelope.Payload)
	if err != nil {
		logger.Infof("Received an envelope from %s with no payload: %s", addr, err)
		return cb.Status_BAD_REQUEST, nil
	}

	if payload.Header == nil {
		logger.Warningf("Malformed envelope received from %s with bad header", addr)
		return cb.Status_BAD_REQUEST, nil
	}

	chdr, err := utils.UnmarshalChannelHeader(payload.Header.ChannelHeader)
	if err != nil {
		logger.Warningf("Failed to unmarshal channel header from %s: %s", addr, err)
		return cb.Status_BAD_REQUEST, nil
	}



	err = h.validateChannelHeader(ctx, chdr)
	if err != nil {
		logger.Warningf("Rejecting deliver for %s due to envelope validation error: %s", addr, err)
		return cb.Status_BAD_REQUEST, nil
	}

	//logger.Info("======chdr.ChannelId=============",chdr.ChannelId)
	/*
	1.query
	mychannel
	 */
	chain := h.ChainManager.GetChain(chdr.ChannelId)
	logger.Infof("===============%v := h.ChainManager.GetChain(%v)====================",chain,chdr.ChannelId)
	if chain == nil {
		// Note, we log this at DEBUG because SDKs will poll waiting for channels to be created
		// So we would expect our log to be somewhat flooded with these
		logger.Debugf("Rejecting deliver for %s because channel %s not found", addr, chdr.ChannelId)
		return cb.Status_NOT_FOUND, nil
	}

	labels := []string{
		"channel", chdr.ChannelId,
		"filtered", strconv.FormatBool(isFiltered(srv)),
	}
	h.Metrics.RequestsReceived.With(labels...).Add(1)
	defer func() {
		labels := append(labels, "success", strconv.FormatBool(status == cb.Status_SUCCESS))
		h.Metrics.RequestsCompleted.With(labels...).Add(1)
	}()

	erroredChan := chain.Errored()
	select {
	case <-erroredChan:
		logger.Warningf("[channel: %s] Rejecting deliver request for %s because of consenter error", chdr.ChannelId, addr)
		return cb.Status_SERVICE_UNAVAILABLE, nil
	default:
	}

	accessControl, err := NewSessionAC(chain, envelope, srv.PolicyChecker, chdr.ChannelId, crypto.ExpiresAt)
	if err != nil {
		logger.Warningf("[channel: %s] failed to create access control object due to %s", chdr.ChannelId, err)
		return cb.Status_BAD_REQUEST, nil
	}

	if err := accessControl.Evaluate(); err != nil {
		logger.Warningf("[channel: %s] Client authorization revoked for deliver request from %s: %s", chdr.ChannelId, addr, err)
		return cb.Status_FORBIDDEN, nil
	}

	seekInfo := &ab.SeekInfo{}
	err = proto.Unmarshal(payload.Data, seekInfo)
	logger.Info("===============payload.Data========================",payload.Data)
	//createChannel
	//logger.Info("==================seekInfo==============")
	//logger.Info("==================seekInfo.Start==============",seekInfo.Stare)//<>
	//logger.Info("==================seekInfo.Stop==============",seekInfo.Stop)//<>
	//logger.Info("==================seekInfo.Behavior==============",seekInfo.Behavior)//BLOCK_UNTIL_READY
	if err != nil {
		logger.Info("[channel: %s] Received a signed deliver request from %s with malformed seekInfo payload: %s", chdr.ChannelId, addr, err)
		return cb.Status_BAD_REQUEST, nil
	}

	if seekInfo.Start == nil || seekInfo.Stop == nil {
		logger.Info("[channel: %s] Received seekInfo message from %s with missing start or stop %v, %v", chdr.ChannelId, addr, seekInfo.Start, seekInfo.Stop)
		// [channel: mychannel] Received seekInfo (0xc00058e240) start:<specified:<> > stop:<specified:<> >  from 172.19.0.7:51014
		return cb.Status_BAD_REQUEST, nil
	}

	logger.Infof("[channel: %s] Received seekInfo (%p) %v from %s", chdr.ChannelId, seekInfo, seekInfo, addr)

	logger.Info("==========chain.Reader().Iterator(seekInfo.Start)=======================")
	re := chain.Reader()
	if re == nil{
		return cb.Status_BAD_REQUEST, nil
	}

	logger.Info("=======1.获取该通道区块0=================")
	var s =&ab.SeekSpecified{Number: 0}
	a :=&ab.SeekPosition_Specified{Specified: s}

	dd := &ab.SeekPosition{Type: a}
	si := &ab.SeekInfo{Start: dd}

	c,_ := re.Iterator(si.Start)

	blo,st := c.Next()
	//logger.Info("====blo=======",blo)
	/*
	====blo======= header:<data_hash:"x\234qo\323\271l\026\215\\\231\245i\355\031\230 \010\007\013\2223D\026\037`\226\376\347u\\\264" > data:<data:"\n\350{\n\307\006\n\025\010\001\032\006\010\204\252\255\215\006\"\tmychannel\022\255\006\n\220\006\n\nOrdererMSP\022\201\006-----BEGIN CERTIFICATE-----\nMIICDDCCAbKgAwIBAgIQO7f5H/GNWl4bCFrav0m09TAKBggqhkjOPQQDAjBpMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEUMBIGA1UEChMLZXhhbXBsZS5jb20xFzAVBgNVBAMTDmNhLmV4YW1w\nbGUuY29tMB4XDTIxMTIwMjA3NTAwMFoXDTMxMTEzMDA3NTAwMFowWDELMAkGA1UE\nBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBGcmFuY2lz\nY28xHDAaBgNVBAMTE29yZGVyZXIuZXhhbXBsZS5jb20wWTATBgcqhkjOPQIBBggq\nhkjOPQMBBwNCAAQ50oX9JKkYpZg+CKMMHRBR9nWjdOLgD9yHfpiM01f+L64b1eEg\nBpwz0ci6KItxvX+j81Q4dRjV0/Tw/eSI7fllo00wSzAOBgNVHQ8BAf8EBAMCB4Aw\nDAYDVR0TAQH/BAIwADArBgNVHSMEJDAigCAiKUD447X2wODjavJYziGH+yguMn94\nPA285AcRrKlxzzAKBggqhkjOPQQDAgNIADBFAiEAvBTKhKJ1q9+Tto05f7a01B0V\n3hORQui4xrriz0G5Z1MCID6i4ksld0Nzhr+45XgZRcMxC+Q8SzEpbXWJDTTo+tIl\n-----END CERTIFICATE-----\n\022\030\244\270\341\260rg\361T\"\327\206\177\3066\264\234D\034\252s\241Eq\256\022\233u\n\372c\010\001\022\365c\022\351\027\n\007Orderer\022\335\027\022\214\025\n\nOrdererOrg\022\375\024\032\322\023\n\003MSP\022\312\023\022\277\023\022\274\023\n\nOrdererMSP\022\307\006-----BEGIN CERTIFICATE-----\nMIICPjCCAeSgAwIBAgIRAIjw5hgPtAkO5lYCXaEaMLUwCgYIKoZIzj0EAwIwaTEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xFDASBgNVBAoTC2V4YW1wbGUuY29tMRcwFQYDVQQDEw5jYS5leGFt\ncGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUwMDBaMGkxCzAJBgNV\nBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1TYW4gRnJhbmNp\nc2NvMRQwEgYDVQQKEwtleGFtcGxlLmNvbTEXMBUGA1UEAxMOY2EuZXhhbXBsZS5j\nb20wWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAARSnHvISLMxgSMdtZf+YbyVAY7s\nYbw4FSguR2FZcmIIwghcF5ILwpLhqL11TWhrCAFBGzECp6IkMZbkGphCi6plo20w\nazAOBgNVHQ8BAf8EBAMCAaYwHQYDVR0lBBYwFAYIKwYBBQUHAwIGCCsGAQUFBwMB\nMA8GA1UdEwEB/wQFMAMBAf8wKQYDVR0OBCIEICIpQPjjtfbA4ONq8ljOIYf7KC4y\nf3g8DbzkBxGsqXHPMAoGCCqGSM49BAMCA0gAMEUCIQCUHJm1/UtxamrMcj2YCa2S\nyddnoNtvnJXMVkJkrBdzvAIgKIMjLNuN1nNpILYI0K9OEMTW5Ty9oe5xZixGWNy5\nGRE=\n-----END CERTIFICATE-----\n\"\201\006-----BEGIN CERTIFICATE-----\nMIICCjCCAbGgAwIBAgIRAPGa4DAlJWzSR0M+g2HlvIgwCgYIKoZIzj0EAwIwaTEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xFDASBgNVBAoTC2V4YW1wbGUuY29tMRcwFQYDVQQDEw5jYS5leGFt\ncGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUwMDBaMFYxCzAJBgNV\nBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1TYW4gRnJhbmNp\nc2NvMRowGAYDVQQDDBFBZG1pbkBleGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqG\nSM49AwEHA0IABNttJVHFgo5jzOfKKr+CZ5swd0qD3SlzqJSyXnP9xxGMKvRDbtuT\nZ/iDvSRTXHLmnuCTaZlm/jywYHC+nzWiPcOjTTBLMA4GA1UdDwEB/wQEAwIHgDAM\nBgNVHRMBAf8EAjAAMCsGA1UdIwQkMCKAICIpQPjjtfbA4ONq8ljOIYf7KC4yf3g8\nDbzkBxGsqXHPMAoGCCqGSM49BAMCA0cAMEQCIGOptxqHKGK8Jx8/9e51v5uR7LrC\nddbj+fNM9+qkBmUlAiAvFmNDoA4h7kMKFdUrZd4k6jmHevxQtoN4ZSaSWW/1uw==\n-----END CERTIFICATE-----\nB\016\n\004SHA2\022\006SHA256J\317\006-----BEGIN CERTIFICATE-----\nMIICRDCCAeqgAwIBAgIRAOf9IIw7LUq0FAGT1hV84N0wCgYIKoZIzj0EAwIwbDEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xFDASBgNVBAoTC2V4YW1wbGUuY29tMRowGAYDVQQDExF0bHNjYS5l\neGFtcGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUwMDBaMGwxCzAJ\nBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1TYW4gRnJh\nbmNpc2NvMRQwEgYDVQQKEwtleGFtcGxlLmNvbTEaMBgGA1UEAxMRdGxzY2EuZXhh\nbXBsZS5jb20wWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAARZ8WTu9AoXBdvT7BYL\n2pvIVzsDR3vmsT5u89mjCtVdb88r5kupGJnOOGfZQis2pzrF4wJ0o18Yt9fdjzxZ\nICsVo20wazAOBgNVHQ8BAf8EBAMCAaYwHQYDVR0lBBYwFAYIKwYBBQUHAwIGCCsG\nAQUFBwMBMA8GA1UdEwEB/wQFMAMBAf8wKQYDVR0OBCIEIP+UJHfeNf6dBDc3gTWV\nReWye1cL41Spr1BkNX8GROuFMAoGCCqGSM49BAMCA0gAMEUCIQDSEr3H/yRfYj32\n0n3xOgwTnuhgiiP4C7w3LmWzF93xswIgIaagjKl5bCjUT2G21/PGvzKLH0+CQxln\nQ4i41pFddCc=\n-----END CERTIFICATE-----\n\032\006Admins\"4\n\006Admins\022*\022 \010\001\022\034\022\010\022\006\010\001\022\002\010\000\032\020\022\016\n\nOrdererMSP\020\001\032\006Admins\"3\n\007Readers\022(\022\036\010\001\022\032\022\010\022\006\010\001\022\002\010\000\032\016\022\014\n\nOrdererMSP\032\006Admins\"3\n\007Writers\022(\022\036\010\001\022\032\022\010\022\006\010\001\022\002\010\000\032\016\022\014\n\nOrdererMSP\032\006Admins*\006Admins\032\037\n\023ChannelRestrictions\022\010\032\006Admins\032$\n\014Capabilities\022\024\022\n\n\010\n\004V1_1\022\000\032\006Admins\032!\n\rConsensusType\022\020\022\006\n\004solo\032\006Admins\032\"\n\tBatchSize\022\025\022\013\010\n\020\200\200\3001\030\200\200 \032\006Admins\032\036\n\014BatchTimeout\022\016\022\004\n\0022s\032\006Admins\"\"\n\007Readers\022\027\022\r\010\003\022\t\n\007Readers\032\006Admins\"\"\n\007Writers\022\027\022\r\010\003\022\t\n\007Writers\032\006Admins\"\"\n\006Admins\022\030\022\016\010\003\022\n\n\006Admins\020\002\032\006Admins\"*\n\017BlockValidation\022\027\022\r\010\003\022\t\n\007Writers\032\006Admins*\006Admins\022\236I\n\013Application\022\216I\010\001\022\374#\n\007Org2MSP\022\360#\032\221\"\n\003MSP\022\211\"\022\376!\022\373!\n\007Org2MSP\022\343\006-----BEGIN CERTIFICATE-----\nMIICUjCCAfigAwIBAgIRAIfgoJC0Df7eMV4lqQemX0kwCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzIuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzIuZXhhbXBsZS5jb20wHhcNMjExMjAyMDc1MDAwWhcNMzExMTMwMDc1MDAw\nWjBzMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEZMBcGA1UEChMQb3JnMi5leGFtcGxlLmNvbTEcMBoGA1UE\nAxMTY2Eub3JnMi5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IA\nBLAZQE8DcMvRZPlkqySmSoitv7Lfz6pT8W7QA//PjKnk8nEqr/yECkTkFVUV2dF3\nMxqxgneiLngAdnyV8z9KnGujbTBrMA4GA1UdDwEB/wQEAwIBpjAdBgNVHSUEFjAU\nBggrBgEFBQcDAgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUwAwEB/zApBgNVHQ4EIgQg\nsbNOCtP+b8NXOrQNEZbQ8HSuvUvMMM6VPQZOVuAR4wswCgYIKoZIzj0EAwIDSAAw\nRQIhAOBO1HY1Wr0IHqnatm7PiR6SDpfyn5k86sIVUedxYJN3AiBlcomKBybUeYyV\neDgzidbY7VsDyn/GiFNGGmE+qyhX8g==\n-----END CERTIFICATE-----\n\"\252\006-----BEGIN CERTIFICATE-----\nMIICKjCCAdGgAwIBAgIRAJboIErw+IB46bNF6Xbw1FAwCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzIuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzIuZXhhbXBsZS5jb20wHhcNMjExMjAyMDc1MDAwWhcNMzExMTMwMDc1MDAw\nWjBsMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEPMA0GA1UECxMGY2xpZW50MR8wHQYDVQQDDBZBZG1pbkBv\ncmcyLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEzaHgcN4f\npClunjfoJiv9SlX144pk2pTRVkDVdOxnSewu09NSnr0er1f3Ro6VUuSxNtj8t9ne\neya8Bb9tvocGbaNNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYD\nVR0jBCQwIoAgsbNOCtP+b8NXOrQNEZbQ8HSuvUvMMM6VPQZOVuAR4wswCgYIKoZI\nzj0EAwIDRwAwRAIgaXs4/mABuf6WEAftlK9yjyZ+1mdvRkBHqni4jEy5lS4CID6W\nwD2GOZaiM82phTDNRW0HSjgj7rGMP3aWSG5I9AdC\n-----END CERTIFICATE-----\nB\016\n\004SHA2\022\006SHA256J\347\006-----BEGIN CERTIFICATE-----\nMIICVjCCAf2gAwIBAgIQUhxZJYzGqzE5nyITMC9zLjAKBggqhkjOPQQDAjB2MQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMi5leGFtcGxlLmNvbTEfMB0GA1UEAxMWdGxz\nY2Eub3JnMi5leGFtcGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUw\nMDBaMHYxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQH\nEw1TYW4gRnJhbmNpc2NvMRkwFwYDVQQKExBvcmcyLmV4YW1wbGUuY29tMR8wHQYD\nVQQDExZ0bHNjYS5vcmcyLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0D\nAQcDQgAEn8ukwF5H4XKARUcET+ZFGK/cPt6asGFzVN8fOvRFKW51mzLSZ9Q5e8FW\nZdl9lwBQ3l1bMlOLtrI5aYqJ3E3TvqNtMGswDgYDVR0PAQH/BAQDAgGmMB0GA1Ud\nJQQWMBQGCCsGAQUFBwMCBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1Ud\nDgQiBCCmeUdJn2MdTwQa/yo0kzdFNfThCql71zir++OQk6PgnDAKBggqhkjOPQQD\nAgNHADBEAiBqPLft3f8gyeIucgxwXXyFP96ynn80yEiD+dH8MK5REwIgMwm4qBgq\nOFVPRino2DG0+DW1Z4T4VoCCwAbYg39RqFI=\n-----END CERTIFICATE-----\nZ\342\r\010\001\022\356\006\n\343\006-----BEGIN CERTIFICATE-----\nMIICUjCCAfigAwIBAgIRAIfgoJC0Df7eMV4lqQemX0kwCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzIuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzIuZXhhbXBsZS5jb20wHhcNMjExMjAyMDc1MDAwWhcNMzExMTMwMDc1MDAw\nWjBzMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEZMBcGA1UEChMQb3JnMi5leGFtcGxlLmNvbTEcMBoGA1UE\nAxMTY2Eub3JnMi5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IA\nBLAZQE8DcMvRZPlkqySmSoitv7Lfz6pT8W7QA//PjKnk8nEqr/yECkTkFVUV2dF3\nMxqxgneiLngAdnyV8z9KnGujbTBrMA4GA1UdDwEB/wQEAwIBpjAdBgNVHSUEFjAU\nBggrBgEFBQcDAgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUwAwEB/zApBgNVHQ4EIgQg\nsbNOCtP+b8NXOrQNEZbQ8HSuvUvMMM6VPQZOVuAR4wswCgYIKoZIzj0EAwIDSAAw\nRQIhAOBO1HY1Wr0IHqnatm7PiR6SDpfyn5k86sIVUedxYJN3AiBlcomKBybUeYyV\neDgzidbY7VsDyn/GiFNGGmE+qyhX8g==\n-----END CERTIFICATE-----\n\022\006client\032\354\006\n\343\006-----BEGIN CERTIFICATE-----\nMIICUjCCAfigAwIBAgIRAIfgoJC0Df7eMV4lqQemX0kwCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzIuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzIuZXhhbXBsZS5jb20wHhcNMjExMjAyMDc1MDAwWhcNMzExMTMwMDc1MDAw\nWjBzMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEZMBcGA1UEChMQb3JnMi5leGFtcGxlLmNvbTEcMBoGA1UE\nAxMTY2Eub3JnMi5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IA\nBLAZQE8DcMvRZPlkqySmSoitv7Lfz6pT8W7QA//PjKnk8nEqr/yECkTkFVUV2dF3\nMxqxgneiLngAdnyV8z9KnGujbTBrMA4GA1UdDwEB/wQEAwIBpjAdBgNVHSUEFjAU\nBggrBgEFBQcDAgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUwAwEB/zApBgNVHQ4EIgQg\nsbNOCtP+b8NXOrQNEZbQ8HSuvUvMMM6VPQZOVuAR4wswCgYIKoZIzj0EAwIDSAAw\nRQIhAOBO1HY1Wr0IHqnatm7PiR6SDpfyn5k86sIVUedxYJN3AiBlcomKBybUeYyV\neDgzidbY7VsDyn/GiFNGGmE+qyhX8g==\n-----END CERTIFICATE-----\n\022\004peer\032\006Admins\"X\n\007Readers\022M\022C\010\001\022?\022\020\022\016\010\001\022\002\010\000\022\002\010\001\022\002\010\002\032\r\022\013\n\007Org2MSP\020\001\032\r\022\013\n\007Org2MSP\020\003\032\r\022\013\n\007Org2MSP\020\002\032\006Admins\"E\n\007Writers\022:\0220\010\001\022,\022\014\022\n\010\001\022\002\010\000\022\002\010\001\032\r\022\013\n\007Org2MSP\020\001\032\r\022\013\n\007Org2MSP\020\002\032\006Admins\"1\n\006Admins\022'\022\035\010\001\022\031\022\010\022\006\010\001\022\002\010\000\032\r\022\013\n\007Org2MSP\020\001\032\006Admins*\006Admins\022\360#\n\007Org1MSP\022\344#\032\205\"\n\003MSP\022\375!\022\362!\022\357!\n\007Org1MSP\022\337\006-----BEGIN CERTIFICATE-----\nMIICUTCCAfegAwIBAgIQTDVrkQN1dC1Xkq0gNdWmNTAKBggqhkjOPQQDAjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEcMBoGA1UEAxMTY2Eu\nb3JnMS5leGFtcGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUwMDBa\nMHMxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMRkwFwYDVQQKExBvcmcxLmV4YW1wbGUuY29tMRwwGgYDVQQD\nExNjYS5vcmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE\nhU1Xj6u7o8o3vTpBMx0XgaKCHYIfwWW4joguVanxY7l2FoRsdeMlbSLpIbGOVeq2\nrqy5H1C1m+d1lvV5eUjlZaNtMGswDgYDVR0PAQH/BAQDAgGmMB0GA1UdJQQWMBQG\nCCsGAQUFBwMCBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdDgQiBCBc\npldgEIiaAL2L5u/dXT77TRUFw4c4xvgl33Q0ZMUW5jAKBggqhkjOPQQDAgNIADBF\nAiEA1moRbtOf7FmKUBtnlbWOeKW1+Ou1gECRbRmk28u82qYCIHCl4liG91dRkZJV\ngHFQhMyFuyn1TBkuY3VdWoTMfntJ\n-----END CERTIFICATE-----\n\"\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAdCgAwIBAgIQdoMtXcRmr8XdkhTXGAwFzDAKBggqhkjOPQQDAjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEcMBoGA1UEAxMTY2Eu\nb3JnMS5leGFtcGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUwMDBa\nMGwxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMQ8wDQYDVQQLEwZjbGllbnQxHzAdBgNVBAMMFkFkbWluQG9y\nZzEuZXhhbXBsZS5jb20wWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAR4lTOcEwH/\n7BA0PJHXxXCQWTvUHqLrw/m1noiFjGZa7wQWZ1NQ+AbOJ8tydWS3xu5YVw3KegfZ\naXcQ2VMAAYsIo00wSzAOBgNVHQ8BAf8EBAMCB4AwDAYDVR0TAQH/BAIwADArBgNV\nHSMEJDAigCBcpldgEIiaAL2L5u/dXT77TRUFw4c4xvgl33Q0ZMUW5jAKBggqhkjO\nPQQDAgNHADBEAiBKZAEOoM00vmTIR0pP7XFBp41hvVL4PGwISqi0v4AwTgIgUB5T\n6gaVVogvaUeMmCqljNGNupyBj1iY0jxDlvGteNA=\n-----END CERTIFICATE-----\nB\016\n\004SHA2\022\006SHA256J\347\006-----BEGIN CERTIFICATE-----\nMIICVzCCAf6gAwIBAgIRAIrP4xsYBvSOZyzAtxl2gV4wCgYIKoZIzj0EAwIwdjEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHzAdBgNVBAMTFnRs\nc2NhLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMjAyMDc1MDAwWhcNMzExMTMwMDc1\nMDAwWjB2MQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UE\nBxMNU2FuIEZyYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEfMB0G\nA1UEAxMWdGxzY2Eub3JnMS5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49\nAwEHA0IABCHAG6UlLVndKC68Ot/m6FZUP5gki9Kt630bQcJvQKlLm9aT1n+h1Zkq\ntE+cxCs7cnkD9k1Vmroyi4zcnVvD4J6jbTBrMA4GA1UdDwEB/wQEAwIBpjAdBgNV\nHSUEFjAUBggrBgEFBQcDAgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUwAwEB/zApBgNV\nHQ4EIgQgDrK2cbW+BdW3uqYuSM8QYsCGsvyyFbskdXhzTBohIy4wCgYIKoZIzj0E\nAwIDRwAwRAIgVTFpaBTqyMxVfDACj3OIjwIS6fUZBDyl5w9x1iuD/JwCIHQ7/Azh\ncmuA1FJLOSMjVFCms/hPgatGAENmKoNhyeUV\n-----END CERTIFICATE-----\nZ\332\r\010\001\022\352\006\n\337\006-----BEGIN CERTIFICATE-----\nMIICUTCCAfegAwIBAgIQTDVrkQN1dC1Xkq0gNdWmNTAKBggqhkjOPQQDAjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEcMBoGA1UEAxMTY2Eu\nb3JnMS5leGFtcGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUwMDBa\nMHMxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMRkwFwYDVQQKExBvcmcxLmV4YW1wbGUuY29tMRwwGgYDVQQD\nExNjYS5vcmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE\nhU1Xj6u7o8o3vTpBMx0XgaKCHYIfwWW4joguVanxY7l2FoRsdeMlbSLpIbGOVeq2\nrqy5H1C1m+d1lvV5eUjlZaNtMGswDgYDVR0PAQH/BAQDAgGmMB0GA1UdJQQWMBQG\nCCsGAQUFBwMCBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdDgQiBCBc\npldgEIiaAL2L5u/dXT77TRUFw4c4xvgl33Q0ZMUW5jAKBggqhkjOPQQDAgNIADBF\nAiEA1moRbtOf7FmKUBtnlbWOeKW1+Ou1gECRbRmk28u82qYCIHCl4liG91dRkZJV\ngHFQhMyFuyn1TBkuY3VdWoTMfntJ\n-----END CERTIFICATE-----\n\022\006client\032\350\006\n\337\006-----BEGIN CERTIFICATE-----\nMIICUTCCAfegAwIBAgIQTDVrkQN1dC1Xkq0gNdWmNTAKBggqhkjOPQQDAjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEcMBoGA1UEAxMTY2Eu\nb3JnMS5leGFtcGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUwMDBa\nMHMxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMRkwFwYDVQQKExBvcmcxLmV4YW1wbGUuY29tMRwwGgYDVQQD\nExNjYS5vcmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE\nhU1Xj6u7o8o3vTpBMx0XgaKCHYIfwWW4joguVanxY7l2FoRsdeMlbSLpIbGOVeq2\nrqy5H1C1m+d1lvV5eUjlZaNtMGswDgYDVR0PAQH/BAQDAgGmMB0GA1UdJQQWMBQG\nCCsGAQUFBwMCBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdDgQiBCBc\npldgEIiaAL2L5u/dXT77TRUFw4c4xvgl33Q0ZMUW5jAKBggqhkjOPQQDAgNIADBF\nAiEA1moRbtOf7FmKUBtnlbWOeKW1+Ou1gECRbRmk28u82qYCIHCl4liG91dRkZJV\ngHFQhMyFuyn1TBkuY3VdWoTMfntJ\n-----END CERTIFICATE-----\n\022\004peer\032\006Admins\"1\n\006Admins\022'\022\035\010\001\022\031\022\010\022\006\010\001\022\002\010\000\032\r\022\013\n\007Org1MSP\020\001\032\006Admins\"X\n\007Readers\022M\022C\010\001\022?\022\020\022\016\010\001\022\002\010\000\022\002\010\001\022\002\010\002\032\r\022\013\n\007Org1MSP\020\001\032\r\022\013\n\007Org1MSP\020\003\032\r\022\013\n\007Org1MSP\020\002\032\006Admins\"E\n\007Writers\022:\0220\010\001\022,\022\014\022\n\010\001\022\002\010\000\022\002\010\001\032\r\022\013\n\007Org1MSP\020\001\032\r\022\013\n\007Org1MSP\020\002\032\006Admins*\006Admins\032$\n\014Capabilities\022\024\022\n\n\010\n\004V1_3\022\000\032\006Admins\"\"\n\006Admins\022\030\022\016\010\003\022\n\n\006Admins\020\002\032\006Admins\"\"\n\007Readers\022\027\022\r\010\003\022\t\n\007Readers\032\006Admins\"\"\n\007Writers\022\027\022\r\010\003\022\t\n\007Writers\032\006Admins*\006Admins\032$\n\014Capabilities\022\024\022\n\n\010\n\004V1_3\022\000\032\006Admins\032&\n\020HashingAlgorithm\022\022\022\010\n\006SHA256\032\006Admins\032*\n\nConsortium\022\034\022\022\n\020SampleConsortium\032\006Admins\032-\n\031BlockDataHashingStructure\022\020\022\006\010\377\377\377\377\017\032\006Admins\032I\n\020OrdererAddresses\0225\022\032\n\030orderer.example.com:7050\032\027/Channel/Orderer/Admins\"\"\n\006Admins\022\030\022\016\010\003\022\n\n\006Admins\020\002\032\006Admins\"\"\n\007Readers\022\027\022\r\010\003\022\t\n\007Readers\032\006Admins\"\"\n\007Writers\022\027\022\r\010\003\022\t\n\007Writers\032\006Admins*\006Admins\022\233\021\n\320\020\n\355\006\n\025\010\002\032\006\010\204\252\255\215\006\"\tmychannel\022\323\006\n\266\006\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAdCgAwIBAgIQdoMtXcRmr8XdkhTXGAwFzDAKBggqhkjOPQQDAjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEcMBoGA1UEAxMTY2Eu\nb3JnMS5leGFtcGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUwMDBa\nMGwxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMQ8wDQYDVQQLEwZjbGllbnQxHzAdBgNVBAMMFkFkbWluQG9y\nZzEuZXhhbXBsZS5jb20wWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAR4lTOcEwH/\n7BA0PJHXxXCQWTvUHqLrw/m1noiFjGZa7wQWZ1NQ+AbOJ8tydWS3xu5YVw3KegfZ\naXcQ2VMAAYsIo00wSzAOBgNVHQ8BAf8EBAMCB4AwDAYDVR0TAQH/BAIwADArBgNV\nHSMEJDAigCBcpldgEIiaAL2L5u/dXT77TRUFw4c4xvgl33Q0ZMUW5jAKBggqhkjO\nPQQDAgNHADBEAiBKZAEOoM00vmTIR0pP7XFBp41hvVL4PGwISqi0v4AwTgIgUB5T\n6gaVVogvaUeMmCqljNGNupyBj1iY0jxDlvGteNA=\n-----END CERTIFICATE-----\n\022\030F\202\353\213h0\304\307\312d\240\320\304\236\034!\227\321\263\245\263&\374p\022\335\t\n\270\002\n\tmychannel\022;\022)\n\013Application\022\032\022\013\n\007Org2MSP\022\000\022\013\n\007Org1MSP\022\000\032\016\n\nConsortium\022\000\032\355\001\022\306\001\n\013Application\022\266\001\010\001\022\013\n\007Org2MSP\022\000\022\013\n\007Org1MSP\022\000\032$\n\014Capabilities\022\024\022\n\n\010\n\004V1_3\022\000\032\006Admins\"\"\n\006Admins\022\030\022\016\010\003\022\n\n\006Admins\020\002\032\006Admins\"\"\n\007Readers\022\027\022\r\010\003\022\t\n\007Readers\032\006Admins\"\"\n\007Writers\022\027\022\r\010\003\022\t\n\007Writers\032\006Admins*\006Admins\032\"\n\nConsortium\022\024\022\022\n\020SampleConsortium\022\237\007\n\323\006\n\266\006\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAdCgAwIBAgIQdoMtXcRmr8XdkhTXGAwFzDAKBggqhkjOPQQDAjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEcMBoGA1UEAxMTY2Eu\nb3JnMS5leGFtcGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUwMDBa\nMGwxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMQ8wDQYDVQQLEwZjbGllbnQxHzAdBgNVBAMMFkFkbWluQG9y\nZzEuZXhhbXBsZS5jb20wWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAR4lTOcEwH/\n7BA0PJHXxXCQWTvUHqLrw/m1noiFjGZa7wQWZ1NQ+AbOJ8tydWS3xu5YVw3KegfZ\naXcQ2VMAAYsIo00wSzAOBgNVHQ8BAf8EBAMCB4AwDAYDVR0TAQH/BAIwADArBgNV\nHSMEJDAigCBcpldgEIiaAL2L5u/dXT77TRUFw4c4xvgl33Q0ZMUW5jAKBggqhkjO\nPQQDAgNHADBEAiBKZAEOoM00vmTIR0pP7XFBp41hvVL4PGwISqi0v4AwTgIgUB5T\n6gaVVogvaUeMmCqljNGNupyBj1iY0jxDlvGteNA=\n-----END CERTIFICATE-----\n\022\030\210i\2341\177\235\023\021_\366\001\231\356\3764\224\"\010\257Dy\344\206\275\022G0E\002!\000\341\337X\250S\026\021W\001^&J%\302\016\342\"yuX@\361\200r\216F\23518\253\254\237\002 b\210\346\320\026{\021\334\321\373M\315\237\320JN\030<\227Y\004\307M\005l3\3012\\\024_`\022F0D\002 v*\260k\213\360r\355\313\256\355s\000/&\347\210G`\311B\252\325\347\363\327\004j\216\216\026\350\002 J\2514Ee\206M\236\014o\3664Q\010|Bl&\325'Pl\347\024\360\332S:\021@\003c\022G0E\002!\000\313dx\010\217Qh:Z\353\026\212?\315w*g\333}\262\323+|\221\037Tg\323\260\274\201\277\002 +!\022\232\265k\221T{l\207|g\232\272\373M\361\355\371Ok:\366'W\2769\325z@\302" > metadata:<metadata:"" metadata:"" metadata:"" metadata:"" >

	*/
	logger.Info("========st",st)//SUCCESS
	if st != cb.Status_SUCCESS{
		return cb.Status_BAD_REQUEST, nil
	}
	//
	logger.Info("=======2.获取配置文件的交易信息，提取envelop=================")
	e,err := utils.ExtractEnvelope(blo,0)
	if err != nil{
		return cb.Status_BAD_REQUEST, nil
	}
	p,err := utils.ExtractPayload(e)
	if err != nil{
		return
	}



	channelHearder, err := utils.UnmarshalChannelHeader(p.Header.ChannelHeader)
	if err != nil {
		return cb.Status_BAD_REQUEST, nil
	}

	logger.Info("===============channelHearder=================",*channelHearder)
	orgName := channelHearder.OrgName
	logger.Info("==========orgName==============",orgName)



	cursor, number := re.Iterator(seekInfo.Start)
	logger.Infof("==========cursor, %v := chain.Reader().Iterator(%v)=======================",number,seekInfo.Start)
	/*
	==========cursor, 0 := chain.Reader().Iterator(specified:<> )
	 */
	defer cursor.Close()
	var stopNum uint64
	switch stop := seekInfo.Stop.Type.(type) {
	case *ab.SeekPosition_Oldest:
		logger.Info("=====case *ab.SeekPosition_Oldest:=========")
		stopNum = number
		logger.Info("========stopNum=========",stopNum)
	case *ab.SeekPosition_Newest:
		logger.Info("=====case *ab.SeekPosition_Newest:=========")
		stopNum = chain.Reader().Height() - 1
		logger.Info("========stopNum=========",stopNum)
	case *ab.SeekPosition_Specified:
		logger.Info("====case *ab.SeekPosition_Specified:=========")
		logger.Info("===========stop.Specified.Number============",stop.Specified.Number)//0
		/*
		create channel
		 */
		stopNum = stop.Specified.Number
		if stopNum < number {
			logger.Warningf("[channel: %s] Received invalid seekInfo message from %s: start number %d greater than stop number %d", chdr.ChannelId, addr, number, stopNum)
			return cb.Status_BAD_REQUEST, nil
		}
	}

	for {
		if seekInfo.Behavior == ab.SeekInfo_FAIL_IF_NOT_READY {
			//logger.Info("=====number====",number)
			//logger.Info("=====chain.Reader().Height()-1 ====",chain.Reader().Height()-1 )
			if number > chain.Reader().Height()-1 {
				return cb.Status_NOT_FOUND, nil
			}
		}

		var block *cb.Block
		var status cb.Status

		logger.Info("====================")
		iterCh := make(chan struct{})
		go func() {
			logger.Info("====1.获取区块====")
			block, status = cursor.Next()
			logger.Info("====2.获取区块====")
			close(iterCh)
		}()

		select {
		case <-ctx.Done():
			logger.Debugf("Context canceled, aborting wait for next block")
			return cb.Status_INTERNAL_SERVER_ERROR, errors.Wrapf(ctx.Err(), "context finished before block retrieved")
		case <-erroredChan:
			logger.Warningf("Aborting deliver for request because of background error")
			return cb.Status_SERVICE_UNAVAILABLE, nil
		case <-iterCh:
			// Iterator has set the block and status vars
		}

		if status != cb.Status_SUCCESS {
			logger.Errorf("[channel: %s] Error reading from channel, cause was: %v", chdr.ChannelId, status)
			return status, nil
		}

		// increment block number to support FAIL_IF_NOT_READY deliver behavior
		number++

		if err := accessControl.Evaluate(); err != nil {
			logger.Warningf("[channel: %s] Client authorization revoked for deliver request from %s: %s", chdr.ChannelId, addr, err)
			return cb.Status_FORBIDDEN, nil
		}

		logger.Debugf("[channel: %s] Delivering block for (%p) for %s", chdr.ChannelId, seekInfo, addr)

		logger.Info("============err := srv.SendBlockResponse(block)=========================")
		//logger.Info("===========1.block.Data.Data====================",block.Data.Data)
		if block != nil{
			if cb.PolicyOrgName[chdr.ChannelId] != ""{
				logger.Info("=============5.cb.PolicyOrgName[chdr.ChannelId] != \"\"==================")
				var bd cb.BlockData
				//遵循规则
				for index,envelopBytes := range block.Data.Data{
					en,err := utils.ExtractEnvelope(block,index)
					if err != nil{
						return cb.Status_BAD_REQUEST, nil
					}
					pa,err := utils.ExtractPayload(en)
					if err != nil{
						return cb.Status_FORBIDDEN, nil
					}

					a,err := utils.UnmarshalChannelHeader(pa.Header.ChannelHeader)
					logger.Info("=================channelHeader",*a)

					logger.Info("===========主节点名===========",a.OrgName)
					txid_signed = a.OrgPki
					logger.Info("===========交易发送者============",txid_signed)
					/*
					{1 0 seconds:1638665094  mychannel  0 [] [] Org1MSP 1234 {} [] 0}
					 */

					payloadSignatureHeader := &cb.SignatureHeader{}
					err = proto.Unmarshal(pa.Header.SignatureHeader,payloadSignatureHeader)
					creator := payloadSignatureHeader.Creator

					sid := &msp.SerializedIdentity{}
					err = proto.Unmarshal(creator, sid)
					logger.Info("==========creator.OrgName",sid.Mspid)

					txid_order= sid.Mspid
					logger.Info("==============交易打包者=========================",txid_order)
					//OrdererMSP



                   //ee := pa.Data utils.EnvelopeToConfigUpdate()

                   logger.Info("==========request_singer==========",request_singer)
                   logger.Info("==========orgName==========",orgName)
                   logger.Info("==========txid_signed==========",txid_signed)

					if request_singer != orgName{
						if  request_singer!= txid_signed{
							logger.Infof("===========3.%v区块交易没有获取权限==================",request_singer)
							continue
						}
					}
					bd.Data = append(bd.Data,envelopBytes)
				}
				//为了保持块的高度一致，交易为空，但是区块头需要
				//logger.Info("=========2.block.Data.Data=============",block.Data.Data)
				//logger.Info("=========bd.Data=============",bd.Data)
				copy(block.Data.Data,bd.Data)
			}
		}
		logger.Info("===========3.block.Data.Data====================",block.Data.Data)
		 err := srv.SendBlockResponse(block)
		 if err != nil{
			logger.Warningf("[channel: %s] Error sending to %s: %s", chdr.ChannelId, addr, err)
			return cb.Status_INTERNAL_SERVER_ERROR, err
		}

		h.Metrics.BlocksSent.With(labels...).Add(1)

		 logger.Info("=======stopNum=========",stopNum)//0
		 logger.Info("=======block.Header.Number=========",block.Header.Number)//0

		if stopNum == block.Header.Number {
			break
		}
	}

	logger.Infof("[channel: %s] Done delivering to %s for (%p)", chdr.ChannelId, addr, seekInfo)
	/*
	 [channel: mychannel] Done delivering to 172.21.0.7:37216 for (0xc000b92340)
	 */

	return cb.Status_SUCCESS, nil
}

func (h *Handler) validateChannelHeader(ctx context.Context, chdr *cb.ChannelHeader) error {
	logger.Info("==Handler==validateChannelHeader==")
	if chdr.GetTimestamp() == nil {
		err := errors.New("channel header in envelope must contain timestamp")
		return err
	}

	envTime := time.Unix(chdr.GetTimestamp().Seconds, int64(chdr.GetTimestamp().Nanos)).UTC()
	serverTime := time.Now()

	if math.Abs(float64(serverTime.UnixNano()-envTime.UnixNano())) > float64(h.TimeWindow.Nanoseconds()) {
		err := errors.Errorf("envelope timestamp %s is more than %s apart from current server time %s", envTime, h.TimeWindow, serverTime)
		return err
	}

	err := h.BindingInspector.Inspect(ctx, chdr)
	if err != nil {
		return err
	}

	return nil
}
