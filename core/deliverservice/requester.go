package deliverclient

import (
	"math"

	"github.com/hyperledger/fabric/common/localmsp"
	"github.com/hyperledger/fabric/core/deliverservice/blocksprovider"
	"github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/orderer"
	"github.com/hyperledger/fabric/protos/utils"
)
/*
   Deliver 服务是建议在grpc之上的，且orderer为Deliver的服务端，则可以推断是peer节点向orderer索要block数据。
   gossip服务的索要对象是core/deliverservice/requrester.go中的blocksRequester，索要行为发生在Requestblocks函数中
 */
type blocksRequester struct {
	chainID string
	client  blocksprovider.BlocksDeliverer
}
/*
索要orderer的block数据，函数传入一个包裹了高度值的LedgerInfo,若该高度值为向orderer索要的block序列号，且高度值大于0，

 */
func (b *blocksRequester) RequestBlocks(ledgerInfoProvider blocksprovider.LedgerInfo) error {
	//当前账本高度
	height, err := ledgerInfoProvider.LedgerHeight()
	if err != nil {
		logger.Errorf("Can't get legder height for channel %s from committer [%s]", b.chainID, err)
		return err
	}
    /*
    seekLatestFromCommitter和seekOldest函数都是先创建一个包装了seekInfo信息（即索要的block的起止范围信息）的HeaderType_
    CONFIG_UPDATE类型的Envelope信息然后b.client.send(env)向orerer端的Deliver服务端索要block。注意，这两个函数索要block范围的起点可能不一样
    ，但是止点都是math.MaxUint64,即最大极限值，这相当于向orderer端索要现在以及将来所产生的所有block
     */
	if height > 0 {
		logger.Debugf("Starting deliver with block [%d] for channel %s", height, b.chainID)
		//调用
		if err := b.seekLatestFromCommitter(height); err != nil {
			return err
		}
	} else {
		logger.Debugf("Starting deliver with olders block for channel %s", b.chainID)
		//调用
		if err := b.seekOldest(); err != nil {
			return err
		}
	}

	return nil
}

func (b *blocksRequester) seekOldest() error {
	seekInfo := &orderer.SeekInfo{
		Start:    &orderer.SeekPosition{Type: &orderer.SeekPosition_Oldest{Oldest: &orderer.SeekOldest{}}},
		Stop:     &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: math.MaxUint64}}},
		Behavior: orderer.SeekInfo_BLOCK_UNTIL_READY,
	}

	//TODO- epoch and msgVersion may need to be obtained for nowfollowing usage in orderer/configupdate/configupdate.go
	msgVersion := int32(0)
	epoch := uint64(0)
	env, err := utils.CreateSignedEnvelope(common.HeaderType_CONFIG_UPDATE, b.chainID, localmsp.NewSigner(), seekInfo, msgVersion, epoch)
	if err != nil {
		return err
	}
	return b.client.Send(env)
}

func (b *blocksRequester) seekLatestFromCommitter(height uint64) error {
	seekInfo := &orderer.SeekInfo{
		Start:    &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: height}}},
		Stop:     &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: math.MaxUint64}}},
		Behavior: orderer.SeekInfo_BLOCK_UNTIL_READY,
	}

	//TODO- epoch and msgVersion may need to be obtained for nowfollowing usage in orderer/configupdate/configupdate.go
	msgVersion := int32(0)
	epoch := uint64(0)
	env, err := utils.CreateSignedEnvelope(common.HeaderType_CONFIG_UPDATE, b.chainID, localmsp.NewSigner(), seekInfo, msgVersion, epoch)
	if err != nil {
		return err
	}
	return b.client.Send(env)
}
