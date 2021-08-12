package kafka

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Shopify/sarama"
	"github.com/golang/protobuf/proto"
	localconfig "github.com/hyperledger/fabric/orderer/localconfig"
	"github.com/hyperledger/fabric/orderer/multichain"
	cb "github.com/hyperledger/fabric/protos/common"
	ab "github.com/hyperledger/fabric/protos/orderer"
	"github.com/hyperledger/fabric/protos/utils"
)

// Used for capturing metrics -- see processMessagesToBlocks
const (
	indexRecvError = iota
	indexUnmarshalError
	indexRecvPass
	indexProcessConnectPass
	indexProcessTimeToCutError
	indexProcessTimeToCutPass
	indexProcessRegularError
	indexProcessRegularPass
	indexSendTimeToCutError
	indexSendTimeToCutPass
	indexExitChanPass
)

func newChain(consenter commonConsenter, support multichain.ConsenterSupport, lastOffsetPersisted int64) (*chainImpl, error) {
	lastCutBlockNumber := getLastCutBlockNumber(support.Height())
	logger.Infof("[channel: %s] Starting chain with last persisted offset %d and last recorded block %d",
		support.ChainID(), lastOffsetPersisted, lastCutBlockNumber)

	errorChan := make(chan struct{})
	close(errorChan) // We need this closed when starting up

	return &chainImpl{
		consenter:           consenter,
		support:             support,
		channel:             newChannel(support.ChainID(), defaultPartition),
		lastOffsetPersisted: lastOffsetPersisted,
		lastCutBlockNumber:  lastCutBlockNumber,

		errorChan: errorChan,
		haltChan:  make(chan struct{}),
		startChan: make(chan struct{}),
	}, nil
}

type chainImpl struct {
	consenter commonConsenter
	support   multichain.ConsenterSupport

	channel             channel
	lastOffsetPersisted int64
	lastCutBlockNumber  uint64

	producer        sarama.SyncProducer//生产者
	parentConsumer  sarama.Consumer //消费者（负责生成和管理分区消费者的）
	channelConsumer sarama.PartitionConsumer //为分区消费者（实际消费消息的）

	// When the partition consumer errors, close the channel. Otherwise, make
	// this an open, unbuffered channel.
	errorChan chan struct{}
	// When a Halt() request comes, close the channel. Unlike errorChan, this
	// channel never re-opens when closed. Its closing triggers the exit of the
	// processMessagesToBlock loop.
	haltChan chan struct{}
	// // Close when the retriable steps in Start have completed.
	startChan chan struct{}
}

// Errored returns a channel which will close when a partition consumer error
// has occurred. Checked by Deliver().
func (chain *chainImpl) Errored() <-chan struct{} {
	return chain.errorChan
}

// Start allocates the necessary resources for staying up to date with this
// Chain. Implements the multichain.Chain interface. Called by
// multichain.NewManagerImpl() which is invoked when the ordering process is
// launched, before the call to NewServer(). Launches a goroutine so as not to
// block the multichain.Manager.
func (chain *chainImpl) Start() {
	go startThread(chain)
}

// Halt frees the resources which were allocated for this Chain. Implements the
// multichain.Chain interface.
func (chain *chainImpl) Halt() {
	select {
	case <-chain.haltChan:
		// This construct is useful because it allows Halt() to be called
		// multiple times (by a single thread) w/o panicking. Recal that a
		// receive from a closed channel returns (the zero value) immediately.
		logger.Warningf("[channel: %s] Halting of chain requested again", chain.support.ChainID())
	default:
		logger.Criticalf("[channel: %s] Halting of chain requested", chain.support.ChainID())
		close(chain.haltChan)
		chain.closeKafkaObjects() // Also close the producer and the consumer
		logger.Debugf("[channel: %s] Closed the haltChan", chain.support.ChainID())
	}
}

// Enqueue accepts a message and returns true on acceptance, or false otheriwse.
// Implements the multichain.Chain interface. Called by Broadcast().
/*

 */
func (chain *chainImpl) Enqueue(env *cb.Envelope) bool {
	logger.Debugf("[channel: %s] Enqueueing envelope...", chain.support.ChainID())
	//在select最外层的select-case中
	select {
	//如果startChan已经被close，则说明orderer的kafka客户端部分（即orderer/kafka/chain.go的chainImpl）已经一切准备就绪
	case <-chain.startChan: // The Start phase has completed
		select {
		case <-chain.haltChan: // The chain has been halted, stop here
		    //如果chain没有停止，即haltChain没有被close
			logger.Warningf("[channel: %s] Will not enqueue, consenter for this channel has been halted", chain.support.ChainID())
			return false
		default: // The post path
			marshaledEnv, err := utils.Marshal(env)
			if err != nil {
				logger.Errorf("[channel: %s] cannot enqueue, unable to marshal envelope = %s", chain.support.ChainID(), err)
				return false
			}
			// We're good to go
			payload := utils.MarshalOrPanic(newRegularMessage(marshaledEnv))
			//把消息打包成kafka类型，也就是使用sarama预定义的Producer Message类型
			message := newProducerMessage(chain.channel, payload)
			//使用sarama的生产者对象讲kafka消息发送到kafka服务端。
			//kafka服务端（即kafka容器）这个暗盒接收到消息并生产消息
			//在之前chiain启动时接收的kafka服务端消息的线程中，即orderer/kafka/chain.go中的proccessMessagesToBlocks(),
			//kafka服务端生产的消息从startThread的chain.errorChain
			if _, _, err := chain.producer.SendMessage(message); err != nil {
				logger.Errorf("[channel: %s] cannot enqueue envelope = %s", chain.support.ChainID(), err)
				return false
			}
			logger.Debugf("[channel: %s] Envelope enqueued successfully", chain.support.ChainID())
			return true
		}
	default: // Not ready yet
	    //还没准备好，不处理消息返回
		logger.Warningf("[channel: %s] Will not enqueue, consenter for this channel hasn't started yet", chain.support.ChainID())
		return false
	}
}

// Called by Start().
func startThread(chain *chainImpl) {
	var err error

	// Set up the producer
	//创建kafka生产者
	chain.producer, err = setupProducerForChannel(chain.consenter.retryOptions(), chain.haltChan, chain.support.SharedConfig().KafkaBrokers(), chain.consenter.brokerConfig(), chain.channel)
	if err != nil {
		logger.Panicf("[channel: %s] Cannot set up producer = %s", chain.channel.topic(), err)
	}
	logger.Infof("[channel: %s] Producer set up successfully", chain.support.ChainID())

	// Have the producer post the CONNECT message

	if err = sendConnectMessage(chain.consenter.retryOptions(), chain.haltChan, chain.producer, chain.channel); err != nil {
		logger.Panicf("[channel: %s] Cannot post CONNECT message = %s", chain.channel.topic(), err)
	}
	logger.Infof("[channel: %s] CONNECT message posted successfully", chain.channel.topic())

	// Set up the parent consumer
	//创建消费者
	chain.parentConsumer, err = setupParentConsumerForChannel(chain.consenter.retryOptions(), chain.haltChan, chain.support.SharedConfig().KafkaBrokers(), chain.consenter.brokerConfig(), chain.channel)
	if err != nil {
		logger.Panicf("[channel: %s] Cannot set up parent consumer = %s", chain.channel.topic(), err)
	}
	logger.Infof("[channel: %s] Parent consumer set up successfully", chain.channel.topic())

	// Set up the channel consumer
	//分区消费者
	chain.channelConsumer, err = setupChannelConsumerForChannel(chain.consenter.retryOptions(), chain.haltChan, chain.parentConsumer, chain.channel, chain.lastOffsetPersisted+1)
	if err != nil {
		logger.Panicf("[channel: %s] Cannot set up channel consumer = %s", chain.channel.topic(), err)
	}
	logger.Infof("[channel: %s] Channel consumer set up successfully", chain.channel.topic())

	close(chain.startChan)
	// Broadcast requests will now go through
	/*
	 chan.errorChain通道在初始化之初是关闭的，而后chain.errorChain在度生成，算是再度开启。
	如果errorChain是关闭的，
	 */
	chain.errorChan = make(chan struct{}) // Deliver requests will also go through
	//分别开启处理peer节点，通过Broadcast，Deliver发送的请求开关（只有开启orderer才会开始处理peer发来的请求)


	logger.Infof("[channel: %s] Start phase completed successfully", chain.channel.topic())

	//启动了for循环接收处理消息过程，该过程是用来接收kafka服务端（在orderer中就是kafka容器）序列化后，返回给kafka客服端（在orderer中就是
	//sarama的消息流，然后依次分类处理，在processMessagesToBlocks中，正常的kafka生产出来的消息，会从chain。channelConsumer.Messagers()这个通道中出来，
	//根据不同的kafka消息类型，在switch-case中分别使用processConnect，processTimeToCut、KafkaMessage_Regular函数进行处理。
	chain.processMessagesToBlocks() // Keep up to date with the channel
}

// processMessagesToBlocks drains the Kafka consumer for the given channel, and
// takes care of converting the stream of ordered messages into blocks for the
// channel's ledger
/*
 在之前chain启动时接收的kafka服务端消息的线程中，kafka服务端生产的消息
 */
func (chain *chainImpl) processMessagesToBlocks() ([]uint64, error) {
	counts := make([]uint64, 11) // For metrics and tests
	msg := new(ab.KafkaMessage)
	var timer <-chan time.Time//是chainImpl启动的ProcessMessagesToBlocks,而消息来源于kafka服务器出了故障
	//也可能是因为客户端没有新的生产消息发给kafka服务器。在这种情况下blockcutter可能已经缓存了一些之前的消息，为了不使
	//这部分消息丢失并且将其及时记录到账本中（比如kafka处理的数量很小或消息流不是很稳定，一条消息后很长时间都不来下一条消息
	//会造成已缓存的数据不能及时记录到账本，有丢失的风险。也有交易已经产生了很长时间，但是包裹交易的消息一致悬空在blockcutter缓存中，
	//无法被消费，进而无法被记录和查询，configtx。yaml配置中，配置的BatchTimeout规定了超时时间，默认2s，当下一条消息超过2s还没有
	//被消费出来，将会主动cut操作，将现有缓存打包成一个block写入账本。
	/*
	   具体的操作是：a) timer初始化状态为nil，当消息进入processREgular（）中，若缓存进入blockCutter，则会进入if ok && len(cutter) == 0 && *timer == nil分支
	   设timer计时器为2s后触发然后返回。（b)下一条消息若在2s之前再放入processRegular（），若仍是被放入缓存，则依然会进入 if ok && len(cutter) == 0 && *timer == nil分支
	   重置timer为2s及时然后返回。(c)当timer没有及时被重置，即超过2s,processMessagesToBlocks会进入case<-timer:分支，调用sendTimeToCut向kafka服务器发送一条TimeToCutMessage
	   消息（包裹着chain.lastCutBlockNumber+1,即若主动cut，会使用block的序号，接着kafka服务器收到，然后偶被消费出来（这个存在一个时间过程），这条TimeToCutMessage消息会再次进入processMessage
	toBlocks的case in，ok = 《- chain.channelConsumer.Messages():分支，进入进入case ab.kafkaMessgage_TimeToCut：分支，调用processTimeToCut{}处理这条消息
	，也就是主动cut。
	（d)在processTimeToCut（）中，如果哦此时if ttcNumber == *lastCutBlockNumber+1(ttcNumber即为（c)点TimeToCutMessage消息包裹的block序列
	 */
	defer func() { // When Halt() is called
		select {
		case <-chain.errorChan: // If already closed, don't do anything

		default:
			close(chain.errorChan)
		}
	}()

	for {
		select {
		case <-chain.haltChan:
			logger.Warningf("[channel: %s] Consenter for channel exiting", chain.support.ChainID())
			counts[indexExitChanPass]++
			return counts, nil
		case kafkaErr := <-chain.channelConsumer.Errors():
			logger.Errorf("[channel: %s] Error during consumption: %s", chain.support.ChainID(), kafkaErr)
			counts[indexRecvError]++
			select {
			case <-chain.errorChan: // If already closed, don't do anything
			default:
				close(chain.errorChan)
			}
			logger.Warningf("[channel: %s] Closed the errorChan", chain.support.ChainID())
			// This covers the edge case where (1) a consumption error has
			// closed the errorChan and thus rendered the chain unavailable to
			// deliver clients, (2) we're already at the newest offset, and (3)
			// there are no new Broadcast requests coming in. In this case,
			// there is no trigger that can recreate the errorChan again and
			// mark the chain as available, so we have to force that trigger via
			// the emission of a CONNECT message. TODO Consider rate limiting
			go sendConnectMessage(chain.consenter.retryOptions(), chain.haltChan, chain.producer, chain.channel)
			//kafka服务器端生产的消息从case in,ok中出被消费等到ConsumerMessage类型的消费消息，并进入该分支。
			//select-case是一个关于errorChan通道开关，errorChan通道在chain初始化之初是关闭的，而后在startThread()中再度生成，算是再度开启
			//
		case in, ok := <-chain.channelConsumer.Messages():
			if !ok {
				logger.Criticalf("[channel: %s] Kafka consumer closed.", chain.support.ChainID())
				return counts, nil
			}
			/*
			select-case是一个关于errorChan通道开关
			 */
			select {
			/*

			 */
			case <-chain.errorChan: // If this channel was closed...
			    //如果errorChan是关闭的，会进入这里
				chain.errorChan = make(chan struct{}) // 再度开启
				logger.Infof("[channel: %s] Marked consenter as available again", chain.support.ChainID())
			default:
			}
			if err := proto.Unmarshal(in.Value, msg); err != nil {
				// This shouldn't happen, it should be filtered at ingress
				logger.Criticalf("[channel: %s] Unable to unmarshal consumed message = %s", chain.support.ChainID(), err)
				counts[indexUnmarshalError]++
				continue
			} else {
				logger.Debugf("[channel: %s] Successfully unmarshalled consumed message, offset is %d. Inspecting type...", chain.support.ChainID(), in.Offset)
				counts[indexRecvPass]++
			}
			//进入一个switch case根据解压消费消息所携带的数据的类型，进入不同分支来处理消息
			switch msg.Type.(type) {
			case *ab.KafkaMessage_Connect:
				_ = processConnect(chain.support.ChainID())
				counts[indexProcessConnectPass]++
			case *ab.KafkaMessage_TimeToCut:
				/*
				   处理这条消息，
				 */
				if err := processTimeToCut(msg.GetTimeToCut(), chain.support, &chain.lastCutBlockNumber, &timer, in.Offset); err != nil {
					logger.Warningf("[channel: %s] %s", chain.support.ChainID(), err)
					logger.Criticalf("[channel: %s] Consenter for channel exiting", chain.support.ChainID())
					counts[indexProcessTimeToCutError]++
					return counts, err // TODO Revisit whether we should indeed stop processing the chain at this point
				}
				counts[indexProcessTimeToCutPass]++
			case *ab.KafkaMessage_Regular:
				//正常来说分支会走这里，然后被processRegular处理
				//in.Offset 该值是消息在kafka服务端分区中的offset，在整个生产消费过程中，序列化排序会依次分配给每个消息，因此每个消息都有唯一的
				//offset,也因此in.Offset也代表每个具体的消费消息。\
				/*
				  time及时器，和该计时器控制的主动Cut操作。
				 */
				if err := processRegular(msg.GetRegular(), chain.support, &timer, in.Offset, &chain.lastCutBlockNumber); err != nil {
					logger.Warningf("[channel: %s] Error when processing incoming message of type REGULAR = %s", chain.support.ChainID(), err)
					counts[indexProcessRegularError]++
				} else {
					counts[indexProcessRegularPass]++
				}
			}
		case <-timer:
			if err := sendTimeToCut(chain.producer, chain.channel, chain.lastCutBlockNumber+1, &timer); err != nil {
				logger.Errorf("[channel: %s] cannot post time-to-cut message = %s", chain.support.ChainID(), err)
				// Do not return though
				counts[indexSendTimeToCutError]++
			} else {
				counts[indexSendTimeToCutPass]++
			}
		}
	}
}

func (chain *chainImpl) closeKafkaObjects() []error {
	var errs []error

	err := chain.channelConsumer.Close()
	if err != nil {
		logger.Errorf("[channel: %s] could not close channelConsumer cleanly = %s", chain.support.ChainID(), err)
		errs = append(errs, err)
	} else {
		logger.Debugf("[channel: %s] Closed the channel consumer", chain.support.ChainID())
	}

	err = chain.parentConsumer.Close()
	if err != nil {
		logger.Errorf("[channel: %s] could not close parentConsumer cleanly = %s", chain.support.ChainID(), err)
		errs = append(errs, err)
	} else {
		logger.Debugf("[channel: %s] Closed the parent consumer", chain.support.ChainID())
	}

	err = chain.producer.Close()
	if err != nil {
		logger.Errorf("[channel: %s] could not close producer cleanly = %s", chain.support.ChainID(), err)
		errs = append(errs, err)
	} else {
		logger.Debugf("[channel: %s] Closed the producer", chain.support.ChainID())
	}

	return errs
}

// Helper functions

func getLastCutBlockNumber(blockchainHeight uint64) uint64 {
	return blockchainHeight - 1
}

func getLastOffsetPersisted(metadataValue []byte, chainID string) int64 {
	if metadataValue != nil {
		// Extract orderer-related metadata from the tip of the ledger first
		kafkaMetadata := &ab.KafkaMetadata{}
		if err := proto.Unmarshal(metadataValue, kafkaMetadata); err != nil {
			logger.Panicf("[channel: %s] Ledger may be corrupted:"+
				"cannot unmarshal orderer metadata in most recent block", chainID)
		}
		return kafkaMetadata.LastOffsetPersisted
	}
	return (sarama.OffsetOldest - 1) // default
}

func newConnectMessage() *ab.KafkaMessage {
	return &ab.KafkaMessage{
		Type: &ab.KafkaMessage_Connect{
			Connect: &ab.KafkaMessageConnect{
				Payload: nil,
			},
		},
	}
}

func newRegularMessage(payload []byte) *ab.KafkaMessage {
	return &ab.KafkaMessage{
		Type: &ab.KafkaMessage_Regular{
			Regular: &ab.KafkaMessageRegular{
				Payload: payload,
			},
		},
	}
}

func newTimeToCutMessage(blockNumber uint64) *ab.KafkaMessage {
	return &ab.KafkaMessage{
		Type: &ab.KafkaMessage_TimeToCut{
			TimeToCut: &ab.KafkaMessageTimeToCut{
				BlockNumber: blockNumber,
			},
		},
	}
}

func newProducerMessage(channel channel, pld []byte) *sarama.ProducerMessage {
	/*
	使用sarama预定的ProducerMessage类型，Topic定义了消息的类型——Key和Value，key为分区号，value为Marshal过的peer点发过来
	的peer点发来的原始消息
	 */
	return &sarama.ProducerMessage{
		Topic: channel.topic(),
		Key:   sarama.StringEncoder(strconv.Itoa(int(channel.partition()))), // TODO Consider writing an IntEncoder?
		Value: sarama.ByteEncoder(pld),
	}
}

func processConnect(channelName string) error {
	logger.Debugf("[channel: %s] It's a connect message - ignoring", channelName)
	return nil
}

func processRegular(regularMessage *ab.KafkaMessageRegular, support multichain.ConsenterSupport, timer *<-chan time.Time, receivedOffset int64, lastCutBlockNumber *uint64) error {
	env := new(cb.Envelope)
	if err := proto.Unmarshal(regularMessage.Payload, env); err != nil {
		// This shouldn't happen, it should be filtered at ingress
		return fmt.Errorf("unmarshal/%s", err)
	}
	/*
	 将原有的数据（即从peer节点发来的消息)从kafka类型消息消息中分解出来后，交由blockcutter模块的orderer进行分块处理。
	 */
	batches, committers, ok := support.BlockCutter().Ordered(env)
	logger.Debugf("[channel: %s] Ordering results: items in batch = %d, ok = %v", support.ChainID(), len(batches), ok)
	if ok && len(batches) == 0 && *timer == nil {
		*timer = time.After(support.SharedConfig().BatchTimeout())
		logger.Debugf("[channel: %s] Just began %s batch timer", support.ChainID(), support.SharedConfig().BatchTimeout().String())
		return nil
	}
	// If !ok, batches == nil, so this will be skipped
	//当返回1或两批消息，程序就会进入for，依次循环处理每批消息。
	for i, batch := range batches {
		// If more than one batch is produced, exactly 2 batches are produced.
		// The receivedOffset for the first batch is one less than the supplied
		// offset to this function.
		//在循环过程中，（a) 计算当批消息的offset值即最后一条消息的offset值+1
		offset := receivedOffset - int64(len(batches)-i-1)
		//(b) 将消息批量打包成block，至此消息正式成块。
		block := support.CreateNextBlock(batch)
		encodedLastOffsetPersisted := utils.MarshalOrPanic(&ab.KafkaMetadata{LastOffsetPersisted: offset})
		//讲block对应的committer集合，然后讲block吸入账本
		support.WriteBlock(block, committers[i], encodedLastOffsetPersisted)
		/*
		    block := support.CreateNextBlock(batch)
		    support.WriteBlock(block, committers[i], encodedLastOffsetPersisted)
		中，调用了support中的ledger创建block和写入block，在写入block之前会逐一执行该block对应的committer集合。至此orderer
		端处理peer节点发送来的消息流程结束。
		 */
		*lastCutBlockNumber++
		logger.Debugf("[channel: %s] Batch filled, just cut block %d - last persisted offset is now %d", support.ChainID(), *lastCutBlockNumber, offset)
	}
	if len(batches) > 0 {
		*timer = nil
	}
	return nil
}

func processTimeToCut(ttcMessage *ab.KafkaMessageTimeToCut, support multichain.ConsenterSupport, lastCutBlockNumber *uint64, timer *<-chan time.Time, receivedOffset int64) error {
	ttcNumber := ttcMessage.GetBlockNumber()
	logger.Debugf("[channel: %s] It's a time-to-cut message for block %d", support.ChainID(), ttcNumber)
	/*
	如果此时if ttcNumber == *lastCutBlockNumber+1,
	ttcNumber即为TimeToCutMessage消息包裹的block序列号，说明在c点提到的时间过程中
	chain.lastCutBlockNumber的值没有变，也就是没有发生新的Cut,因为一旦有新的Cut发生，chain.lastCutBlockNumber就会增1，
	 */
	if ttcNumber == *lastCutBlockNumber+1 {
		//没有发生新的cut，则说明blockcutter中现在缓存额数据依然是发送timeToCutMessage消息时的数据，或者有新的数据，但是依然没有达到cut的条件
		//此时就会进行主动cut，blockcutter中现有的缓存被打包出来后清空，timer计时器也会被置位nil，在processMessageToBlocks中的for-select-case
		//等待再次从kafka中消费出消息。
		*timer = nil
		logger.Debugf("[channel: %s] Nil'd the timer", support.ChainID())
		batch, committers := support.BlockCutter().Cut()
		if len(batch) == 0 {
			return fmt.Errorf("got right time-to-cut message (for block %d),"+
				" no pending requests though; this might indicate a bug", *lastCutBlockNumber+1)
		}
		//调用了support中的ledge创建block和吸入block，在写入block之前会逐一执行该block对应的committer集合。至此，orderer端处理
		//peer节点发送来的消息的流程结束。
		block := support.CreateNextBlock(batch)
		encodedLastOffsetPersisted := utils.MarshalOrPanic(&ab.KafkaMetadata{LastOffsetPersisted: receivedOffset})
		support.WriteBlock(block, committers, encodedLastOffsetPersisted)
		*lastCutBlockNumber++
		logger.Debugf("[channel: %s] Proper time-to-cut received, just cut block %d", support.ChainID(), *lastCutBlockNumber)
		return nil
	} else if ttcNumber > *lastCutBlockNumber+1 {
		return fmt.Errorf("got larger time-to-cut message (%d) than allowed/expected (%d)"+
			" - this might indicate a bug", ttcNumber, *lastCutBlockNumber+1)
	}
	logger.Debugf("[channel: %s] Ignoring stale time-to-cut-message for block %d", support.ChainID(), ttcNumber)
	return nil
}

// Post a CONNECT message to the channel using the given retry options. This
// prevents the panicking that would occur if we were to set up a consumer and
// seek on a partition that hadn't been written to yet.
func sendConnectMessage(retryOptions localconfig.Retry, exitChan chan struct{}, producer sarama.SyncProducer, channel channel) error {
	logger.Infof("[channel: %s] About to post the CONNECT message...", channel.topic())

	payload := utils.MarshalOrPanic(newConnectMessage())
	message := newProducerMessage(channel, payload)

	retryMsg := "Attempting to post the CONNECT message..."
	postConnect := newRetryProcess(retryOptions, exitChan, channel, retryMsg, func() error {
		_, _, err := producer.SendMessage(message)
		return err
	})

	return postConnect.retry()
}

func sendTimeToCut(producer sarama.SyncProducer, channel channel, timeToCutBlockNumber uint64, timer *<-chan time.Time) error {
	logger.Debugf("[channel: %s] Time-to-cut block %d timer expired", channel.topic(), timeToCutBlockNumber)
	*timer = nil
	payload := utils.MarshalOrPanic(newTimeToCutMessage(timeToCutBlockNumber))
	message := newProducerMessage(channel, payload)
	_, _, err := producer.SendMessage(message)
	return err
}

// Sets up the partition consumer for a channel using the given retry options.
func setupChannelConsumerForChannel(retryOptions localconfig.Retry, haltChan chan struct{}, parentConsumer sarama.Consumer, channel channel, startFrom int64) (sarama.PartitionConsumer, error) {
	var err error
	var channelConsumer sarama.PartitionConsumer

	logger.Infof("[channel: %s] Setting up the channel consumer for this channel (start offset: %d)...", channel.topic(), startFrom)

	retryMsg := "Connecting to the Kafka cluster"
	setupChannelConsumer := newRetryProcess(retryOptions, haltChan, channel, retryMsg, func() error {
		channelConsumer, err = parentConsumer.ConsumePartition(channel.topic(), channel.partition(), startFrom)
		return err
	})

	return channelConsumer, setupChannelConsumer.retry()
}

// Sets up the parent consumer for a channel using the given retry options.
func setupParentConsumerForChannel(retryOptions localconfig.Retry, haltChan chan struct{}, brokers []string, brokerConfig *sarama.Config, channel channel) (sarama.Consumer, error) {
	var err error
	var parentConsumer sarama.Consumer

	logger.Infof("[channel: %s] Setting up the parent consumer for this channel...", channel.topic())

	retryMsg := "Connecting to the Kafka cluster"
	setupParentConsumer := newRetryProcess(retryOptions, haltChan, channel, retryMsg, func() error {
		parentConsumer, err = sarama.NewConsumer(brokers, brokerConfig)
		return err
	})

	return parentConsumer, setupParentConsumer.retry()
}

// Sets up the writer/producer for a channel using the given retry options.
func setupProducerForChannel(retryOptions localconfig.Retry, haltChan chan struct{}, brokers []string, brokerConfig *sarama.Config, channel channel) (sarama.SyncProducer, error) {
	var err error
	var producer sarama.SyncProducer

	logger.Infof("[channel: %s] Setting up the producer for this channel...", channel.topic())

	retryMsg := "Connecting to the Kafka cluster"
	setupProducer := newRetryProcess(retryOptions, haltChan, channel, retryMsg, func() error {
		producer, err = sarama.NewSyncProducer(brokers, brokerConfig)
		return err
	})

	return producer, setupProducer.retry()
}
