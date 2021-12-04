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
	"os"
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
		logger.Info("=========envelope===================",*envelope)
		p,_:= utils.ExtractPayload(envelope)
		logger.Info("==============envelope.Payload===============",p.Data)
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
	logger.Info("=============1.获取配置变量=======")
	oname:=os.Getenv("CORE_PEER_LOCALMSPID")
	logger.Info("=======2.CORE_PEER_LOCALMSPID===========",oname)
	if oname == ""{
		oname = "Org3MSP"
	}
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

	logger.Info("=======1。获取该通道区块0=================")
	a :=&ab.SeekPosition_Specified{}
	a.Specified.Number = uint64(0)
	si := &ab.SeekInfo{}
	si.Start.Type = a
	c,_ := re.Iterator(si.Start)

	b, s := c.Next()
	logger.Info("========s",s)
	if s != cb.Status_SUCCESS{
		return cb.Status_BAD_REQUEST, nil
	}

	logger.Info("=======2.获取配置文件的交易信息，提取envelop=================")
	e,err := utils.ExtractEnvelope(b,0)
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
		logger.Info("===========1.block.Data.Data====================",block.Data.Data)
		if block != nil{
			if orgName != ""{
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

					payloadSignatureHeader := &cb.SignatureHeader{}
					err = proto.Unmarshal(pa.Header.SignatureHeader,payloadSignatureHeader)
					creator := payloadSignatureHeader.Creator

					sid := &msp.SerializedIdentity{}
					err = proto.Unmarshal(creator, sid)
					logger.Info("==========creator.OrgName",sid.Mspid)
					if orgName != oname {

					}
					logger.Info("=======oname=======",oname)
					logger.Info("=======sid.Mspid=======",sid.Mspid)
					if oname != sid.Mspid{
						if oname != orgName{
							logger.Info("===========3.区块交易没有获取权限==================")
							continue
						}
					}
					bd.Data = append(bd.Data,envelopBytes)
				}
				//为了保持块的高度一致，交易为空，但是区块头需要
				logger.Info("=========2.block.Data.Data=============",block.Data.Data)
				logger.Info("=========bd.Data=============",bd.Data)
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
