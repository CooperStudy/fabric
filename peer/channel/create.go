/*
Copyright IBM Corp. 2017 All Rights Reserved.

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

package channel

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/configtx"
	localsigner "github.com/hyperledger/fabric/common/localmsp"
	"github.com/hyperledger/fabric/common/tools/configtxgen/encoder"
	genesisconfig "github.com/hyperledger/fabric/common/tools/configtxgen/localconfig"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/peer/common"
	cb "github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

//ConfigTxFileNotFound channel create configuration tx file not found
type ConfigTxFileNotFound string

func (e ConfigTxFileNotFound) Error() string {
	logger.Info("==peer channel create.go===Error:start=========")
	defer func() {
		logger.Info("==peer channel create.go===Error:end=========")
	}()
	return fmt.Sprintf("channel create configuration tx file not found %s", string(e))
}

//InvalidCreateTx invalid channel create transaction
type InvalidCreateTx string

func (e InvalidCreateTx) Error() string {
	return fmt.Sprintf("Invalid channel create transaction : %s", string(e))
}

func createCmd(cf *ChannelCmdFactory) *cobra.Command {
	logger.Info("==peer channel create.go===createCmd:start=========")
	defer func() {
		logger.Info("==peer channel create.go===createCmd:end=========")
	}()

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a channel",
		Long:  "Create a channel and write the genesis block to a file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return create(cmd, args, cf)
		},
	}
	flagList := []string{
		"channelID",
		"file",
		"outputBlock",
		"timeout",
	}
	attachFlags(createCmd, flagList)

	return createCmd
}

func createChannelFromDefaults(cf *ChannelCmdFactory) (*cb.Envelope, error) {
	logger.Info("==peer channel create.go===createChannelFromDefaults:start=========")
	defer func() {
		logger.Info("==peer channel create.go===createChannelFromDefaults:end=========")
	}()

	chCrtEnv, err := encoder.MakeChannelCreationTransaction(channelID, localsigner.NewSigner(), genesisconfig.Load(genesisconfig.SampleSingleMSPChannelProfile))
	if err != nil {
		return nil, err
	}

	return chCrtEnv, nil
}

func createChannelFromConfigTx(configTxFileName string) (*cb.Envelope, error) {
	logger.Info("==peer channel create.go===createChannelFromConfigTx:start=========")
	defer func() {
		logger.Info("==peer channel create.go===createChannelFromConfigTx:end=========")
	}()
	//cftx, err := ioutil.ReadFile(configTxFileName)
	//if err != nil {
	//	return nil, ConfigTxFileNotFound(err.Error())
	//}
    cftx := []byte{10,175,2,10,16,10,14,8,2,26,6,8,166,238,204,139,6,34,2,99,49,18,154,2,10,151,2,10,2,99,49,18,46,18,28,10,11,65,112,112,108,105,99,97,116,105,111,110,18,13,18,11,10,7,79,114,103,49,77,83,80,18,0,26,14,10,10,67,111,110,115,111,114,116,105,117,109,18,0,26,224,1,18,185,1,10,11,65,112,112,108,105,99,97,116,105,111,110,18,169,1,8,1,18,11,10,7,79,114,103,49,77,83,80,18,0,26,36,10,12,67,97,112,97,98,105,108,105,116,105,101,115,18,20,18,10,10,8,10,4,86,49,95,51,18,0,26,6,65,100,109,105,110,115,34,34,10,7,87,114,105,116,101,114,115,18,23,18,13,8,3,18,9,10,7,87,114,105,116,101,114,115,26,6,65,100,109,105,110,115,34,34,10,6,65,100,109,105,110,115,18,24,18,14,8,3,18,10,10,6,65,100,109,105,110,115,16,2,26,6,65,100,109,105,110,115,34,34,10,7,82,101,97,100,101,114,115,18,23,18,13,8,3,18,9,10,7,82,101,97,100,101,114,115,26,6,65,100,109,105,110,115,42,6,65,100,109,105,110,115,26,34,10,10,67,111,110,115,111,114,116,105,117,109,18,20,18,18,10,16,83,97,109,112,108,101,67,111,110,115,111,114,116,105,117,109}
	//cftx := []byte{10,175,2,10,16,10,14,8,2,26,6,8,250,215,204,139,6,34,2,99,49,18,154,2,10,151,2,10,2,99,49,18,46,18,28,10,11,65,112,112,108,105,99,97,116,105,111,110,18,13,18,11,10,7,79,114,103,49,77,83,80,18,0,26,14,10,10,67,111,110,115,111,114,116,105,117,109,18,0,26,224,1,18,185,1,10,11,65,112,112,108,105,99,97,116,105,111,110,18,169,1,8,1,18,11,10,7,79,114,103,49,77,83,80,18,0,26,36,10,12,67,97,112,97,98,105,108,105,116,105,101,115,18,20,18,10,10,8,10,4,86,49,95,51,18,0,26,6,65,100,109,105,110,115,34,34,10,7,82,101,97,100,101,114,115,18,23,18,13,8,3,18,9,10,7,82,101,97,100,101,114,115,26,6,65,100,109,105,110,115,34,34,10,7,87,114,105,116,101,114,115,18,23,18,13,8,3,18,9,10,7,87,114,105,116,101,114,115,26,6,65,100,109,105,110,115,34,34,10,6,65,100,109,105,110,115,18,24,18,14,8,3,18,10,10,6,65,100,109,105,110,115,16,2,26,6,65,100,109,105,110,115,42,6,65,100,109,105,110,115,26,34,10,10,67,111,110,115,111,114,116,105,117,109,18,20,18,18,10,16,83,97,109,112,108,101,67,111,110,115,111,114,116,105,117,109}

	logger.Info("========cftx=========",string(cftx))
	env,err := utils.UnmarshalEnvelope(cftx)
	if env != nil{
		logger.Info("====env.Payload=======",env.Payload)
		logger.Info("====env.Signature=======",env.Signature)
	}

	return env,err
}

func sanityCheckAndSignConfigTx(envConfigUpdate *cb.Envelope) (*cb.Envelope, error) {
	logger.Info("==peer channel create.go===sanityCheckAndSignConfigTx:start=========")
	defer func() {
		logger.Info("==peer channel create.go===sanityCheckAndSignConfigTx:end=========")
	}()
	payload, err := utils.ExtractPayload(envConfigUpdate)
	logger.Info("===payload======",*payload)
	if err != nil {
		return nil, InvalidCreateTx("bad payload")
	}
	logger.Info("===payload.Header======",*payload.Header)

	if payload.Header == nil || payload.Header.ChannelHeader == nil {
		return nil, InvalidCreateTx("bad header")
	}
	/*
	type ChannelHeader struct {
		Type int32 `protobuf:"varint,1,opt,name=type,proto3" json:"type,omitempty"`
		Version int32 `protobuf:"varint,2,opt,name=version,proto3" json:"version,omitempty"`
		Timestamp *timestamp.Timestamp `protobuf:"bytes,3,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
		ChannelId string `protobuf:"bytes,4,opt,name=channel_id,json=channelId,proto3" json:"channel_id,omitempty"`
		TxId string `protobuf:"bytes,5,opt,name=tx_id,json=txId,proto3" json:"tx_id,omitempty"`
		Epoch uint64 `protobuf:"varint,6,opt,name=epoch,proto3" json:"epoch,omitempty"`
		Extension []byte `protobuf:"bytes,7,opt,name=extension,proto3" json:"extension,omitempty"`
		TlsCertHash          []byte   `protobuf:"bytes,8,opt,name=tls_cert_hash,json=tlsCertHash,proto3" json:"tls_cert_hash,omitempty"`

	/*
	type:2
	timestamp:<seconds:1634796922 >
	channel_id:"b"



	}
	 */

	ch, err := utils.UnmarshalChannelHeader(payload.Header.ChannelHeader)
    logger.Info("======ch:======",ch)
	if err != nil {
		return nil, InvalidCreateTx("could not unmarshall channel header")
	}

	if ch.Type != int32(cb.HeaderType_CONFIG_UPDATE) {
		return nil, InvalidCreateTx("bad type")
	}

	if ch.ChannelId == "" {
		return nil, InvalidCreateTx("empty channel id")
	}

	// Specifying the chainID on the CLI is usually redundant, as a hack, set it
	// here if it has not been set explicitly
	if channelID == "" {
		channelID = ch.ChannelId
	}

	if ch.ChannelId != channelID {
		return nil, InvalidCreateTx(fmt.Sprintf("mismatched channel ID %s != %s", ch.ChannelId, channelID))
	}

	configUpdateEnv, err := configtx.UnmarshalConfigUpdateEnvelope(payload.Data)
	if err != nil {
		return nil, InvalidCreateTx("Bad config update env")
	}

	signer := localsigner.NewSigner()
	sigHeader, err := signer.NewSignatureHeader()
	if err != nil {
		return nil, err
	}

	configSig := &cb.ConfigSignature{
		SignatureHeader: utils.MarshalOrPanic(sigHeader),
	}

	configSig.Signature, err = signer.Sign(util.ConcatenateBytes(configSig.SignatureHeader, configUpdateEnv.ConfigUpdate))

	configUpdateEnv.Signatures = append(configUpdateEnv.Signatures, configSig)

	return utils.CreateSignedEnvelope(cb.HeaderType_CONFIG_UPDATE, channelID, signer, configUpdateEnv, 0, 0)
}

func sendCreateChainTransaction(cf *ChannelCmdFactory) error {
	logger.Info("======sendCreateChainTransaction:start===")
	defer func() {
		logger.Info("=====sendCreateChainTransaction:end===")
	}()

	var err error
	var chCrtEnv *cb.Envelope

    //=======channelTxFile======= ./channel-artifacts/channel.tx
	logger.Info("=======channelTxFile=======",channelTxFile)
	if channelTxFile != "" {
		if chCrtEnv, err = createChannelFromConfigTx(channelTxFile); err != nil {
			return err
		}
	} else {
		if chCrtEnv, err = createChannelFromDefaults(cf); err != nil {
			return err
		}
	}

	if chCrtEnv, err = sanityCheckAndSignConfigTx(chCrtEnv); err != nil {
		return err
	}

	var broadcastClient common.BroadcastClient
	broadcastClient, err = cf.BroadcastFactory()
	if err != nil {
		return errors.WithMessage(err, "error getting broadcast client")
	}

	defer broadcastClient.Close()
	err = broadcastClient.Send(chCrtEnv)

	return err
}

func executeCreate(cf *ChannelCmdFactory) error {
	logger.Info("==peer channel create.go===executeCreate:start=========")
	defer func() {
		logger.Info("==peer channel create.go===executeCreate:end=========")
	}()

	err := sendCreateChainTransaction(cf)
	if err != nil {
		return err
	}

	block, err := getGenesisBlock(cf)
	if err != nil {
		return err
	}

	b, err := proto.Marshal(block)
	if err != nil {
		return err
	}

	file := channelID + ".block"
	if outputBlock != common.UndefinedParamValue {
		file = outputBlock
	}
	err = ioutil.WriteFile(file, b, 0644)
	if err != nil {
		return err
	}

	return nil
}

func getGenesisBlock(cf *ChannelCmdFactory) (*cb.Block, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			cf.DeliverClient.Close()
			return nil, errors.New("timeout waiting for channel creation")
		default:
			if block, err := cf.DeliverClient.GetSpecifiedBlock(0); err != nil {
				cf.DeliverClient.Close()
				cf, err = InitCmdFactory(EndorserNotRequired, PeerDeliverNotRequired, OrdererRequired)
				if err != nil {
					return nil, errors.WithMessage(err, "failed connecting")
				}
				time.Sleep(200 * time.Millisecond)
			} else {
				cf.DeliverClient.Close()
				return block, nil
			}
		}
	}
}

func create(cmd *cobra.Command, args []string, cf *ChannelCmdFactory) error {
	fmt.Println("====fabric peer channel create.go====create:start==")
	defer func() {
		fmt.Println("====fabric peer channel create.go====create:end==")
	}()


	logger.Info("======create channel==============")
	/*

	 */







	// the global chainID filled by the "-c" command
	if channelID == common.UndefinedParamValue {
		return errors.New("must supply channel ID")
	}

	// Parsing of the command line is done so silence cmd usage
	cmd.SilenceUsage = true

	var err error
	if cf == nil {
		cf, err = InitCmdFactory(EndorserNotRequired, PeerDeliverNotRequired, OrdererRequired)
		if err != nil {
			return err
		}
	}
	return executeCreate(cf)
}
