package blockcutter

import (
	"github.com/hyperledger/fabric/common/config"
	"github.com/hyperledger/fabric/orderer/common/filter"
	cb "github.com/hyperledger/fabric/protos/common"

	"github.com/op/go-logging"
	)


var logger = logging.MustGetLogger("orderer/common/blockcutter")
/*
块分割工具，用于分割block，具体为定义的Receiver，类似与工厂流水线的自动打包机，一条条的消息在流水线上被传到cutter处，把
一条条消息打包成一批（一箱）消息，同时返回整理好的这批消息对应的committer集合，至此cutter的任务完成。每一批消息被当做一个block
执行完对应的committer之后被写入账本。在configtx。yaml中关于Orderer的配置项中，BatchSize部分规定了block的大小：MaxMeessageCout制定饿了block中最多
能够存储的消息总数，AbsoluteMaxBytes指定了block中最大的字节数（block cutter处理消息的过程中，会努力使每一批消息计量保持在这个值上）。根据这三个值，cutter在工作的
时候（具体指blockcutter。go中的Orderer函数）1。若一个Envelope的数据大小（Payload+签名）大于PreferredMaxBytes，不论当前缓存如何，立即Cut。2）若一个Envelop被要求
单纯存储在一个block里面（即该消息对应的committer的Isolated()返回true），要立即cut。3）若一个Envelope的大小加上blockcutter已有消息的大小之和大于 MaxMessageCount
要立即cut，5）还有一个中比较特殊的Cut，有confitx.yaml配置项，batchtimeout控制，当时间超过此值的时候，chain启动的处理消息的流程中主动触发的Cut。cut所做的工作，就是将
当前缓存的的消息，和committer返回供blockcutter与当前处理的Envelope打包成一批或者两批消息，然后清空缓存信息。在在上述需要cut的情况中，只有1）2）会产生两批消息，且先是旧消息
（即blockcutter中之前缓存的消息）为一批，receiver中，filters是一个RuleSet,定义了过滤条件集合，均来自与orderer/multiChain/chainsupport.go中的
createStandardFilters或createSystemChainFilters（后者只是比前者多了一个systemChainFilter过滤对象，4）所示）；pendingBatch是一个Envelope组合，用来缓存Envelope消息
pendingBatchSizeBytes用来记录这些缓存消息的大小，pendingCommitters是一个Committer数组，用来缓存每个Envelope对应的committer。

 */

// Receiver defines a sink for the ordered broadcast messages
type Receiver interface {
	// Ordered should be invoked sequentially as messages are ordered
	// If the current message valid, and no batches need to be cut:
	//   - Ordered will return nil, nil, and true (indicating ok).
	// If the current message valid, and batches need to be cut:
	//   - Ordered will return 1 or 2 batches of messages, 1 or 2 batches of committers, and true (indicating ok).
	// If the current message is invalid:
	//   - Ordered will return nil, nil, and false (to indicate not ok).
	//
	// Given a valid message, if the current message needs to be isolated (as determined during filtering).
	//   - Ordered will return:
	//     * The pending batch of (if not empty), and a second batch containing only the isolated message.
	//     * The corresponding batches of committers.
	//     * true (indicating ok).
	// Otherwise, given a valid message, the pending batch, if not empty, will be cut and returned if:
	//   - The current message needs to be isolated (as determined during filtering).
	//   - The current message will cause the pending batch size in bytes to exceed BatchSize.PreferredMaxBytes.
	//   - After adding the current message to the pending batch, the message count has reached BatchSize.MaxMessageCount.
	Ordered(msg *cb.Envelope) ([][]*cb.Envelope, [][]filter.Committer, bool)

	// Cut returns the current batch and starts a new one
	Cut() ([]*cb.Envelope, []filter.Committer)
}

type receiver struct {
	sharedConfigManager   config.Orderer
	filters               *filter.RuleSet//RuleSet定义了过滤条件集合
	pendingBatch          []*cb.Envelope//是一个Envelope数组，用来缓存Envelope消息；
	pendingBatchSizeBytes uint32 //用来记录这些缓存的消息大小，
	pendingCommitters     []filter.Committer//pendingCommitters是一个Committer数组，用来缓存每个Envelope对应的committer
}

// NewReceiverImpl creates a Receiver implementation based on the given configtxorderer manager and filters
func NewReceiverImpl(sharedConfigManager config.Orderer, filters *filter.RuleSet) Receiver {
	return &receiver{
		sharedConfigManager: sharedConfigManager,
		filters:             filters,
	}
}

// Ordered should be invoked sequentially as messages are ordered
// If the current message valid, and no batches need to be cut:
//   - Ordered will return nil, nil, and true (indicating ok).
// If the current message valid, and batches need to be cut:
//   - Ordered will return 1 or 2 batches of messages, 1 or 2 batches of committers, and true (indicating ok).
// If the current message is invalid:
//   - Ordered will return nil, nil, and false (to indicate not ok).
//
// Given a valid message, if the current message needs to be isolated (as determined during filtering).
//   - Ordered will return:
//     * The pending batch of (if not empty), and a second batch containing only the isolated message.
//     * The corresponding batches of committers.
//     * true (indicating ok).
// Otherwise, given a valid message, the pending batch, if not empty, will be cut and returned if:
//   - The current message needs to be isolated (as determined during filtering).
//   - The current message will cause the pending batch size in bytes to exceed BatchSize.PreferredMaxBytes.
//   - After adding the current message to the pending batch, the message count has reached BatchSize.MaxMessageCount.
func (r *receiver) Ordered(msg *cb.Envelope) ([][]*cb.Envelope, [][]filter.Committer, bool) {
	// The messages must be filtered a second time in case configuration has changed since the message was received
	committer, err := r.filters.Apply(msg)
	if err != nil {
		logger.Debugf("Rejecting message: %s", err)
		return nil, nil, false
	}

	messageSizeBytes := messageSizeBytes(msg)

	if committer.Isolated() || messageSizeBytes > r.sharedConfigManager.BatchSize().PreferredMaxBytes {

		if committer.Isolated() {
			logger.Debugf("Found message which requested to be isolated, cutting into its own batch")
		} else {
			logger.Debugf("The current message, with %v bytes, is larger than the preferred batch size of %v bytes and will be isolated.", messageSizeBytes, r.sharedConfigManager.BatchSize().PreferredMaxBytes)
		}

		messageBatches := [][]*cb.Envelope{}
		committerBatches := [][]filter.Committer{}

		// cut pending batch, if it has any messages
		if len(r.pendingBatch) > 0 {
			messageBatch, committerBatch := r.Cut()
			messageBatches = append(messageBatches, messageBatch)
			committerBatches = append(committerBatches, committerBatch)
		}

		// create new batch with single message
		messageBatches = append(messageBatches, []*cb.Envelope{msg})
		committerBatches = append(committerBatches, []filter.Committer{committer})

		return messageBatches, committerBatches, true
	}

	messageBatches := [][]*cb.Envelope{}
	committerBatches := [][]filter.Committer{}

	messageWillOverflowBatchSizeBytes := r.pendingBatchSizeBytes+messageSizeBytes > r.sharedConfigManager.BatchSize().PreferredMaxBytes

	if messageWillOverflowBatchSizeBytes {
		logger.Debugf("The current message, with %v bytes, will overflow the pending batch of %v bytes.", messageSizeBytes, r.pendingBatchSizeBytes)
		logger.Debugf("Pending batch would overflow if current message is added, cutting batch now.")
		messageBatch, committerBatch := r.Cut()
		messageBatches = append(messageBatches, messageBatch)
		committerBatches = append(committerBatches, committerBatch)
	}

	logger.Debugf("Enqueuing message into batch")
	r.pendingBatch = append(r.pendingBatch, msg)
	r.pendingBatchSizeBytes += messageSizeBytes
	r.pendingCommitters = append(r.pendingCommitters, committer)

	if uint32(len(r.pendingBatch)) >= r.sharedConfigManager.BatchSize().MaxMessageCount {
		logger.Debugf("Batch size met, cutting batch")
		messageBatch, committerBatch := r.Cut()
		messageBatches = append(messageBatches, messageBatch)
		committerBatches = append(committerBatches, committerBatch)
	}

	// return nils instead of empty slices
	if len(messageBatches) == 0 {
		return nil, nil, true
	}

	return messageBatches, committerBatches, true

}

// Cut returns the current batch and starts a new one
func (r *receiver) Cut() ([]*cb.Envelope, []filter.Committer) {
	batch := r.pendingBatch
	r.pendingBatch = nil
	committers := r.pendingCommitters
	r.pendingCommitters = nil
	r.pendingBatchSizeBytes = 0
	return batch, committers
}

func messageSizeBytes(message *cb.Envelope) uint32 {
	return uint32(len(message.Payload) + len(message.Signature))
}
