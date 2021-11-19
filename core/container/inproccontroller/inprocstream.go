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

package inproccontroller

import (
	"errors"
	"fmt"

	pb "github.com/hyperledger/fabric/protos/peer"
)

//SendPanicFailure
type SendPanicFailure string

func (e SendPanicFailure) Error() string {
	fmt.Println("==SendPanicFailure===Error==")
	return fmt.Sprintf("send failure %s", string(e))
}

// PeerChaincodeStream interface for stream between Peer and chaincode instance.
type inProcStream struct {
	recv <-chan *pb.ChaincodeMessage
	send chan<- *pb.ChaincodeMessage
}

func newInProcStream(recv <-chan *pb.ChaincodeMessage, send chan<- *pb.ChaincodeMessage) *inProcStream {
	fmt.Println("==newInProcStream==")
	return &inProcStream{recv, send}
}

func (s *inProcStream) Send(msg *pb.ChaincodeMessage) (err error) {
	fmt.Println("==inProcStream==Send==")
	//send may happen on a closed channel when the system is
	//shutting down. Just catch the exception and return error
	defer func() {
		if r := recover(); r != nil {
			err = SendPanicFailure(fmt.Sprintf("%s", r))
			return
		}
	}()
	fmt.Println("=============s.send <- msg==================",msg)
	/*
	type:TRANSACTION payload:"\n\026getinstalledchaincodes" txid:"2b2c74e60d07bab156af0e01a26f8f864388d828c7574fa70c46113fe8e2cb26" proposal:<proposal_bytes:"\n\264\007\n\\\010\003\032\014\010\225\232\334\214\006\020\373\235\230\273\002*@2b2c74e60d07bab156af0e01a26f8f864388d828c7574fa70c46113fe8e2cb26:\010\022\006\022\004lscc\022\323\006\n\266\006\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKjCCAdGgAwIBAgIRALv8NdhtIyJGHk1DO+cZfC8wCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTE1MDkyNzAwWhcNMzExMTEzMDkyNzAw\nWjBsMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEPMA0GA1UECxMGY2xpZW50MR8wHQYDVQQDDBZBZG1pbkBv\ncmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE/CCjMHcx\nvfqLB3z5Vk2reOaI1PRaaYorpw/Mp7ClnyESh2zNQZs5pgHJ9YvOcldEmBphsFpx\nwNMZzLszVl+rH6NNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYD\nVR0jBCQwIoAgK2A1hrR6qxl2EB98w7Jf2ym/5tg1vgaeXeJ3fLbWL2QwCgYIKoZI\nzj0EAwIDRwAwRAIgad6tAd+oYmiTPBB2jSLMTXsULylfGyDEIal7QWgaf0gCIDCL\njrYGhOqn1dbt/YRLCvDeOdexUJFj/0rFnJ9tWKrl\n-----END CERTIFICATE-----\n\022\030_\230\216\357\333!D\003\355\216'\253\235\376\270\010\316\247\304\321U\210\325\372\022(\n&\n$\010\001\022\006\022\004lscc\032\030\n\026getinstalledchaincodes" signature:"0E\002!\000\240\020ai\267\351\025\225\226\310\214W\240]\213\214\265\215d\303\255V_#\271\377\260'0\253\r\003\002 c5+3\246\256-E\247v\016\314\261I\030\273\251\342\223\306\007*\226(W\326j\313H\320\342\346" >
	type:RESPONSE txid:"984a58ad2309e56ab128dfb77edf139a892dd2df44d53216c4fb3f4b26206a54" channel_id:"mychannel"
	*/
	s.send <- msg
	return
}

func (s *inProcStream) Recv() (*pb.ChaincodeMessage, error) {
	fmt.Println("==inProcStream==Recv==")
	msg, ok := <-s.recv
	if !ok {
		return nil, errors.New("channel is closed")
	}
	return msg, nil
}
