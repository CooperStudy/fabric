/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package commands

import "github.com/hyperledger/fabric/common/flogging"

type OutputBlock struct {
	ChannelID   string
	Profile     string
	ConfigPath  string
	OutputBlock string
}
var logger = flogging.MustGetLogger("integration.nwo.commands")
func (o OutputBlock) SessionName() string {
	logger.Info("=============OutputBlock=====SessionName==============")
	return "configtxgen-output-block"
}

func (o OutputBlock) Args() []string {
	logger.Info("=============OutputBlock=====Args==============")
	return []string{
		"-channelID", o.ChannelID,
		"-profile", o.Profile,
		"-configPath", o.ConfigPath,
		"-outputBlock", o.OutputBlock,
	}
}

type CreateChannelTx struct {
	ChannelID             string
	Profile               string
	ConfigPath            string
	OutputCreateChannelTx string
}

func (c CreateChannelTx) SessionName() string {
	logger.Info("=============CreateChannelTx=====SessionName==============")
	return "configtxgen-create-channel-tx"
}

func (c CreateChannelTx) Args() []string {
	logger.Info("=============CreateChannelTx=====Args==============")
	return []string{
		"-channelID", c.ChannelID,
		"-profile", c.Profile,
		"-configPath", c.ConfigPath,
		"-outputCreateChannelTx", c.OutputCreateChannelTx,
	}
}

type OutputAnchorPeersUpdate struct {
	ChannelID               string
	Profile                 string
	ConfigPath              string
	AsOrg                   string
	OutputAnchorPeersUpdate string
}

func (o OutputAnchorPeersUpdate) SessionName() string {
	logger.Info("=============OutputAnchorPeersUpdate=====SessionName==============")
	return "configtxgen-output-anchor-peers-update"
}

func (o OutputAnchorPeersUpdate) Args() []string {
	logger.Info("=============OutputAnchorPeersUpdate=====Args==============")
	return []string{
		"-channelID", o.ChannelID,
		"-profile", o.Profile,
		"-configPath", o.ConfigPath,
		"-asOrg", o.AsOrg,
		"-outputAnchorPeersUpdate", o.OutputAnchorPeersUpdate,
	}
}
