/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package commands

import "fmt"

type NodeStart struct {
	PeerID string
	Dir    string
}

func (n NodeStart) SessionName() string {
	return n.PeerID
}

func (n NodeStart) WorkingDir() string {
	return n.Dir
}

func (n NodeStart) Args() []string {
	return []string{
		"node", "start",
	}
}

type ChannelCreate struct {
	ChannelID   string
	Orderer     string
	File        string
	OutputBlock string
}

func (c ChannelCreate) SessionName() string {
	logger.Info("=============ChannelCreate=====SessionName==============")
	return "peer-channel-create"
}

func (c ChannelCreate) Args() []string {
	logger.Info("=============ChannelCreate=====Args==============")
	return []string{
		"channel", "create",
		"--channelID", c.ChannelID,
		"--orderer", c.Orderer,
		"--file", c.File,
		"--outputBlock", c.OutputBlock,
	}
}

type ChannelJoin struct {
	BlockPath string
}

func (c ChannelJoin) SessionName() string {
	logger.Info("=============ChannelJoin=====SessionName==============")
	return "peer-channel-join"
}

func (c ChannelJoin) Args() []string {
	logger.Info("=============ChannelJoin==============")
	return []string{
		"channel", "join",
		"-b", c.BlockPath,
	}
}

type ChannelFetch struct {
	ChannelID  string
	Block      string
	Orderer    string
	OutputFile string
}

func (c ChannelFetch) SessionName() string {
	logger.Info("=====ChannelFetch========SessionName==============")
	return "peer-channel-fetch"
}

func (c ChannelFetch) Args() []string {
	logger.Info("=====ChannelFetch========Args==============")
	args := []string{
		"channel", "fetch", c.Block,
	}
	if c.ChannelID != "" {
		args = append(args, "--channelID", c.ChannelID)
	}
	if c.Orderer != "" {
		args = append(args, "--orderer", c.Orderer)
	}
	if c.OutputFile != "" {
		args = append(args, c.OutputFile)
	}
	return args
}

type ChaincodePackage struct {
	Name       string
	Version    string
	Path       string
	Lang       string
	OutputFile string
}

func (c ChaincodePackage) SessionName() string {
	logger.Info("=====ChaincodePackage========SessionName==============")
	return "peer-chaincode-package"
}

func (c ChaincodePackage) Args() []string {
	logger.Info("=====ChaincodePackage========Args==============")
	args := []string{
		"chaincode", "package",
		"--name", c.Name,
		"--version", c.Version,
		"--path", c.Path,
		c.OutputFile,
	}

	if c.Lang != "" {
		args = append(args, "--lang", c.Lang)
	}

	return args
}

type ChaincodeInstall struct {
	Name        string
	Version     string
	Path        string
	Lang        string
	PackageFile string
}

func (c ChaincodeInstall) SessionName() string {
	logger.Info("=====ChaincodeInstall========SessionName==============")
	return "peer-chaincode-install"
}

func (c ChaincodeInstall) Args() []string {
	logger.Info("=====ChaincodeInstall========Args==============")
	args := []string{
		"chaincode", "install",
	}

	if c.PackageFile != "" {
		args = append(args, c.PackageFile)
	}
	if c.Name != "" {
		args = append(args, "--name", c.Name)
	}
	if c.Version != "" {
		args = append(args, "--version", c.Version)
	}
	if c.Path != "" {
		args = append(args, "--path", c.Path)
	}
	if c.Lang != "" {
		args = append(args, "--lang", c.Lang)
	}

	return args
}

type ChaincodeInstantiate struct {
	ChannelID         string
	Orderer           string
	Name              string
	Version           string
	Ctor              string
	Policy            string
	Lang              string
	CollectionsConfig string
}

func (c ChaincodeInstantiate) SessionName() string {
	logger.Info("=====ChaincodeInstantiate========SessionName==============")
	return "peer-chaincode-instantiate"
}

func (c ChaincodeInstantiate) Args() []string {
	logger.Info("=====ChaincodeInstantiate========Args==============")
	args := []string{
		"chaincode", "instantiate",
		"--channelID", c.ChannelID,
		"--orderer", c.Orderer,
		"--name", c.Name,
		"--version", c.Version,
		"--ctor", c.Ctor,
		"--policy", c.Policy,
	}
	if c.CollectionsConfig != "" {
		args = append(args, "--collections-config", c.CollectionsConfig)
	}

	if c.Lang != "" {
		args = append(args, "--lang", c.Lang)
	}

	return args
}

type ChaincodeListInstalled struct{}

func (c ChaincodeListInstalled) SessionName() string {
	logger.Info("=====ChaincodeListInstalled========SessionName==============")
	return "peer-chaincode-list-installed"
}

func (c ChaincodeListInstalled) Args() []string {
	logger.Info("=====ChaincodeListInstalled========Args==============")
	return []string{
		"chaincode", "list", "--installed",
	}
}

type ChaincodeListInstantiated struct {
	ChannelID string
}

func (c ChaincodeListInstantiated) SessionName() string {
	logger.Info("=====ChaincodeListInstantiated========SessionName==============")
	return "peer-chaincode-list-instantiated"
}

func (c ChaincodeListInstantiated) Args() []string {
	logger.Info("=====ChaincodeListInstantiated========Args==============")
	return []string{
		"chaincode", "list", "--instantiated",
		"--channelID", c.ChannelID,
	}
}

type ChaincodeQuery struct {
	ChannelID string
	Name      string
	Ctor      string
}

func (c ChaincodeQuery) SessionName() string {
	logger.Info("=====ChaincodeQuery========SessionName==============")
	return "peer-chaincode-query"
}

func (c ChaincodeQuery) Args() []string {
	logger.Info("=====ChaincodeQuery========Args==============")
	return []string{
		"chaincode", "query",
		"--channelID", c.ChannelID,
		"--name", c.Name,
		"--ctor", c.Ctor,
	}
}

type ChaincodeInvoke struct {
	ChannelID     string
	Orderer       string
	Name          string
	Ctor          string
	PeerAddresses []string
	WaitForEvent  bool
}

func (c ChaincodeInvoke) SessionName() string {
	logger.Info("=====ChaincodeInvoke========SessionName==============")
	return "peer-chaincode-invoke"
}

func (c ChaincodeInvoke) Args() []string {
	logger.Info("=====ChaincodeInvoke========Args==============")
	args := []string{
		"chaincode", "invoke",
		"--channelID", c.ChannelID,
		"--orderer", c.Orderer,
		"--name", c.Name,
		"--ctor", c.Ctor,
	}
	for _, p := range c.PeerAddresses {
		args = append(args, "--peerAddresses", p)
	}
	if c.WaitForEvent {
		args = append(args, "--waitForEvent")
	}
	return args
}

type ChaincodeUpgrade struct {
	Name              string
	Version           string
	Path              string // optional
	ChannelID         string
	Orderer           string
	Ctor              string
	Policy            string
	CollectionsConfig string // optional
}

func (c ChaincodeUpgrade) SessionName() string {
	logger.Info("=====ChaincodeUpgrade========SessionName==============")
	return "peer-chaincode-upgrade"
}

func (c ChaincodeUpgrade) Args() []string {
	logger.Info("=====ChaincodeUpgrade========Args==============")
	args := []string{
		"chaincode", "upgrade",
		"--name", c.Name,
		"--version", c.Version,
		"--channelID", c.ChannelID,
		"--orderer", c.Orderer,
		"--ctor", c.Ctor,
		"--policy", c.Policy,
	}
	if c.Path != "" {
		args = append(args, "--path", c.Path)
	}
	if c.CollectionsConfig != "" {
		args = append(args, "--collections-config", c.CollectionsConfig)
	}
	return args
}

type SignConfigTx struct {
	File string
}

func (s SignConfigTx) SessionName() string {
	logger.Info("=====SignConfigTx========SessionName==============")
	return "peer-channel-signconfigtx"
}

func (s SignConfigTx) Args() []string {
	logger.Info("=====SignConfigTx========Args==============")
	return []string{
		"channel", "signconfigtx",
		"--file", s.File,
	}
}

type ChannelUpdate struct {
	ChannelID string
	Orderer   string
	File      string
}

func (c ChannelUpdate) SessionName() string {
	logger.Info("=====ChannelUpdate========SessionName==============")
	return "peer-channel-update"
}

func (c ChannelUpdate) Args() []string {
	logger.Info("=====ChannelUpdate========Args==============")
	return []string{
		"channel", "update",
		"--channelID", c.ChannelID,
		"--orderer", c.Orderer,
		"--file", c.File,
	}
}

type ChannelInfo struct {
	ChannelID string
}

func (c ChannelInfo) SessionName() string {
	logger.Info("=====ChannelInfo========SessionName==============")
	return "peer-channel-info"
}

func (c ChannelInfo) Args() []string {
	logger.Info("=====ChannelInfo========Args==============")
	return []string{
		"channel", "getinfo",
		"-c", c.ChannelID,
	}
}

type LoggingSetLevel struct {
	Logger string
	Level  string
}

func (l LoggingSetLevel) SessionName() string {
	logger.Info("=====LoggingSetLevel========SessionName==============")
	return "peer-logging-setlevel"
}

func (l LoggingSetLevel) Args() []string {
	logger.Info("=====LoggingSetLevel========Args==============")
	return []string{
		"logging", "setlevel", l.Logger, l.Level,
	}
}
