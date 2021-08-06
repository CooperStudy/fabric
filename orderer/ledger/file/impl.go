package fileledger

import (
	"github.com/hyperledger/fabric/common/ledger/blkstorage"
	ledger "github.com/hyperledger/fabric/orderer/ledger"
	cb "github.com/hyperledger/fabric/protos/common"
	ab "github.com/hyperledger/fabric/protos/orderer"
	"github.com/op/go-logging"
)

var logger = logging.MustGetLogger("orderer/fileledger")
var closedChan chan struct{}

func init() {
	closedChan = make(chan struct{})
	close(closedChan)
}
/*
 BlockStore被包装进了fileLedger，是ReadWriter接口的实现。
 */
type fileLedger struct {
	blockStore blkstorage.BlockStore
	signal     chan struct{}
}

//迭代器
type fileLedgerIterator struct {
	ledger      *fileLedger
	blockNumber uint64 //传入一个链上（实际上账本中）查询block开始迭代的位置赋值给迭代器成员blocknumber，比如star两个Position为0（即seek_Position_oldest),则说明
	/*从链的第一块block开始遍历迭代，再比如startPosition为seek_newest,则说明从链的第一个块开始遍历迭代，再比如startPosition为seekPosition_newest，则说明从链的
	最新的一个块block开始遍历迭代，其余的值均算为seekPosition_specified类型的,即任意指定脸上的一个block块。
	任何查询fileLedger的行为，都是通过循坏调用这个迭代器的Next()来实现的，每调用一次，BlockNumber就增1，fileLedgerIterator迭代器的一个特征就是Next()会在迭代到还没有
	写入到账本block（即当前账本最新的block的接下来将要有一个block)时阻塞等待，一致等到该block被写入到账本中，这个阻塞控制涉及到两个chan，impl.go中closedChan和fileLedger
	的成员signal，下面
	*/
}

// Next blocks until there is a new block available, or returns an error if the
// next block is no longer retrievable
func (i *fileLedgerIterator) Next() (*cb.Block, cb.Status) {
	for {
		//当迭代器当前遍历到block序列号小于等于账本上当前最新的block序列号时(即ledger.height()-1)，在next()中，会进入if 分支
		//根据orderer对账本的操作，可知ledger.Height()返回的是链上将要添加的下一个block序列号，查询后返回，当迭代器遍历到ledger.height()
		//的时候，Next()会进入<-i.ledger.signal等待。当一个新的block要被写入账本，怎会调用fileLedger的Append（block),在Append（block）中，
		//把block写入账本成功后，会调用一下close(f1.signal)此时Next()的等待，
		if i.blockNumber < i.ledger.Height() {
			block, err := i.ledger.blockStore.RetrieveBlockByNumber(i.blockNumber)
			if err != nil {
				return nil, cb.Status_SERVICE_UNAVAILABLE
			}
			i.blockNumber++
			return block, cb.Status_SUCCESS
		}
		//这里将继续等待signal，直到在Append(block)中写入新的block后cloose(f1.signal)一下程序才会继续前进，在下文会调用cursor.Next()
		<-i.ledger.signal
	}
}

// ReadyChan supplies a channel which will block until Next will not block
/*
 根据当前迭代器的实际情况进行操作。情况1：当i.blockNumber >　i.ledger.Hegith()-1时，说明迭代器已经遍历需要查询账本的下一个区块
 还没有写入的block了，则返回signal;情况2，相反直接返回closedChan。这个控制会在Deliver服务处理客户端索要block的时候用到，即在
 orderer/common/deliver/deliver.go的handle（）中，
 */
func (i *fileLedgerIterator) ReadyChan() <-chan struct{} {
	signal := i.ledger.signal
	if i.blockNumber > i.ledger.Height()-1 {
		return signal
	}
	//直接返回closedChan
	return closedChan
}

// Iterator returns an Iterator, as specified by a cb.SeekInfo message, and its
// starting block number
func (fl *fileLedger) Iterator(startPosition *ab.SeekPosition) (ledger.Iterator, uint64) {
	switch start := startPosition.Type.(type) {
	case *ab.SeekPosition_Oldest:
		return &fileLedgerIterator{ledger: fl, blockNumber: 0}, 0
	case *ab.SeekPosition_Newest:
		info, err := fl.blockStore.GetBlockchainInfo()
		if err != nil {
			logger.Panic(err)
		}
		newestBlockNumber := info.Height - 1
		return &fileLedgerIterator{ledger: fl, blockNumber: newestBlockNumber}, newestBlockNumber
	case *ab.SeekPosition_Specified:
		height := fl.Height()
		if start.Specified.Number > height {
			return &ledger.NotFoundErrorIterator{}, 0
		}
		return &fileLedgerIterator{ledger: fl, blockNumber: start.Specified.Number}, start.Specified.Number
	default:
		return &ledger.NotFoundErrorIterator{}, 0
	}
}

// Height returns the number of blocks on the ledger
func (fl *fileLedger) Height() uint64 {
	info, err := fl.blockStore.GetBlockchainInfo()
	if err != nil {
		logger.Panic(err)
	}
	return info.Height
}

// Append a new block to the ledger
func (fl *fileLedger) Append(block *cb.Block) error {
	//当一个新的block要被写入账本，则会调用fileLedger的Append（block）
	err := fl.blockStore.AddBlock(block)
	if err == nil {
		//block成功写入账本之后，就会调用close(fl.signal)(此时Next()的等待会结束，再次进入if i.blockNumnber < i.ledger.Height()分支取block），
		close(fl.signal)
		//然后紧接着fl.signal = make(chan struct{}),在给signal创建一个chan（此时若再调用next()，仍会进入<-i.ledger.signal等待）。
		fl.signal = make(chan struct{})
	}
	return err
}
