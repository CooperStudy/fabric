/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package gossip

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
)

type emitBatchCallback func([]interface{})

//batchingEmitter is used for the gossip push/forwarding phase.
// Messages are added into the batchingEmitter, and they are forwarded periodically T times in batches and then discarded.
// If the batchingEmitter's stored message count reaches a certain capacity, that also triggers a message dispatch
type batchingEmitter interface {
	// Add adds a message to be batched
	Add(interface{})

	// Stop stops the component
	Stop()

	// Size returns the amount of pending messages to be emitted
	Size() int
}

// newBatchingEmitter accepts the following parameters:
// iterations: number of times each message is forwarded
// burstSize: a threshold that triggers a forwarding because of message count
// latency: the maximum delay that each message can be stored without being forwarded
// cb: a callback that is called in order for the forwarding to take place
func newBatchingEmitter(iterations, burstSize int, latency time.Duration, cb emitBatchCallback) batchingEmitter {
	fmt.Println("====newBatchingEmitter===")
	if iterations < 0 {
		panic(errors.Errorf("Got a negative iterations number"))
	}

	p := &batchingEmitterImpl{
		cb:         cb,
		delay:      latency,
		iterations: iterations,
		burstSize:  burstSize,
		lock:       &sync.Mutex{},
		buff:       make([]*batchedMessage, 0),
		stopFlag:   int32(0),
	}

	if iterations != 0 {
		go p.periodicEmit()
	}

	return p
}

func (p *batchingEmitterImpl) periodicEmit() {
	//fmt.Println("====batchingEmitterImpl===periodicEmit==")
	for !p.toDie() {
		time.Sleep(p.delay)
		p.lock.Lock()
		p.emit()
		p.lock.Unlock()
	}
}

func (p *batchingEmitterImpl) emit() {
	if p.toDie() {
		return
	}
	if len(p.buff) == 0 {
		return
	}
	msgs2beEmitted := make([]interface{}, len(p.buff))
	for i, v := range p.buff {
		//fmt.Println("=====i",i)
		//fmt.Println("=====v",v)
		msgs2beEmitted[i] = v.data
		//fmt.Println("==v.data===",v.data)
		//GossipMessage: tag:EMPTY alive_msg:<membership:<endpoint:"peer0.org1.example.com:7051" pki_id:"o$\242\205\020\032\336\250\3065\r\354\274\242\3071\360yx\243*\\\250v\303\362\314\247\265\263\340\220" > timestamp:<inc_num:1637029473266694744 seq_num:11 > > , Envelope: 83 bytes, Signature: 71 bytes Secret payload: 29 bytes, Secret Signature: 70 bytes
	}

	//fmt.Println("====msgs2beEmitted======",msgs2beEmitted)
	p.cb(msgs2beEmitted)
	p.decrementCounters()
}

func (p *batchingEmitterImpl) decrementCounters() {
	fmt.Println("====batchingEmitterImpl===decrementCounters==")
	n := len(p.buff)
	//fmt.Println("========n=========",n)
	for i := 0; i < n; i++ {
		msg := p.buff[i]
		msg.iterationsLeft--
		//fmt.Println("=====msg============",msg)
		//fmt.Println("=====msg.iterationsLeft============",msg.iterationsLeft)
		//fmt.Println("+=====p.buff====",p.buff)
		if msg.iterationsLeft == 0 {
			//fmt.Println("=====msg.iterationsLeft=========",msg.iterationsLeft)
			p.buff = append(p.buff[:i], p.buff[i+1:]...)
			//fmt.Println("===========p.buff============",p.buff)
			n--
			//fmt.Println("======n=======",n)
			i--
			//fmt.Println("======i=======",i)
		}
	}
}

func (p *batchingEmitterImpl) toDie() bool {
	return atomic.LoadInt32(&(p.stopFlag)) == int32(1)
}

type batchingEmitterImpl struct {
	iterations int
	burstSize  int
	delay      time.Duration
	cb         emitBatchCallback
	lock       *sync.Mutex
	buff       []*batchedMessage
	stopFlag   int32
}

type batchedMessage struct {
	data           interface{}
	iterationsLeft int
}

func (p *batchingEmitterImpl) Stop() {
	//fmt.Println("====batchingEmitterImpl===Stop==")
	atomic.StoreInt32(&(p.stopFlag), int32(1))
}

func (p *batchingEmitterImpl) Size() int {
	//fmt.Println("====batchingEmitterImpl===Size==")
	p.lock.Lock()
	defer p.lock.Unlock()
	return len(p.buff)
}

func (p *batchingEmitterImpl) Add(message interface{}) {
	//fmt.Println("====batchingEmitterImpl===Add==")
	//fmt.Println("===message=====",message)
	if p.iterations == 0 {
		return
	}
	p.lock.Lock()
	defer p.lock.Unlock()

	p.buff = append(p.buff, &batchedMessage{data: message, iterationsLeft: p.iterations})

	if len(p.buff) >= p.burstSize {
		p.emit()
	}
}
