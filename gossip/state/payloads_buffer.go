/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package state

import (
	"sync"
	"sync/atomic"

	"github.com/hyperledger/fabric/gossip/util"
	proto "github.com/hyperledger/fabric/protos/gossip"
	"github.com/op/go-logging"
)

// PayloadsBuffer is used to store payloads into which used to
// support payloads with blocks reordering according to the
// sequence numbers. It also will provide the capability
// to signal whenever expected block has arrived.
type PayloadsBuffer interface {
	// Adds new block into the buffer
	Push(payload *proto.Payload)

	// Returns next expected sequence number
	Next() uint64

	// Remove and return payload with given sequence number
	Pop() *proto.Payload

	// Get current buffer size
	Size() int

	// Channel to indicate event when new payload pushed with sequence
	// number equal to the next expected value.
	Ready() chan struct{}

	Close()
}

// PayloadsBufferImpl structure to implement PayloadsBuffer interface
// store inner state of available payloads and sequence numbers
type PayloadsBufferImpl struct {
	next uint64

	buf map[uint64]*proto.Payload

	readyChan chan struct{}

	mutex sync.RWMutex

	logger *logging.Logger
}

// NewPayloadsBuffer is factory function to create new payloads buffer
func NewPayloadsBuffer(next uint64) PayloadsBuffer {
	return &PayloadsBufferImpl{
		buf:       make(map[uint64]*proto.Payload),
		readyChan: make(chan struct{}, 0),
		next:      next,
		logger:    util.GetLogger(util.LoggingStateModule, ""),
	}
}

// Ready function returns the channel which indicates whenever expected
// next block has arrived and one could safely pop out
// next sequence of blocks
func (b *PayloadsBufferImpl) Ready() chan struct{} {
	return b.readyChan
}

// Push new payload into the buffer structure in case new arrived payload
// sequence number is below the expected next block number payload will be
// thrown away and error will be returned.
func (b *PayloadsBufferImpl) Push(payload *proto.Payload) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	seqNum := payload.SeqNum

	if seqNum < b.next || b.buf[seqNum] != nil {
		logger.Debugf("Payload with sequence number = %d has been already processed", payload.SeqNum)
		return
	}
   //buf map[uint64]*proto.Payload
	b.buf[seqNum] = payload

	// Send notification that next sequence has arrived
	/*
	   next指期望的下一个消息序号，消息序号即是一个频道的状态高度，也即一个频道的高度
	   则向readyChan通道发送ready命令以指示期望的消息已经收到，即当前阶段的数据已经准备好了，
	   这里有个前提是Q：ordering服务输出的消息都是排序过并依次输出给state模块的。例如在push（）函数中，
	   若next为2，则说明序号为1的消息之前肯定已经被接收并处理过，当前想要接收序号为2的消息，，在Q的前提下，
	   state模块再接收到消息的序号也只可能是1或2，因为ordering服务可能会因为某些原因重发已经发过的消息1，但是绝不可能
	   从跳过序号为2，而去发序号为3的消息，因此如果若Push的消息序号小于2，则会直接返回。若等于2，则存储进
	   成员buf后，发送ready命令。此外pop函数从payloads中弹出一个消息给state模拟块处理，同时将next增1，
	比如 一旦序号2的消息而被弹出交给state模块处理，则next变为3，即此时序号为2的消息已经在处理，且payloads的消息排序输出功能
	 */
	if seqNum == b.next {
		// Do not block execution of current routine
		go func() {
			b.readyChan <- struct{}{}
		}()
	}
}

// Next function provides the number of the next expected block
func (b *PayloadsBufferImpl) Next() uint64 {
	// Atomically read the value of the top sequence number
	return atomic.LoadUint64(&b.next)
}

// Pop function extracts the payload according to the next expected block
// number, if no next block arrived yet, function returns nil.
func (b *PayloadsBufferImpl) Pop() *proto.Payload {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	result := b.buf[b.Next()]

	if result != nil {
		// If there is such sequence in the buffer need to delete it
		delete(b.buf, b.Next())
		// Increment next expect block index
		atomic.AddUint64(&b.next, 1)
	}
	return result
}

// Size returns current number of payloads stored within buffer
func (b *PayloadsBufferImpl) Size() int {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return len(b.buf)
}

// Close cleanups resources and channels in maintained
func (b *PayloadsBufferImpl) Close() {
	close(b.readyChan)
}
