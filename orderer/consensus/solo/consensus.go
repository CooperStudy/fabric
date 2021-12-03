/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package solo

import (
	"fmt"
	"time"

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/orderer/consensus"
	cb "github.com/hyperledger/fabric/protos/common"
)

var logger = flogging.MustGetLogger("orderer.consensus.solo")

type consenter struct{}

type chain struct {
	support  consensus.ConsenterSupport
	sendChan chan *message
	exitChan chan struct{}
}

type message struct {
	configSeq uint64
	normalMsg *cb.Envelope
	configMsg *cb.Envelope
}

// New creates a new consenter for the solo consensus scheme.
// The solo consensus scheme is very simple, and allows only one consenter for a given chain (this process).
// It accepts messages being delivered via Order/Configure, orders them, and then uses the blockcutter to form the messages
// into blocks before writing to the given ledger
func New() consensus.Consenter {
	logger.Info("=========New==========")
	return &consenter{}
}

func (solo *consenter) HandleChain(support consensus.ConsenterSupport, metadata *cb.Metadata) (consensus.Chain, error) {
	logger.Info("=========consenter=======HandleChain===")
	return newChain(support), nil
}

func newChain(support consensus.ConsenterSupport) *chain {
	logger.Info("=========newChain===")
	return &chain{
		support:  support,
		sendChan: make(chan *message),
		exitChan: make(chan struct{}),
	}
}

func (ch *chain) Start() {
	go ch.main()
}

func (ch *chain) Halt() {
	logger.Info("=====chain====Halt===")
	select {
	case <-ch.exitChan:
		// Allow multiple halts without panic
	default:
		close(ch.exitChan)
	}
}

func (ch *chain) WaitReady() error {
	logger.Info("=====chain====WaitReady===")
	return nil
}

// Order accepts normal messages for ordering
func (ch *chain) Order(env *cb.Envelope, configSeq uint64) error {
	logger.Info("=====chain====Order===")
	select {
	case ch.sendChan <- &message{
		configSeq: configSeq,
		normalMsg: env,
	}:
		return nil
	case <-ch.exitChan:
		return fmt.Errorf("Exiting")
	}
}

// Configure accepts configuration update messages for ordering
func (ch *chain) Configure(config *cb.Envelope, configSeq uint64) error {
	logger.Info("=====chain====Configure===")
	select {
	case ch.sendChan <- &message{
		configSeq: configSeq,
		configMsg: config,
	}:
		return nil
	case <-ch.exitChan:
		return fmt.Errorf("Exiting")
	}
}

// Errored only closes on exit
func (ch *chain) Errored() <-chan struct{} {
	logger.Info("=====chain====Errored===")
	return ch.exitChan
}

func (ch *chain) main() {
	logger.Info("=====solo==chain====main===")
	var timer <-chan time.Time
	var err error

	for {
		seq := ch.support.Sequence()
		err = nil
		select {
		case msg := <-ch.sendChan:
			//logger.Info("===========case msg := <-ch.sendChan==============")
			//logger.Info("===========msg.configSeq==============")
			//logger.Info("===========msg.configSeq==============",msg.configSeq)
			//if msg.configMsg != nil{
				//logger.Info("===========msg.configMsg.Payload:==============",msg.configMsg.Payload)
				/*

				*/
			//}

			//logger.Info("===========msg.normalMsg==============")
			/*
			create Channel
			 */

			//if msg.normalMsg != nil{
			//	//logger.Info("===========msg.normalMsg.Payload==============",msg.normalMsg.Payload)
			//}
			if msg.configMsg == nil {
				//logger.Info("==============msg.configMsg == nil============")
				// NormalMsg
				//logger.Infof("====msg.configSeq:%v====",msg.configSeq)//0
				//logger.Infof("====seq:%v====",seq)//0
				if msg.configSeq < seq {
					logger.Infof("===_, err = ch.support.ProcessNormalMsg(%v)====",*msg.normalMsg)
					_, err = ch.support.ProcessNormalMsg(msg.normalMsg)
					if err != nil {
						logger.Warningf("Discarding bad normal message: %s", err)
						continue
					}
				}
				logger.Info("==batches, pending := ch.support.BlockCutter().Ordered(msg.normalMsg)===")
				batches, pending := ch.support.BlockCutter().Ordered(msg.normalMsg)

				logger.Info("==========len(batches)=============",len(batches))
				logger.Info("==========pending=============",pending)
				for _, batch := range batches {
					logger.Info("==============batch===========",batch)
					block := ch.support.CreateNextBlock(batch)
					ch.support.WriteBlock(block, nil)
				}

				switch {
				case timer != nil && !pending:
					logger.Info("=========case timer != nil && !pending:==============")
					// Timer is already running but there are no messages pending, stop the timer
					timer = nil
				case timer == nil && pending:
					logger.Info("=========case timer == nil && pending:==============")
					// Timer is not already running and there are messages pending, so start it
					a:= ch.support.SharedConfig().BatchTimeout()
					logger.Infof("==BatchTimeout():%v=",a)
					timer = time.After(a)
					logger.Debugf("Just began %s batch timer", ch.support.SharedConfig().BatchTimeout().String())
				default:
					logger.Info("====default=============")
					// Do nothing when:
					// 1. Timer is already running and there are messages pending
					// 2. Timer is not set and there are no messages pending
				}

			} else {
				// ConfigMsg
				logger.Infof("====msg.configSeq:%v====",msg.configSeq)//0
				logger.Infof("====seq:%v====",seq)//0
				if msg.configSeq < seq {
					msg.configMsg, _, err = ch.support.ProcessConfigMsg(msg.configMsg)
					if err != nil {
						logger.Warningf("Discarding bad config message: %s", err)
						continue
					}
				}
				logger.Info("==batch := ch.support.BlockCutter().Cut()=========")
				/*
				1.createChannel
				 */
				batch := ch.support.BlockCutter().Cut()
				if batch != nil {
					logger.Info("==========block := ch.support.CreateNextBlock(batch)======================")
					block := ch.support.CreateNextBlock(batch)
					logger.Info("=========ch.support.WriteBlock(block, nil)======================")
					ch.support.WriteBlock(block, nil)
				}

				block := ch.support.CreateNextBlock([]*cb.Envelope{msg.configMsg})
				logger.Info("==========solo====ch.support.WriteConfigBlock(block, nil)====")
				ch.support.WriteConfigBlock(block, nil)
				timer = nil
			}
		case <-timer:
			logger.Info("===========case <-timer==============")
			//clear the timer
			timer = nil

			batch := ch.support.BlockCutter().Cut()
			logger.Info("==========len(batch)================",len(batch))
			if len(batch) == 0 {
				logger.Info("Batch timer expired with no pending requests, this might indicate a bug")
				continue
			}
			logger.Debugf("Batch timer expired, creating block")
			block := ch.support.CreateNextBlock(batch)
			ch.support.WriteBlock(block, nil)
		case <-ch.exitChan:
			logger.Info("===========case <-ch.exitChan==============")
			logger.Debugf("Exiting")
			return
		}
	}
}
