/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package commands



type Peers struct {
	UserCert string
	UserKey  string
	MSPID    string
	Server   string
	Channel  string
}

func (p Peers) SessionName() string {
	logger.Info("=============Peers=====SessionName==============")
	return "discover-peers"
}

func (p Peers) Args() []string {
	logger.Info("=============Peers=====Args==============")
	return []string{
		"--userCert", p.UserCert,
		"--userKey", p.UserKey,
		"--MSP", p.MSPID,
		"peers",
		"--server", p.Server,
		"--channel", p.Channel,
	}
}

type Config struct {
	UserCert string
	UserKey  string
	MSPID    string
	Server   string
	Channel  string
}

func (c Config) SessionName() string {
	logger.Info("=============Config=====SessionName==============")
	return "discover-config"
}

func (c Config) Args() []string {
	logger.Info("=============Config=====Args==============")
	return []string{
		"--userCert", c.UserCert,
		"--userKey", c.UserKey,
		"--MSP", c.MSPID,
		"config",
		"--server", c.Server,
		"--channel", c.Channel,
	}
}

type Endorsers struct {
	UserCert    string
	UserKey     string
	MSPID       string
	Server      string
	Channel     string
	Chaincode   string
	Chaincodes  []string
	Collection  string
	Collections []string
}

func (e Endorsers) SessionName() string {
	logger.Info("=============Endorsers=====SessionName==============")
	return "discover-endorsers"
}

func (e Endorsers) Args() []string {
	logger.Info("=============Endorsers=====Args==============")
	args := []string{
		"--userCert", e.UserCert,
		"--userKey", e.UserKey,
		"--MSP", e.MSPID,
		"endorsers",
		"--server", e.Server,
		"--channel", e.Channel,
	}
	if e.Chaincode != "" {
		args = append(args, "--chaincode", e.Chaincode)
	}
	for _, cc := range e.Chaincodes {
		args = append(args, "--chaincode", cc)
	}
	if e.Collection != "" {
		args = append(args, "--collection", e.Collection)
	}
	for _, c := range e.Collections {
		args = append(args, "--collection", c)
	}
	return args
}
