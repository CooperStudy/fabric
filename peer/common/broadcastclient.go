/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

                 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	cb "github.com/hyperledger/fabric/protos/common"
	ab "github.com/hyperledger/fabric/protos/orderer"
	"github.com/pkg/errors"
)

type BroadcastClient interface {
	//Send data to orderer
	Send(env *cb.Envelope) error
	Close() error
}

type broadcastClient struct {
	client ab.AtomicBroadcast_BroadcastClient
}

// GetBroadcastClient creates a simple instance of the BroadcastClient interface
func GetBroadcastClient() (BroadcastClient, error) {
	logger.Info("====GetBroadcastClient====")
	oc, err := NewOrdererClientFromEnv()
	if err != nil {
		return nil, err
	}
	bc, err := oc.Broadcast()
	if err != nil {
		return nil, err
	}

	return &broadcastClient{client: bc}, nil
}

func (s *broadcastClient) getAck() error {
	logger.Info("====broadcastClient==getAck==")
	msg, err := s.client.Recv()
	logger.Info("========msg===========",msg)
	if err != nil {
		return err
	}
	if msg.Status != cb.Status_SUCCESS {
		return errors.Errorf("got unexpected status: %v -- %s", msg.Status, msg.Info)
	}
	return nil
}

//Send data to orderer
func (s *broadcastClient) Send(env *cb.Envelope) error {
	/*
	1.create channel
	cong here
	 */
	logger.Info("====broadcastClient==Send==")
	logger.Infof("======send envelope:%v",*env)
	/*
	create Channel
	 */
	if err := s.client.Send(env); err != nil {
		return errors.WithMessage(err, "could not send")
	}

	err := s.getAck()

	return err
}

func (s *broadcastClient) Close() error {
	logger.Info("====broadcastClient==Close==")
	return s.client.CloseSend()
}
