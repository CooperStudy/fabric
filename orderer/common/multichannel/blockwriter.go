/*
Copyright IBM Corp. 2017 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package multichannel

import (
	"sync"

	"github.com/golang/protobuf/proto"
	newchannelconfig "github.com/hyperledger/fabric/common/channelconfig"
	"github.com/hyperledger/fabric/common/configtx"
	"github.com/hyperledger/fabric/common/crypto"
	"github.com/hyperledger/fabric/common/ledger/blockledger"
	"github.com/hyperledger/fabric/common/util"
	cb "github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/utils"
)

type blockWriterSupport interface {
	crypto.LocalSigner
	blockledger.ReadWriter
	configtx.Validator
	Update(*newchannelconfig.Bundle)
	CreateBundle(channelID string, config *cb.Config) (*newchannelconfig.Bundle, error)
}

// BlockWriter efficiently writes the blockchain to disk.
// To safely use BlockWriter, only one thread should interact with it.
// BlockWriter will spawn additional committing go routines and handle locking
// so that these other go routines safely interact with the calling one.
type BlockWriter struct {
	support            blockWriterSupport
	registrar          *Registrar
	lastConfigBlockNum uint64
	lastConfigSeq      uint64
	lastBlock          *cb.Block
	committingBlock    sync.Mutex
}

func newBlockWriter(lastBlock *cb.Block, r *Registrar, support blockWriterSupport) *BlockWriter {
	logger.Info("==newBlockWriter==")
	bw := &BlockWriter{

		support:       support,
		lastConfigSeq: support.Sequence(),
		lastBlock:     lastBlock,
		registrar:     r,
	}

	// If this is the genesis block, the lastconfig field may be empty, and, the last config block is necessarily block 0
	// so no need to initialize lastConfig
	if lastBlock.Header.Number != 0 {
		var err error
		bw.lastConfigBlockNum, err = utils.GetLastConfigIndexFromBlock(lastBlock)
		if err != nil {
			logger.Panicf("[channel: %s] Error extracting last config block from block metadata: %s", support.ChainID(), err)
		}
	}

	logger.Debugf("[channel: %s] Creating block writer for tip of chain (blockNumber=%d, lastConfigBlockNum=%d, lastConfigSeq=%d)", support.ChainID(), lastBlock.Header.Number, bw.lastConfigBlockNum, bw.lastConfigSeq)
	return bw
}

// CreateNextBlock creates a new block with the next block number, and the given contents.
func (bw *BlockWriter) CreateNextBlock(messages []*cb.Envelope) *cb.Block {
	logger.Info("==newBlockWriter=CreateNextBlock===")
	logger.Info("=====previousBlockHash====")
	previousBlockHash := bw.lastBlock.Header.Hash()

	data := &cb.BlockData{
		Data: make([][]byte, len(messages)),
	}

	/*
	envelope是交易信息，存储在block.Data中
	 */
	var err error
	for i, msg := range messages {
		//logger.Infof("=====i:%v",i)//0
		//logger.Infof("=====msg:%v",msg)
		/*
		1.create Channel
		=====msg:payload:"\n\316\006\n\034\010\004\032\006\010\223\240\247\215\006\"\020byfn-sys-channel\022\255\006\n\220\006\n\nOrdererMSP\022\201\006-----BEGIN CERTIFICATE-----\nMIICDDCCAbKgAwIBAgIQO7f5H/GNWl4bCFrav0m09TAKBggqhkjOPQQDAjBpMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEUMBIGA1UEChMLZXhhbXBsZS5jb20xFzAVBgNVBAMTDmNhLmV4YW1w\nbGUuY29tMB4XDTIxMTIwMjA3NTAwMFoXDTMxMTEzMDA3NTAwMFowWDELMAkGA1UE\nBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBGcmFuY2lz\nY28xHDAaBgNVBAMTE29yZGVyZXIuZXhhbXBsZS5jb20wWTATBgcqhkjOPQIBBggq\nhkjOPQMBBwNCAAQ50oX9JKkYpZg+CKMMHRBR9nWjdOLgD9yHfpiM01f+L64b1eEg\nBpwz0ci6KItxvX+j81Q4dRjV0/Tw/eSI7fllo00wSzAOBgNVHQ8BAf8EBAMCB4Aw\nDAYDVR0TAQH/BAIwADArBgNVHSMEJDAigCAiKUD447X2wODjavJYziGH+yguMn94\nPA285AcRrKlxzzAKBggqhkjOPQQDAgNIADBFAiEAvBTKhKJ1q9+Tto05f7a01B0V\n3hORQui4xrriz0G5Z1MCID6i4ksld0Nzhr+45XgZRcMxC+Q8SzEpbXWJDTTo+tIl\n-----END CERTIFICATE-----\n\022\030^\213\260\226\300\316:cc\034\275|\322\375\035f\017\303Qs\314\t\025\225\022\264|\n\350{\n\307\006\n\025\010\001\032\006\010\223\240\247\215\006\"\tmychannel\022\255\006\n\220\006\n\nOrdererMSP\022\201\006-----BEGIN CERTIFICATE-----\nMIICDDCCAbKgAwIBAgIQO7f5H/GNWl4bCFrav0m09TAKBggqhkjOPQQDAjBpMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEUMBIGA1UEChMLZXhhbXBsZS5jb20xFzAVBgNVBAMTDmNhLmV4YW1w\nbGUuY29tMB4XDTIxMTIwMjA3NTAwMFoXDTMxMTEzMDA3NTAwMFowWDELMAkGA1UE\nBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBGcmFuY2lz\nY28xHDAaBgNVBAMTE29yZGVyZXIuZXhhbXBsZS5jb20wWTATBgcqhkjOPQIBBggq\nhkjOPQMBBwNCAAQ50oX9JKkYpZg+CKMMHRBR9nWjdOLgD9yHfpiM01f+L64b1eEg\nBpwz0ci6KItxvX+j81Q4dRjV0/Tw/eSI7fllo00wSzAOBgNVHQ8BAf8EBAMCB4Aw\nDAYDVR0TAQH/BAIwADArBgNVHSMEJDAigCAiKUD447X2wODjavJYziGH+yguMn94\nPA285AcRrKlxzzAKBggqhkjOPQQDAgNIADBFAiEAvBTKhKJ1q9+Tto05f7a01B0V\n3hORQui4xrriz0G5Z1MCID6i4ksld0Nzhr+45XgZRcMxC+Q8SzEpbXWJDTTo+tIl\n-----END CERTIFICATE-----\n\022\030\347\030\201\222\223\331\2105\036\215\271\002\r\001?|/^qEZ\302\256u\022\233u\n\372c\010\001\022\365c\022\351\027\n\007Orderer\022\335\027\022\214\025\n\nOrdererOrg\022\375\024\032\322\023\n\003MSP\022\312\023\022\277\023\022\274\023\n\nOrdererMSP\022\307\006-----BEGIN CERTIFICATE-----\nMIICPjCCAeSgAwIBAgIRAIjw5hgPtAkO5lYCXaEaMLUwCgYIKoZIzj0EAwIwaTEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xFDASBgNVBAoTC2V4YW1wbGUuY29tMRcwFQYDVQQDEw5jYS5leGFt\ncGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUwMDBaMGkxCzAJBgNV\nBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1TYW4gRnJhbmNp\nc2NvMRQwEgYDVQQKEwtleGFtcGxlLmNvbTEXMBUGA1UEAxMOY2EuZXhhbXBsZS5j\nb20wWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAARSnHvISLMxgSMdtZf+YbyVAY7s\nYbw4FSguR2FZcmIIwghcF5ILwpLhqL11TWhrCAFBGzECp6IkMZbkGphCi6plo20w\nazAOBgNVHQ8BAf8EBAMCAaYwHQYDVR0lBBYwFAYIKwYBBQUHAwIGCCsGAQUFBwMB\nMA8GA1UdEwEB/wQFMAMBAf8wKQYDVR0OBCIEICIpQPjjtfbA4ONq8ljOIYf7KC4y\nf3g8DbzkBxGsqXHPMAoGCCqGSM49BAMCA0gAMEUCIQCUHJm1/UtxamrMcj2YCa2S\nyddnoNtvnJXMVkJkrBdzvAIgKIMjLNuN1nNpILYI0K9OEMTW5Ty9oe5xZixGWNy5\nGRE=\n-----END CERTIFICATE-----\n\"\201\006-----BEGIN CERTIFICATE-----\nMIICCjCCAbGgAwIBAgIRAPGa4DAlJWzSR0M+g2HlvIgwCgYIKoZIzj0EAwIwaTEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xFDASBgNVBAoTC2V4YW1wbGUuY29tMRcwFQYDVQQDEw5jYS5leGFt\ncGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUwMDBaMFYxCzAJBgNV\nBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1TYW4gRnJhbmNp\nc2NvMRowGAYDVQQDDBFBZG1pbkBleGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqG\nSM49AwEHA0IABNttJVHFgo5jzOfKKr+CZ5swd0qD3SlzqJSyXnP9xxGMKvRDbtuT\nZ/iDvSRTXHLmnuCTaZlm/jywYHC+nzWiPcOjTTBLMA4GA1UdDwEB/wQEAwIHgDAM\nBgNVHRMBAf8EAjAAMCsGA1UdIwQkMCKAICIpQPjjtfbA4ONq8ljOIYf7KC4yf3g8\nDbzkBxGsqXHPMAoGCCqGSM49BAMCA0cAMEQCIGOptxqHKGK8Jx8/9e51v5uR7LrC\nddbj+fNM9+qkBmUlAiAvFmNDoA4h7kMKFdUrZd4k6jmHevxQtoN4ZSaSWW/1uw==\n-----END CERTIFICATE-----\nB\016\n\004SHA2\022\006SHA256J\317\006-----BEGIN CERTIFICATE-----\nMIICRDCCAeqgAwIBAgIRAOf9IIw7LUq0FAGT1hV84N0wCgYIKoZIzj0EAwIwbDEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xFDASBgNVBAoTC2V4YW1wbGUuY29tMRowGAYDVQQDExF0bHNjYS5l\neGFtcGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUwMDBaMGwxCzAJ\nBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1TYW4gRnJh\nbmNpc2NvMRQwEgYDVQQKEwtleGFtcGxlLmNvbTEaMBgGA1UEAxMRdGxzY2EuZXhh\nbXBsZS5jb20wWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAARZ8WTu9AoXBdvT7BYL\n2pvIVzsDR3vmsT5u89mjCtVdb88r5kupGJnOOGfZQis2pzrF4wJ0o18Yt9fdjzxZ\nICsVo20wazAOBgNVHQ8BAf8EBAMCAaYwHQYDVR0lBBYwFAYIKwYBBQUHAwIGCCsG\nAQUFBwMBMA8GA1UdEwEB/wQFMAMBAf8wKQYDVR0OBCIEIP+UJHfeNf6dBDc3gTWV\nReWye1cL41Spr1BkNX8GROuFMAoGCCqGSM49BAMCA0gAMEUCIQDSEr3H/yRfYj32\n0n3xOgwTnuhgiiP4C7w3LmWzF93xswIgIaagjKl5bCjUT2G21/PGvzKLH0+CQxln\nQ4i41pFddCc=\n-----END CERTIFICATE-----\n\032\006Admins\"3\n\007Writers\022(\022\036\010\001\022\032\022\010\022\006\010\001\022\002\010\000\032\016\022\014\n\nOrdererMSP\032\006Admins\"4\n\006Admins\022*\022 \010\001\022\034\022\010\022\006\010\001\022\002\010\000\032\020\022\016\n\nOrdererMSP\020\001\032\006Admins\"3\n\007Readers\022(\022\036\010\001\022\032\022\010\022\006\010\001\022\002\010\000\032\016\022\014\n\nOrdererMSP\032\006Admins*\006Admins\032\037\n\023ChannelRestrictions\022\010\032\006Admins\032$\n\014Capabilities\022\024\022\n\n\010\n\004V1_1\022\000\032\006Admins\032!\n\rConsensusType\022\020\022\006\n\004solo\032\006Admins\032\"\n\tBatchSize\022\025\022\013\010\n\020\200\200\3001\030\200\200 \032\006Admins\032\036\n\014BatchTimeout\022\016\022\004\n\0022s\032\006Admins\"\"\n\006Admins\022\030\022\016\010\003\022\n\n\006Admins\020\002\032\006Admins\"*\n\017BlockValidation\022\027\022\r\010\003\022\t\n\007Writers\032\006Admins\"\"\n\007Readers\022\027\022\r\010\003\022\t\n\007Readers\032\006Admins\"\"\n\007Writers\022\027\022\r\010\003\022\t\n\007Writers\032\006Admins*\006Admins\022\236I\n\013Application\022\216I\010\001\022\360#\n\007Org1MSP\022\344#\032\205\"\n\003MSP\022\375!\022\362!\022\357!\n\007Org1MSP\022\337\006-----BEGIN CERTIFICATE-----\nMIICUTCCAfegAwIBAgIQTDVrkQN1dC1Xkq0gNdWmNTAKBggqhkjOPQQDAjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEcMBoGA1UEAxMTY2Eu\nb3JnMS5leGFtcGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUwMDBa\nMHMxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMRkwFwYDVQQKExBvcmcxLmV4YW1wbGUuY29tMRwwGgYDVQQD\nExNjYS5vcmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE\nhU1Xj6u7o8o3vTpBMx0XgaKCHYIfwWW4joguVanxY7l2FoRsdeMlbSLpIbGOVeq2\nrqy5H1C1m+d1lvV5eUjlZaNtMGswDgYDVR0PAQH/BAQDAgGmMB0GA1UdJQQWMBQG\nCCsGAQUFBwMCBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdDgQiBCBc\npldgEIiaAL2L5u/dXT77TRUFw4c4xvgl33Q0ZMUW5jAKBggqhkjOPQQDAgNIADBF\nAiEA1moRbtOf7FmKUBtnlbWOeKW1+Ou1gECRbRmk28u82qYCIHCl4liG91dRkZJV\ngHFQhMyFuyn1TBkuY3VdWoTMfntJ\n-----END CERTIFICATE-----\n\"\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAdCgAwIBAgIQdoMtXcRmr8XdkhTXGAwFzDAKBggqhkjOPQQDAjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEcMBoGA1UEAxMTY2Eu\nb3JnMS5leGFtcGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUwMDBa\nMGwxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMQ8wDQYDVQQLEwZjbGllbnQxHzAdBgNVBAMMFkFkbWluQG9y\nZzEuZXhhbXBsZS5jb20wWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAR4lTOcEwH/\n7BA0PJHXxXCQWTvUHqLrw/m1noiFjGZa7wQWZ1NQ+AbOJ8tydWS3xu5YVw3KegfZ\naXcQ2VMAAYsIo00wSzAOBgNVHQ8BAf8EBAMCB4AwDAYDVR0TAQH/BAIwADArBgNV\nHSMEJDAigCBcpldgEIiaAL2L5u/dXT77TRUFw4c4xvgl33Q0ZMUW5jAKBggqhkjO\nPQQDAgNHADBEAiBKZAEOoM00vmTIR0pP7XFBp41hvVL4PGwISqi0v4AwTgIgUB5T\n6gaVVogvaUeMmCqljNGNupyBj1iY0jxDlvGteNA=\n-----END CERTIFICATE-----\nB\016\n\004SHA2\022\006SHA256J\347\006-----BEGIN CERTIFICATE-----\nMIICVzCCAf6gAwIBAgIRAIrP4xsYBvSOZyzAtxl2gV4wCgYIKoZIzj0EAwIwdjEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHzAdBgNVBAMTFnRs\nc2NhLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMjAyMDc1MDAwWhcNMzExMTMwMDc1\nMDAwWjB2MQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UE\nBxMNU2FuIEZyYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEfMB0G\nA1UEAxMWdGxzY2Eub3JnMS5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49\nAwEHA0IABCHAG6UlLVndKC68Ot/m6FZUP5gki9Kt630bQcJvQKlLm9aT1n+h1Zkq\ntE+cxCs7cnkD9k1Vmroyi4zcnVvD4J6jbTBrMA4GA1UdDwEB/wQEAwIBpjAdBgNV\nHSUEFjAUBggrBgEFBQcDAgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUwAwEB/zApBgNV\nHQ4EIgQgDrK2cbW+BdW3uqYuSM8QYsCGsvyyFbskdXhzTBohIy4wCgYIKoZIzj0E\nAwIDRwAwRAIgVTFpaBTqyMxVfDACj3OIjwIS6fUZBDyl5w9x1iuD/JwCIHQ7/Azh\ncmuA1FJLOSMjVFCms/hPgatGAENmKoNhyeUV\n-----END CERTIFICATE-----\nZ\332\r\010\001\022\352\006\n\337\006-----BEGIN CERTIFICATE-----\nMIICUTCCAfegAwIBAgIQTDVrkQN1dC1Xkq0gNdWmNTAKBggqhkjOPQQDAjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEcMBoGA1UEAxMTY2Eu\nb3JnMS5leGFtcGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUwMDBa\nMHMxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMRkwFwYDVQQKExBvcmcxLmV4YW1wbGUuY29tMRwwGgYDVQQD\nExNjYS5vcmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE\nhU1Xj6u7o8o3vTpBMx0XgaKCHYIfwWW4joguVanxY7l2FoRsdeMlbSLpIbGOVeq2\nrqy5H1C1m+d1lvV5eUjlZaNtMGswDgYDVR0PAQH/BAQDAgGmMB0GA1UdJQQWMBQG\nCCsGAQUFBwMCBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdDgQiBCBc\npldgEIiaAL2L5u/dXT77TRUFw4c4xvgl33Q0ZMUW5jAKBggqhkjOPQQDAgNIADBF\nAiEA1moRbtOf7FmKUBtnlbWOeKW1+Ou1gECRbRmk28u82qYCIHCl4liG91dRkZJV\ngHFQhMyFuyn1TBkuY3VdWoTMfntJ\n-----END CERTIFICATE-----\n\022\006client\032\350\006\n\337\006-----BEGIN CERTIFICATE-----\nMIICUTCCAfegAwIBAgIQTDVrkQN1dC1Xkq0gNdWmNTAKBggqhkjOPQQDAjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEcMBoGA1UEAxMTY2Eu\nb3JnMS5leGFtcGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUwMDBa\nMHMxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMRkwFwYDVQQKExBvcmcxLmV4YW1wbGUuY29tMRwwGgYDVQQD\nExNjYS5vcmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE\nhU1Xj6u7o8o3vTpBMx0XgaKCHYIfwWW4joguVanxY7l2FoRsdeMlbSLpIbGOVeq2\nrqy5H1C1m+d1lvV5eUjlZaNtMGswDgYDVR0PAQH/BAQDAgGmMB0GA1UdJQQWMBQG\nCCsGAQUFBwMCBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdDgQiBCBc\npldgEIiaAL2L5u/dXT77TRUFw4c4xvgl33Q0ZMUW5jAKBggqhkjOPQQDAgNIADBF\nAiEA1moRbtOf7FmKUBtnlbWOeKW1+Ou1gECRbRmk28u82qYCIHCl4liG91dRkZJV\ngHFQhMyFuyn1TBkuY3VdWoTMfntJ\n-----END CERTIFICATE-----\n\022\004peer\032\006Admins\"X\n\007Readers\022M\022C\010\001\022?\022\020\022\016\010\001\022\002\010\000\022\002\010\001\022\002\010\002\032\r\022\013\n\007Org1MSP\020\001\032\r\022\013\n\007Org1MSP\020\003\032\r\022\013\n\007Org1MSP\020\002\032\006Admins\"E\n\007Writers\022:\0220\010\001\022,\022\014\022\n\010\001\022\002\010\000\022\002\010\001\032\r\022\013\n\007Org1MSP\020\001\032\r\022\013\n\007Org1MSP\020\002\032\006Admins\"1\n\006Admins\022'\022\035\010\001\022\031\022\010\022\006\010\001\022\002\010\000\032\r\022\013\n\007Org1MSP\020\001\032\006Admins*\006Admins\022\374#\n\007Org2MSP\022\360#\032\221\"\n\003MSP\022\211\"\022\376!\022\373!\n\007Org2MSP\022\343\006-----BEGIN CERTIFICATE-----\nMIICUjCCAfigAwIBAgIRAIfgoJC0Df7eMV4lqQemX0kwCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzIuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzIuZXhhbXBsZS5jb20wHhcNMjExMjAyMDc1MDAwWhcNMzExMTMwMDc1MDAw\nWjBzMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEZMBcGA1UEChMQb3JnMi5leGFtcGxlLmNvbTEcMBoGA1UE\nAxMTY2Eub3JnMi5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IA\nBLAZQE8DcMvRZPlkqySmSoitv7Lfz6pT8W7QA//PjKnk8nEqr/yECkTkFVUV2dF3\nMxqxgneiLngAdnyV8z9KnGujbTBrMA4GA1UdDwEB/wQEAwIBpjAdBgNVHSUEFjAU\nBggrBgEFBQcDAgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUwAwEB/zApBgNVHQ4EIgQg\nsbNOCtP+b8NXOrQNEZbQ8HSuvUvMMM6VPQZOVuAR4wswCgYIKoZIzj0EAwIDSAAw\nRQIhAOBO1HY1Wr0IHqnatm7PiR6SDpfyn5k86sIVUedxYJN3AiBlcomKBybUeYyV\neDgzidbY7VsDyn/GiFNGGmE+qyhX8g==\n-----END CERTIFICATE-----\n\"\252\006-----BEGIN CERTIFICATE-----\nMIICKjCCAdGgAwIBAgIRAJboIErw+IB46bNF6Xbw1FAwCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzIuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzIuZXhhbXBsZS5jb20wHhcNMjExMjAyMDc1MDAwWhcNMzExMTMwMDc1MDAw\nWjBsMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEPMA0GA1UECxMGY2xpZW50MR8wHQYDVQQDDBZBZG1pbkBv\ncmcyLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEzaHgcN4f\npClunjfoJiv9SlX144pk2pTRVkDVdOxnSewu09NSnr0er1f3Ro6VUuSxNtj8t9ne\neya8Bb9tvocGbaNNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYD\nVR0jBCQwIoAgsbNOCtP+b8NXOrQNEZbQ8HSuvUvMMM6VPQZOVuAR4wswCgYIKoZI\nzj0EAwIDRwAwRAIgaXs4/mABuf6WEAftlK9yjyZ+1mdvRkBHqni4jEy5lS4CID6W\nwD2GOZaiM82phTDNRW0HSjgj7rGMP3aWSG5I9AdC\n-----END CERTIFICATE-----\nB\016\n\004SHA2\022\006SHA256J\347\006-----BEGIN CERTIFICATE-----\nMIICVjCCAf2gAwIBAgIQUhxZJYzGqzE5nyITMC9zLjAKBggqhkjOPQQDAjB2MQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMi5leGFtcGxlLmNvbTEfMB0GA1UEAxMWdGxz\nY2Eub3JnMi5leGFtcGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUw\nMDBaMHYxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQH\nEw1TYW4gRnJhbmNpc2NvMRkwFwYDVQQKExBvcmcyLmV4YW1wbGUuY29tMR8wHQYD\nVQQDExZ0bHNjYS5vcmcyLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0D\nAQcDQgAEn8ukwF5H4XKARUcET+ZFGK/cPt6asGFzVN8fOvRFKW51mzLSZ9Q5e8FW\nZdl9lwBQ3l1bMlOLtrI5aYqJ3E3TvqNtMGswDgYDVR0PAQH/BAQDAgGmMB0GA1Ud\nJQQWMBQGCCsGAQUFBwMCBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1Ud\nDgQiBCCmeUdJn2MdTwQa/yo0kzdFNfThCql71zir++OQk6PgnDAKBggqhkjOPQQD\nAgNHADBEAiBqPLft3f8gyeIucgxwXXyFP96ynn80yEiD+dH8MK5REwIgMwm4qBgq\nOFVPRino2DG0+DW1Z4T4VoCCwAbYg39RqFI=\n-----END CERTIFICATE-----\nZ\342\r\010\001\022\356\006\n\343\006-----BEGIN CERTIFICATE-----\nMIICUjCCAfigAwIBAgIRAIfgoJC0Df7eMV4lqQemX0kwCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzIuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzIuZXhhbXBsZS5jb20wHhcNMjExMjAyMDc1MDAwWhcNMzExMTMwMDc1MDAw\nWjBzMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEZMBcGA1UEChMQb3JnMi5leGFtcGxlLmNvbTEcMBoGA1UE\nAxMTY2Eub3JnMi5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IA\nBLAZQE8DcMvRZPlkqySmSoitv7Lfz6pT8W7QA//PjKnk8nEqr/yECkTkFVUV2dF3\nMxqxgneiLngAdnyV8z9KnGujbTBrMA4GA1UdDwEB/wQEAwIBpjAdBgNVHSUEFjAU\nBggrBgEFBQcDAgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUwAwEB/zApBgNVHQ4EIgQg\nsbNOCtP+b8NXOrQNEZbQ8HSuvUvMMM6VPQZOVuAR4wswCgYIKoZIzj0EAwIDSAAw\nRQIhAOBO1HY1Wr0IHqnatm7PiR6SDpfyn5k86sIVUedxYJN3AiBlcomKBybUeYyV\neDgzidbY7VsDyn/GiFNGGmE+qyhX8g==\n-----END CERTIFICATE-----\n\022\006client\032\354\006\n\343\006-----BEGIN CERTIFICATE-----\nMIICUjCCAfigAwIBAgIRAIfgoJC0Df7eMV4lqQemX0kwCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzIuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzIuZXhhbXBsZS5jb20wHhcNMjExMjAyMDc1MDAwWhcNMzExMTMwMDc1MDAw\nWjBzMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEZMBcGA1UEChMQb3JnMi5leGFtcGxlLmNvbTEcMBoGA1UE\nAxMTY2Eub3JnMi5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IA\nBLAZQE8DcMvRZPlkqySmSoitv7Lfz6pT8W7QA//PjKnk8nEqr/yECkTkFVUV2dF3\nMxqxgneiLngAdnyV8z9KnGujbTBrMA4GA1UdDwEB/wQEAwIBpjAdBgNVHSUEFjAU\nBggrBgEFBQcDAgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUwAwEB/zApBgNVHQ4EIgQg\nsbNOCtP+b8NXOrQNEZbQ8HSuvUvMMM6VPQZOVuAR4wswCgYIKoZIzj0EAwIDSAAw\nRQIhAOBO1HY1Wr0IHqnatm7PiR6SDpfyn5k86sIVUedxYJN3AiBlcomKBybUeYyV\neDgzidbY7VsDyn/GiFNGGmE+qyhX8g==\n-----END CERTIFICATE-----\n\022\004peer\032\006Admins\"X\n\007Readers\022M\022C\010\001\022?\022\020\022\016\010\001\022\002\010\000\022\002\010\001\022\002\010\002\032\r\022\013\n\007Org2MSP\020\001\032\r\022\013\n\007Org2MSP\020\003\032\r\022\013\n\007Org2MSP\020\002\032\006Admins\"E\n\007Writers\022:\0220\010\001\022,\022\014\022\n\010\001\022\002\010\000\022\002\010\001\032\r\022\013\n\007Org2MSP\020\001\032\r\022\013\n\007Org2MSP\020\002\032\006Admins\"1\n\006Admins\022'\022\035\010\001\022\031\022\010\022\006\010\001\022\002\010\000\032\r\022\013\n\007Org2MSP\020\001\032\006Admins*\006Admins\032$\n\014Capabilities\022\024\022\n\n\010\n\004V1_3\022\000\032\006Admins\"\"\n\006Admins\022\030\022\016\010\003\022\n\n\006Admins\020\002\032\006Admins\"\"\n\007Readers\022\027\022\r\010\003\022\t\n\007Readers\032\006Admins\"\"\n\007Writers\022\027\022\r\010\003\022\t\n\007Writers\032\006Admins*\006Admins\032&\n\020HashingAlgorithm\022\022\022\010\n\006SHA256\032\006Admins\032*\n\nConsortium\022\034\022\022\n\020SampleConsortium\032\006Admins\032-\n\031BlockDataHashingStructure\022\020\022\006\010\377\377\377\377\017\032\006Admins\032I\n\020OrdererAddresses\0225\022\032\n\030orderer.example.com:7050\032\027/Channel/Orderer/Admins\032$\n\014Capabilities\022\024\022\n\n\010\n\004V1_3\022\000\032\006Admins\"\"\n\007Writers\022\027\022\r\010\003\022\t\n\007Writers\032\006Admins\"\"\n\006Admins\022\030\022\016\010\003\022\n\n\006Admins\020\002\032\006Admins\"\"\n\007Readers\022\027\022\r\010\003\022\t\n\007Readers\032\006Admins*\006Admins\022\233\021\n\317\020\n\355\006\n\025\010\002\032\006\010\223\240\247\215\006\"\tmychannel\022\323\006\n\266\006\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAdCgAwIBAgIQdoMtXcRmr8XdkhTXGAwFzDAKBggqhkjOPQQDAjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEcMBoGA1UEAxMTY2Eu\nb3JnMS5leGFtcGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUwMDBa\nMGwxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMQ8wDQYDVQQLEwZjbGllbnQxHzAdBgNVBAMMFkFkbWluQG9y\nZzEuZXhhbXBsZS5jb20wWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAR4lTOcEwH/\n7BA0PJHXxXCQWTvUHqLrw/m1noiFjGZa7wQWZ1NQ+AbOJ8tydWS3xu5YVw3KegfZ\naXcQ2VMAAYsIo00wSzAOBgNVHQ8BAf8EBAMCB4AwDAYDVR0TAQH/BAIwADArBgNV\nHSMEJDAigCBcpldgEIiaAL2L5u/dXT77TRUFw4c4xvgl33Q0ZMUW5jAKBggqhkjO\nPQQDAgNHADBEAiBKZAEOoM00vmTIR0pP7XFBp41hvVL4PGwISqi0v4AwTgIgUB5T\n6gaVVogvaUeMmCqljNGNupyBj1iY0jxDlvGteNA=\n-----END CERTIFICATE-----\n\022\030R\264\212\363\023O\321\3005H\010\367\305\231\273\265\256\341\244\303\341'!Y\022\334\t\n\270\002\n\tmychannel\022;\022)\n\013Application\022\032\022\013\n\007Org1MSP\022\000\022\013\n\007Org2MSP\022\000\032\016\n\nConsortium\022\000\032\355\001\022\306\001\n\013Application\022\266\001\010\001\022\013\n\007Org1MSP\022\000\022\013\n\007Org2MSP\022\000\032$\n\014Capabilities\022\024\022\n\n\010\n\004V1_3\022\000\032\006Admins\"\"\n\007Writers\022\027\022\r\010\003\022\t\n\007Writers\032\006Admins\"\"\n\006Admins\022\030\022\016\010\003\022\n\n\006Admins\020\002\032\006Admins\"\"\n\007Readers\022\027\022\r\010\003\022\t\n\007Readers\032\006Admins*\006Admins\032\"\n\nConsortium\022\024\022\022\n\020SampleConsortium\022\236\007\n\323\006\n\266\006\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAdCgAwIBAgIQdoMtXcRmr8XdkhTXGAwFzDAKBggqhkjOPQQDAjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEcMBoGA1UEAxMTY2Eu\nb3JnMS5leGFtcGxlLmNvbTAeFw0yMTEyMDIwNzUwMDBaFw0zMTExMzAwNzUwMDBa\nMGwxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMQ8wDQYDVQQLEwZjbGllbnQxHzAdBgNVBAMMFkFkbWluQG9y\nZzEuZXhhbXBsZS5jb20wWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAR4lTOcEwH/\n7BA0PJHXxXCQWTvUHqLrw/m1noiFjGZa7wQWZ1NQ+AbOJ8tydWS3xu5YVw3KegfZ\naXcQ2VMAAYsIo00wSzAOBgNVHQ8BAf8EBAMCB4AwDAYDVR0TAQH/BAIwADArBgNV\nHSMEJDAigCBcpldgEIiaAL2L5u/dXT77TRUFw4c4xvgl33Q0ZMUW5jAKBggqhkjO\nPQQDAgNHADBEAiBKZAEOoM00vmTIR0pP7XFBp41hvVL4PGwISqi0v4AwTgIgUB5T\n6gaVVogvaUeMmCqljNGNupyBj1iY0jxDlvGteNA=\n-----END CERTIFICATE-----\n\022\0306\314<\214\244\336\220s\036{x\357\365\242\357B\014y>\2544\373\222\010\022F0D\002 \017*\035\004<%\017\024\270Y\314\207\263X\242<\"O\250\314V\253\343\240\t8\211ib\203\010\222\002 {\330JuM\336\246\334\266g\361WQlek\375\271+\273\374\376\327\215\364\347\367\n\375T4\222\022G0E\002!\000\2725\364\224\201P\356\242WD\331'.>3\004\326{\276^\014\217\362{\017w>Qr(K\230\002 %\271w\356c'e\214\021\001+\362\215\032u\371\013w\031\213?\213]\030cy\025=\271\266\214c\022G0E\002!\000\205k\337\261\255\336\272\267k3\351\334Pv\215p\236\321\254?\004\303\031_\215\234,S\252%B\207\002 -X0\212\266\003\376\330\201\354\306\306\377o\276\3453'\275\254C\210\354bYR\016o\n\366u\341" signature:"0D\002 d\000O*\337jNc\357\221V\220^.i\254-\356\311\315\351\256I\263\312\264\367\342g\303#T\002 h\331V\204E\214\365\241?OF\245\303\363pO\256\002\363\033\n4\027{\260\264\324\235\006\031O]"

		*/
		data.Data[i], err = proto.Marshal(msg)
		if err != nil {
			logger.Panicf("Could not marshal envelope: %s", err)
		}
	}

	block := cb.NewBlock(bw.lastBlock.Header.Number+1, previousBlockHash)
	block.Header.DataHash = data.Hash()
	block.Data = data

	return block
}

// WriteConfigBlock should be invoked for blocks which contain a config transaction.
// This call will block until the new config has taken effect, then will return
// while the block is written asynchronously to disk.
func (bw *BlockWriter) WriteConfigBlock(block *cb.Block, encodedMetadataValue []byte) {
	logger.Info("==BlockWriter=WriteConfigBlock===")
	ctx, err := utils.ExtractEnvelope(block, 0)
	if err != nil {
		logger.Panicf("Told to write a config block, but could not get configtx: %s", err)
	}

	payload, err := utils.UnmarshalPayload(ctx.Payload)
	if err != nil {
		logger.Panicf("Told to write a config block, but configtx payload is invalid: %s", err)
	}

	if payload.Header == nil {
		logger.Panicf("Told to write a config block, but configtx payload header is missing")
	}

	chdr, err := utils.UnmarshalChannelHeader(payload.Header.ChannelHeader)
	if err != nil {
		logger.Panicf("Told to write a config block with an invalid channel header: %s", err)
	}

	switch chdr.Type {
	case int32(cb.HeaderType_ORDERER_TRANSACTION):
		//logger.Info("==========case int32(cb.HeaderType_ORDERER_TRANSACTION)====================")
		/*
		create Channel
		 */
		newChannelConfig, err := utils.UnmarshalEnvelope(payload.Data)
		if err != nil {
			logger.Panicf("Told to write a config block with new channel, but did not have config update embedded: %s", err)
		}
		bw.registrar.newChain(newChannelConfig)
	case int32(cb.HeaderType_CONFIG):
		logger.Info("=========case int32(cb.HeaderType_CONFIG)====================")
		configEnvelope, err := configtx.UnmarshalConfigEnvelope(payload.Data)
		if err != nil {
			logger.Panicf("Told to write a config block with new channel, but did not have config envelope encoded: %s", err)
		}

		err = bw.support.Validate(configEnvelope)
		if err != nil {
			logger.Panicf("Told to write a config block with new config, but could not apply it: %s", err)
		}

		bundle, err := bw.support.CreateBundle(chdr.ChannelId, configEnvelope.Config)
		if err != nil {
			logger.Panicf("Told to write a config block with a new config, but could not convert it to a bundle: %s", err)
		}

		bw.support.Update(bundle)
	default:
		logger.Panicf("Told to write a config block with unknown header type: %v", chdr.Type)
	}

	bw.WriteBlock(block, encodedMetadataValue)
}

// WriteBlock should be invoked for blocks which contain normal transactions.
// It sets the target block as the pending next block, and returns before it is committed.
// Before returning, it acquires the committing lock, and spawns a go routine which will
// annotate the block with metadata and signatures, and write the block to the ledger
// then release the lock.  This allows the calling thread to begin assembling the next block
// before the commit phase is complete.
func (bw *BlockWriter) WriteBlock(block *cb.Block, encodedMetadataValue []byte) {
	logger.Info("==BlockWriter=WriteBlock===")
	bw.committingBlock.Lock()
	bw.lastBlock = block

	go func() {
		defer bw.committingBlock.Unlock()
		bw.commitBlock(encodedMetadataValue)
	}()
}

// commitBlock should only ever be invoked with the bw.committingBlock held
// this ensures that the encoded config sequence numbers stay in sync
func (bw *BlockWriter) commitBlock(encodedMetadataValue []byte) {
	logger.Info("==BlockWriter=commitBlock===")
	// Set the orderer-related metadata field
	if encodedMetadataValue != nil {
		bw.lastBlock.Metadata.Metadata[cb.BlockMetadataIndex_ORDERER] = utils.MarshalOrPanic(&cb.Metadata{Value: encodedMetadataValue})
	}
	bw.addBlockSignature(bw.lastBlock)
	bw.addLastConfigSignature(bw.lastBlock)

	err := bw.support.Append(bw.lastBlock)
	if err != nil {
		logger.Panicf("[channel: %s] Could not append block: %s", bw.support.ChainID(), err)
	}
	logger.Debugf("[channel: %s] Wrote block %d", bw.support.ChainID(), bw.lastBlock.GetHeader().Number)
}

func (bw *BlockWriter) addBlockSignature(block *cb.Block) {
	logger.Info("==BlockWriter=addBlockSignature===")
	blockSignature := &cb.MetadataSignature{
		SignatureHeader: utils.MarshalOrPanic(utils.NewSignatureHeaderOrPanic(bw.support)),
	}

	// Note, this value is intentionally nil, as this metadata is only about the signature, there is no additional metadata
	// information required beyond the fact that the metadata item is signed.
	blockSignatureValue := []byte(nil)

	blockSignature.Signature = utils.SignOrPanic(bw.support, util.ConcatenateBytes(blockSignatureValue, blockSignature.SignatureHeader, block.Header.Bytes()))

	block.Metadata.Metadata[cb.BlockMetadataIndex_SIGNATURES] = utils.MarshalOrPanic(&cb.Metadata{
		Value: blockSignatureValue,
		Signatures: []*cb.MetadataSignature{
			blockSignature,
		},
	})
}

func (bw *BlockWriter) addLastConfigSignature(block *cb.Block) {
	logger.Info("==BlockWriter=addLastConfigSignature===")
	configSeq := bw.support.Sequence()
	if configSeq > bw.lastConfigSeq {
		logger.Debugf("[channel: %s] Detected lastConfigSeq transitioning from %d to %d, setting lastConfigBlockNum from %d to %d", bw.support.ChainID(), bw.lastConfigSeq, configSeq, bw.lastConfigBlockNum, block.Header.Number)
		bw.lastConfigBlockNum = block.Header.Number
		bw.lastConfigSeq = configSeq
	}

	lastConfigSignature := &cb.MetadataSignature{
		SignatureHeader: utils.MarshalOrPanic(utils.NewSignatureHeaderOrPanic(bw.support)),
	}

	lastConfigValue := utils.MarshalOrPanic(&cb.LastConfig{Index: bw.lastConfigBlockNum})
	logger.Debugf("[channel: %s] About to write block, setting its LAST_CONFIG to %d", bw.support.ChainID(), bw.lastConfigBlockNum)

	lastConfigSignature.Signature = utils.SignOrPanic(bw.support, util.ConcatenateBytes(lastConfigValue, lastConfigSignature.SignatureHeader, block.Header.Bytes()))

	block.Metadata.Metadata[cb.BlockMetadataIndex_LAST_CONFIG] = utils.MarshalOrPanic(&cb.Metadata{
		Value: lastConfigValue,
		Signatures: []*cb.MetadataSignature{
			lastConfigSignature,
		},
	})
}
