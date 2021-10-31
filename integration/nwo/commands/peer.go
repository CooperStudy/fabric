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
	fmt.Println("=============ChannelCreate=====SessionName==============")
	return "peer-channel-create"
}

func (c ChannelCreate) Args() []string {
	fmt.Println("=============ChannelCreate=====Args==============")
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
	fmt.Println("=============ChannelJoin=====SessionName==============")
	return "peer-channel-join"
}

func (c ChannelJoin) Args() []string {
	fmt.Println("=============ChannelJoin==============")
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
	fmt.Println("=====ChannelFetch========SessionName==============")
	return "peer-channel-fetch"
}

func (c ChannelFetch) Args() []string {
	fmt.Println("=====ChannelFetch========Args==============")
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
	fmt.Println("=====ChaincodePackage========SessionName==============")
	return "peer-chaincode-package"
}

func (c ChaincodePackage) Args() []string {
	fmt.Println("=====ChaincodePackage========Args==============")
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
	fmt.Println("=====ChaincodeInstall========SessionName==============")
	return "peer-chaincode-install"
}

func (c ChaincodeInstall) Args() []string {
	fmt.Println("=====ChaincodeInstall========Args==============")
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
	fmt.Println("=====ChaincodeInstantiate========SessionName==============")
	return "peer-chaincode-instantiate"
}

func (c ChaincodeInstantiate) Args() []string {
	fmt.Println("=====ChaincodeInstantiate========Args==============")
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
	fmt.Println("=====ChaincodeListInstalled========SessionName==============")
	return "peer-chaincode-list-installed"
}

func (c ChaincodeListInstalled) Args() []string {
	fmt.Println("=====ChaincodeListInstalled========Args==============")
	return []string{
		"chaincode", "list", "--installed",
	}
}

type ChaincodeListInstantiated struct {
	ChannelID string
}

func (c ChaincodeListInstantiated) SessionName() string {
	fmt.Println("=====ChaincodeListInstantiated========SessionName==============")
	return "peer-chaincode-list-instantiated"
}

func (c ChaincodeListInstantiated) Args() []string {
	fmt.Println("=====ChaincodeListInstantiated========Args==============")
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
	fmt.Println("=====ChaincodeQuery========SessionName==============")
	return "peer-chaincode-query"
}

func (c ChaincodeQuery) Args() []string {
	fmt.Println("=====ChaincodeQuery========Args==============")
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
	fmt.Println("=====ChaincodeInvoke========SessionName==============")
	return "peer-chaincode-invoke"
}

func (c ChaincodeInvoke) Args() []string {
	fmt.Println("=====ChaincodeInvoke========Args==============")
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
	fmt.Println("=====ChaincodeUpgrade========SessionName==============")
	return "peer-chaincode-upgrade"
}

func (c ChaincodeUpgrade) Args() []string {
	fmt.Println("=====ChaincodeUpgrade========Args==============")
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
	fmt.Println("=====SignConfigTx========SessionName==============")
	return "peer-channel-signconfigtx"
}

func (s SignConfigTx) Args() []string {
	fmt.Println("=====SignConfigTx========Args==============")
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
	fmt.Println("=====ChannelUpdate========SessionName==============")
	return "peer-channel-update"
}

func (c ChannelUpdate) Args() []string {
	fmt.Println("=====ChannelUpdate========Args==============")
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
	fmt.Println("=====ChannelInfo========SessionName==============")
	return "peer-channel-info"
}

func (c ChannelInfo) Args() []string {
	fmt.Println("=====ChannelInfo========Args==============")
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
	fmt.Println("=====LoggingSetLevel========SessionName==============")
	return "peer-logging-setlevel"
}

func (l LoggingSetLevel) Args() []string {
	fmt.Println("=====LoggingSetLevel========Args==============")
	return []string{
		"logging", "setlevel", l.Logger, l.Level,
	}
}
