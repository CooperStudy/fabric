/*
Copyright IBM Corp. 2017 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/hyperledger/fabric/bccsp/factory"
	"github.com/hyperledger/fabric/common/channelconfig"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/common/tools/configtxgen/encoder"
	genesisconfig "github.com/hyperledger/fabric/common/tools/configtxgen/localconfig"
	"github.com/hyperledger/fabric/common/tools/configtxgen/metadata"
	"github.com/hyperledger/fabric/common/tools/protolator"
	cb "github.com/hyperledger/fabric/protos/common"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/hyperledger/fabric/protos/utils"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"strings"
)

var exitCode = 0

var logger = flogging.MustGetLogger("common.tools.configtxgen")

func doOutputBlock(config *genesisconfig.Profile, channelID string, outputBlock string) error {
	pgen := encoder.New(config)
	logger.Info("Generating genesis block")
	if config.Consortiums == nil {
		logger.Warning("Genesis block does not contain a consortiums group definition.  This block cannot be used for orderer bootstrap.")
	}
	genesisBlock := pgen.GenesisBlockForChannel(channelID)
	logger.Info("Writing genesis block")
	err := ioutil.WriteFile(outputBlock, utils.MarshalOrPanic(genesisBlock), 0644)
	if err != nil {
		return fmt.Errorf("Error writing genesis block: %s", err)
	}
	return nil
}

func doOutputChannelCreateTx(conf *genesisconfig.Profile, channelID string, outputChannelCreateTx string) error {
	logger.Info("============doOutputChannelCreateTx:start=====")
/**
    Consortium   string                 `yaml:"Consortium"`
  	Application  *Application           `yaml:"Application"`
  	Orderer      *Orderer               `yaml:"Orderer"`
  	Consortiums  map[string]*Consortium `yaml:"Consortiums"`
  	Capabilities map[string]bool        `yaml:"Capabilities"`
  	Policies     map[string]*Policy     `yaml:"Policies"`
 */
    fmt.Println("====conf====",conf)
	fmt.Println("===conf.Consortium==",conf.Consortium)//SampleConsortium
	if conf.Application != nil{
		fmt.Println("===conf.Application==",*conf.Application)
	}else{
		fmt.Println("===conf.Application",conf.Application)
	}
	fmt.Println("===conf.Application.Policies==",conf.Application.Policies)
	for k,v := range conf.Application.Policies {
		fmt.Println(k)
		fmt.Println(v)
		if v != nil{
			fmt.Println(*v)
		}else{
			fmt.Println(v)
		}
	}
	fmt.Println("===conf.Application.Capabilities==",conf.Application.Capabilities)
	fmt.Println("===conf.Application.Organizations==",conf.Application.Organizations)
    for k,v := range conf.Application.Organizations {
    	fmt.Println(k)
    	fmt.Println(v)
    	if v != nil{
    		fmt.Println(*v)
		}else{
			fmt.Println(v)
		}
	}
	fmt.Println("===conf.Application.ACLS==",conf.Application.ACLs)
    if conf.Application.Resources != nil{
		fmt.Println("===conf.Application.Resources==",*conf.Application.Resources)
	}else{
		fmt.Println("===conf.Application.Resources==",conf.Application.Resources)
	}

	// {[0xc00019e0e0 0xc00019e150] map[V1_2:false V1_1:false V1_3:true] <nil> map[Writers:0xc0004c8420 Admins:0xc0004c84a0 Readers:0xc0004c8520] map[]}
	if conf.Orderer != nil{
		fmt.Println("===conf.Orderer==",*conf.Orderer)
	}else{
		//===conf.Orderer== <nil>
		fmt.Println("===conf.Orderer==",conf.Orderer)
	}
	fmt.Println("===conf.Consortiums==",conf.Consortiums)
    //===conf.Consortiums== map[]
	fmt.Println("===conf.Capabilities==",conf.Capabilities)
    //===conf.Capabilities== map[]
	fmt.Println("===conf.Policies==",conf.Policies)
    //===conf.Policies== map[]



	fmt.Println("=====outputChannelCreateTx=",outputChannelCreateTx)

	logger.Info("Generating new channel configtx")
	defer func() {
		logger.Info("============doOutputChannelCreateTx:end=====")
	}()
	configtx, err := encoder.MakeChannelCreationTransaction(channelID, nil, conf)
	if err != nil {
		return err
	}

	logger.Info("Writing new channel tx")
	err = ioutil.WriteFile(outputChannelCreateTx, utils.MarshalOrPanic(configtx), 0644)
	if err != nil {
		return fmt.Errorf("Error writing channel create tx: %s", err)
	}
	return nil
}

func doOutputAnchorPeersUpdate(conf *genesisconfig.Profile, channelID string, outputAnchorPeersUpdate string, asOrg string) error {
	logger.Info("Generating anchor peer update")
	if asOrg == "" {
		return fmt.Errorf("Must specify an organization to update the anchor peer for")
	}

	if conf.Application == nil {
		return fmt.Errorf("Cannot update anchor peers without an application section")
	}

	var org *genesisconfig.Organization
	for _, iorg := range conf.Application.Organizations {
		if iorg.Name == asOrg {
			org = iorg
		}
	}

	if org == nil {
		return fmt.Errorf("No organization name matching: %s", asOrg)
	}

	anchorPeers := make([]*pb.AnchorPeer, len(org.AnchorPeers))
	for i, anchorPeer := range org.AnchorPeers {
		anchorPeers[i] = &pb.AnchorPeer{
			Host: anchorPeer.Host,
			Port: int32(anchorPeer.Port),
		}
	}

	configUpdate := &cb.ConfigUpdate{
		ChannelId: channelID,
		WriteSet:  cb.NewConfigGroup(),
		ReadSet:   cb.NewConfigGroup(),
	}

	// Add all the existing config to the readset
	configUpdate.ReadSet.Groups[channelconfig.ApplicationGroupKey] = cb.NewConfigGroup()
	configUpdate.ReadSet.Groups[channelconfig.ApplicationGroupKey].Version = 1
	configUpdate.ReadSet.Groups[channelconfig.ApplicationGroupKey].ModPolicy = channelconfig.AdminsPolicyKey
	configUpdate.ReadSet.Groups[channelconfig.ApplicationGroupKey].Groups[org.Name] = cb.NewConfigGroup()
	configUpdate.ReadSet.Groups[channelconfig.ApplicationGroupKey].Groups[org.Name].Values[channelconfig.MSPKey] = &cb.ConfigValue{}
	configUpdate.ReadSet.Groups[channelconfig.ApplicationGroupKey].Groups[org.Name].Policies[channelconfig.ReadersPolicyKey] = &cb.ConfigPolicy{}
	configUpdate.ReadSet.Groups[channelconfig.ApplicationGroupKey].Groups[org.Name].Policies[channelconfig.WritersPolicyKey] = &cb.ConfigPolicy{}
	configUpdate.ReadSet.Groups[channelconfig.ApplicationGroupKey].Groups[org.Name].Policies[channelconfig.AdminsPolicyKey] = &cb.ConfigPolicy{}

	// Add all the existing at the same versions to the writeset
	configUpdate.WriteSet.Groups[channelconfig.ApplicationGroupKey] = cb.NewConfigGroup()
	configUpdate.WriteSet.Groups[channelconfig.ApplicationGroupKey].Version = 1
	configUpdate.WriteSet.Groups[channelconfig.ApplicationGroupKey].ModPolicy = channelconfig.AdminsPolicyKey
	configUpdate.WriteSet.Groups[channelconfig.ApplicationGroupKey].Groups[org.Name] = cb.NewConfigGroup()
	configUpdate.WriteSet.Groups[channelconfig.ApplicationGroupKey].Groups[org.Name].Version = 1
	configUpdate.WriteSet.Groups[channelconfig.ApplicationGroupKey].Groups[org.Name].ModPolicy = channelconfig.AdminsPolicyKey
	configUpdate.WriteSet.Groups[channelconfig.ApplicationGroupKey].Groups[org.Name].Values[channelconfig.MSPKey] = &cb.ConfigValue{}
	configUpdate.WriteSet.Groups[channelconfig.ApplicationGroupKey].Groups[org.Name].Policies[channelconfig.ReadersPolicyKey] = &cb.ConfigPolicy{}
	configUpdate.WriteSet.Groups[channelconfig.ApplicationGroupKey].Groups[org.Name].Policies[channelconfig.WritersPolicyKey] = &cb.ConfigPolicy{}
	configUpdate.WriteSet.Groups[channelconfig.ApplicationGroupKey].Groups[org.Name].Policies[channelconfig.AdminsPolicyKey] = &cb.ConfigPolicy{}
	configUpdate.WriteSet.Groups[channelconfig.ApplicationGroupKey].Groups[org.Name].Values[channelconfig.AnchorPeersKey] = &cb.ConfigValue{
		Value:     utils.MarshalOrPanic(channelconfig.AnchorPeersValue(anchorPeers).Value()),
		ModPolicy: channelconfig.AdminsPolicyKey,
	}

	configUpdateEnvelope := &cb.ConfigUpdateEnvelope{
		ConfigUpdate: utils.MarshalOrPanic(configUpdate),
	}

	update := &cb.Envelope{
		Payload: utils.MarshalOrPanic(&cb.Payload{
			Header: &cb.Header{
				ChannelHeader: utils.MarshalOrPanic(&cb.ChannelHeader{
					ChannelId: channelID,
					Type:      int32(cb.HeaderType_CONFIG_UPDATE),
				}),
			},
			Data: utils.MarshalOrPanic(configUpdateEnvelope),
		}),
	}

	logger.Info("Writing anchor peer update")
	err := ioutil.WriteFile(outputAnchorPeersUpdate, utils.MarshalOrPanic(update), 0644)
	if err != nil {
		return fmt.Errorf("Error writing channel anchor peer update: %s", err)
	}
	return nil
}

func doInspectBlock(inspectBlock string) error {
	logger.Info("Inspecting block")
	data, err := ioutil.ReadFile(inspectBlock)
	if err != nil {
		return fmt.Errorf("Could not read block %s", inspectBlock)
	}

	logger.Info("Parsing genesis block")
	block, err := utils.UnmarshalBlock(data)
	if err != nil {
		return fmt.Errorf("error unmarshaling to block: %s", err)
	}
	err = protolator.DeepMarshalJSON(os.Stdout, block)
	if err != nil {
		return fmt.Errorf("malformed block contents: %s", err)
	}
	return nil
}

func doInspectChannelCreateTx(inspectChannelCreateTx string) error {
	logger.Info("Inspecting transaction")
	data, err := ioutil.ReadFile(inspectChannelCreateTx)
	if err != nil {
		return fmt.Errorf("could not read channel create tx: %s", err)
	}

	logger.Info("Parsing transaction")
	env, err := utils.UnmarshalEnvelope(data)
	if err != nil {
		return fmt.Errorf("Error unmarshaling envelope: %s", err)
	}

	err = protolator.DeepMarshalJSON(os.Stdout, env)
	if err != nil {
		return fmt.Errorf("malformed transaction contents: %s", err)
	}

	return nil
}

func doPrintOrg(t *genesisconfig.TopLevel, printOrg string) error {
	for _, org := range t.Organizations {
		fmt.Println("org",org)
		if org.Name == printOrg {
			og, err := encoder.NewOrdererOrgGroup(org)
			if err != nil {
				return errors.Wrapf(err, "bad org definition for org %s", org.Name)
			}

			if err := protolator.DeepMarshalJSON(os.Stdout, &cb.DynamicConsortiumOrgGroup{ConfigGroup: og}); err != nil {
				return errors.Wrapf(err, "malformed org definition for org: %s", org.Name)
			}
			return nil
		}
	}
	return errors.Errorf("organization %s not found", printOrg)
}

func main() {
	var outputBlock, outputChannelCreateTx, profile, configPath, channelID, inspectBlock, inspectChannelCreateTx, outputAnchorPeersUpdate, asOrg, printOrg string

	flag.StringVar(&outputBlock, "outputBlock", "", "The path to write the genesis block to (if set)")
	flag.StringVar(&channelID, "channelID", "", "The channel ID to use in the configtx")
	flag.StringVar(&outputChannelCreateTx, "outputCreateChannelTx", "", "The path to write a channel creation configtx to (if set)")
	flag.StringVar(&profile, "profile", genesisconfig.SampleInsecureSoloProfile, "The profile from configtx.yaml to use for generation.")
	flag.StringVar(&configPath, "configPath", "", "The path containing the configuration to use (if set)")
	flag.StringVar(&inspectBlock, "inspectBlock", "", "Prints the configuration contained in the block at the specified path")
	flag.StringVar(&inspectChannelCreateTx, "inspectChannelCreateTx", "", "Prints the configuration contained in the transaction at the specified path")
	flag.StringVar(&outputAnchorPeersUpdate, "outputAnchorPeersUpdate", "", "Creates an config update to update an anchor peer (works only with the default channel creation, and only for the first update)")
	flag.StringVar(&asOrg, "asOrg", "", "Performs the config generation as a particular organization (by name), only including values in the write set that org (likely) has privilege to set")
	flag.StringVar(&printOrg, "printOrg", "", "Prints the definition of an organization as JSON. (useful for adding an org to a channel manually)")

	version := flag.Bool("version", false, "Show version information")

	flag.Parse()

	if channelID == "" && (outputBlock != "" || outputChannelCreateTx != "" || outputAnchorPeersUpdate != "") {
		channelID = genesisconfig.TestChainID
		logger.Warningf("Omitting the channel ID for configtxgen for output operations is deprecated.  Explicitly passing the channel ID will be required in the future, defaulting to '%s'.", channelID)
	}

	// show version
	if *version {
		printVersion()
		os.Exit(exitCode)
	}

	// don't need to panic when running via command line
	defer func() {
		if err := recover(); err != nil {
			if strings.Contains(fmt.Sprint(err), "Error reading configuration: Unsupported Config Type") {
				logger.Error("Could not find configtx.yaml. " +
					"Please make sure that FABRIC_CFG_PATH or -configPath is set to a path " +
					"which contains configtx.yaml")
				os.Exit(1)
			}
			if strings.Contains(fmt.Sprint(err), "Could not find profile") {
				logger.Error(fmt.Sprint(err) + ". " +
					"Please make sure that FABRIC_CFG_PATH or -configPath is set to a path " +
					"which contains configtx.yaml with the specified profile")
				os.Exit(1)
			}
			logger.Panic(err)
		}
	}()

	logger.Info("Loading configuration")
	factory.InitFactories(nil)
	var profileConfig *genesisconfig.Profile
	if outputBlock != "" || outputChannelCreateTx != "" || outputAnchorPeersUpdate != "" {
		if configPath != "" {
			profileConfig = genesisconfig.Load(profile, configPath)
		} else {
			profileConfig = genesisconfig.Load(profile)
		}
	}
	var topLevelConfig *genesisconfig.TopLevel
	//if configPath != "" {
	//	topLevelConfig = genesisconfig.LoadTopLevel(configPath)
	//} else {
	//	topLevelConfig = genesisconfig.LoadTopLevel()
	//}

    configstr := "{\"Organizations\": [\n    {\n      \"Name\": \"OrdererOrg\",\n      \"ID\": \"OrdererMSP\",\n      \"MSPDir\": \"crypto-config/ordererOrganizations/example.com/msp\",\n      \"Policies\": {\n        \"Readers\": {\n          \"Type\": \"Signature\",\n          \"Rule\": \"OR('OrdererMSP.member')\"\n        },\n        \"Writers\": {\n          \"Type\": \"Signature\",\n          \"Rule\": \"OR('OrdererMSP.member')\"\n        },\n        \"Admins\": {\n          \"Type\": \"Signature\",\n          \"Rule\": \"OR('OrdererMSP.admin')\"\n        }\n      }\n    },\n    {\n      \"Name\": \"Org1MSP\",\n      \"ID\": \"Org1MSP\",\n      \"MSPDir\": \"crypto-config/peerOrganizations/org1.example.com/msp\",\n      \"Policies\": {\n        \"Readers\": {\n          \"Type\": \"Signature\",\n          \"Rule\": \"OR('Org1MSP.admin', 'Org1MSP.peer', 'Org1MSP.client')\"\n        },\n        \"Writers\": {\n          \"Type\": \"Signature\",\n          \"Rule\": \"OR('Org1MSP.admin', 'Org1MSP.client')\"\n        },\n        \"Admins\": {\n          \"Type\": \"Signature\",\n          \"Rule\": \"OR('Org1MSP.admin')\"\n        }\n      },\n      \"AnchorPeers\": [\n        {\n          \"Host\": \"peer0.org1.example.com\",\n          \"Port\": 7051\n        }\n      ]\n    },\n    {\n      \"Name\": \"Org2MSP\",\n      \"ID\": \"Org2MSP\",\n      \"MSPDir\": \"crypto-config/peerOrganizations/org2.example.com/msp\",\n      \"Policies\": {\n        \"Readers\": {\n          \"Type\": \"Signature\",\n          \"Rule\": \"OR('Org2MSP.admin', 'Org2MSP.peer', 'Org2MSP.client')\"\n        },\n        \"Writers\": {\n          \"Type\": \"Signature\",\n          \"Rule\": \"OR('Org2MSP.admin', 'Org2MSP.client')\"\n        },\n        \"Admins\": {\n          \"Type\": \"Signature\",\n          \"Rule\": \"OR('Org2MSP.admin')\"\n        }\n      },\n      \"AnchorPeers\": [\n        {\n          \"Host\": \"peer0.org2.example.com\",\n          \"Port\": 7051\n        }\n      ]\n    }\n  ],\n  \"Capabilities\": {\n    \"Channel\": {\n      \"V1_3\": true\n    },\n    \"Orderer\": {\n      \"V1_1\": true\n    },\n    \"Application\": {\n      \"V1_3\": true,\n      \"V1_2\": false,\n      \"V1_1\": false\n    }\n  },\n  \"Application\": {\n    \"Organizations\": null,\n    \"Policies\": {\n      \"Readers\": {\n        \"Type\": \"ImplicitMeta\",\n        \"Rule\": \"ANY Readers\"\n      },\n      \"Writers\": {\n        \"Type\": \"ImplicitMeta\",\n        \"Rule\": \"ANY Writers\"\n      },\n      \"Admins\": {\n        \"Type\": \"ImplicitMeta\",\n        \"Rule\": \"MAJORITY Admins\"\n      }\n    },\n    \"Capabilities\": {\n      \"V1_3\": true,\n      \"V1_2\": false,\n      \"V1_1\": false\n    }\n  },\n  \"Orderer\": {\n    \"OrdererType\": \"solo\",\n    \"Addresses\": [\n      \"orderer.example.com:7050\"\n    ],\n    \"BatchTimeout\": \"2s\",\n    \"BatchSize\": {\n      \"MaxMessageCount\": \"10\",\n      \"AbsoluteMaxBytes\": \"99\",\n      \"PreferredMaxBytes\": \"512\"\n    },\n    \"Kafka\": {\n      \"Brokers\": [\n        \"127.0.0.1:9092\"\n      ]\n    },\n    \"Organizations\": null,\n    \"Policies\": {\n      \"Readers\": {\n        \"Type\": \"ImplicitMeta\",\n        \"Rule\": \"ANY Readers\"\n      },\n      \"Writers\": {\n        \"Type\": \"ImplicitMeta\",\n        \"Rule\": \"ANY Writers\"\n      },\n      \"Admins\": {\n        \"Type\": \"ImplicitMeta\",\n        \"Rule\": \"MAJORITY Admins\"\n      },\n      \"BlockValidation\": {\n        \"Type\": \"ImplicitMeta\",\n        \"Rule\": \"ANY Writers\"\n      }\n    }\n  },\n  \"Channel\": {\n    \"Policies\": {\n      \"Readers\": {\n        \"Type\": \"ImplicitMeta\",\n        \"Rule\": \"ANY Readers\"\n      },\n      \"Writers\": {\n        \"Type\": \"ImplicitMeta\",\n        \"Rule\": \"ANY Writers\"\n      },\n      \"Admins\": {\n        \"Type\": \"ImplicitMeta\",\n        \"Rule\": \"MAJORITY Admins\"\n      }\n    },\n    \"Capabilities\": {\n      \"V1_3\": true\n    }\n  },\n  \"Profiles\": {\n    \"OneOrgsOrdererGenesis\": {\n      \"Policies\": {\n        \"Readers\": {\n          \"Type\": \"ImplicitMeta\",\n          \"Rule\": \"ANY Readers\"\n        },\n        \"Writers\": {\n          \"Type\": \"ImplicitMeta\",\n          \"Rule\": \"ANY Writers\"\n        },\n        \"Admins\": {\n          \"Type\": \"ImplicitMeta\",\n          \"Rule\": \"MAJORITY Admins\"\n        }\n      },\n      \"Capabilities\": {\n        \"V1_3\": true\n      },\n      \"Orderer\": {\n        \"OrdererType\": \"solo\",\n        \"Addresses\": [\n          \"orderer.example.com:7050\"\n        ],\n        \"BatchTimeout\": \"2s\",\n        \"BatchSize\": {\n          \"MaxMessageCount\": 10,\n          \"AbsoluteMaxBytes\": \"99 MB\",\n          \"PreferredMaxBytes\": \"512\"\n        },\n        \"Kafka\": {\n          \"Brokers\": [\n            \"127.0.0.1:9092\"\n          ]\n        },\n        \"Organizations\": [\n          {\n            \"Name\": \"OrdererOrg\",\n            \"ID\": \"OrdererMSP\",\n            \"MSPDir\": \"crypto-config/ordererOrganizations/example.com/msp\",\n            \"Policies\": {\n              \"Readers\": {\n                \"Type\": \"Signature\",\n                \"Rule\": \"OR('OrdererMSP.member')\"\n              },\n              \"Writers\": {\n                \"Type\": \"Signature\",\n                \"Rule\": \"OR('OrdererMSP.member')\"\n              },\n              \"Admins\": {\n                \"Type\": \"Signature\",\n                \"Rule\": \"OR('OrdererMSP.admin')\"\n              }\n            }\n          }\n        ],\n        \"Policies\": {\n          \"Readers\": {\n            \"Type\": \"ImplicitMeta\",\n            \"Rule\": \"ANY Readers\"\n          },\n          \"Writers\": {\n            \"Type\": \"ImplicitMeta\",\n            \"Rule\": \"ANY Writers\"\n          },\n          \"Admins\": {\n            \"Type\": \"ImplicitMeta\",\n            \"Rule\": \"MAJORITY Admins\"\n          },\n          \"BlockValidation\": {\n            \"Type\": \"ImplicitMeta\",\n            \"Rule\": \"ANY Writers\"\n          }\n        },\n        \"Capabilities\": {\n          \"V1_1\": true\n        }\n      },\n      \"Consortiums\": {\n        \"SampleConsortium\": {\n          \"Organizations\": [\n            {\n              \"Name\": \"Org1MSP\",\n              \"ID\": \"Org1MSP\",\n              \"MSPDir\": \"crypto-config/peerOrganizations/org1.example.com/msp\",\n              \"Policies\": {\n                \"Readers\": {\n                  \"Type\": \"Signature\",\n                  \"Rule\": \"OR('Org1MSP.admin', 'Org1MSP.peer', 'Org1MSP.client')\"\n                },\n                \"Writers\": {\n                  \"Type\": \"Signature\",\n                  \"Rule\": \"OR('Org1MSP.admin', 'Org1MSP.client')\"\n                },\n                \"Admins\": {\n                  \"Type\": \"Signature\",\n                  \"Rule\": \"OR('Org1MSP.admin')\"\n                }\n              },\n              \"AnchorPeers\": [\n                {\n                  \"Host\": \"peer0.org1.example.com\",\n                  \"Port\": 7051\n                }\n              ]\n            },\n            {\n              \"Name\": \"Org2MSP\",\n              \"ID\": \"Org2MSP\",\n              \"MSPDir\": \"crypto-config/peerOrganizations/org2.example.com/msp\",\n              \"Policies\": {\n                \"Readers\": {\n                  \"Type\": \"Signature\",\n                  \"Rule\": \"OR('Org2MSP.admin', 'Org2MSP.peer', 'Org2MSP.client')\"\n                },\n                \"Writers\": {\n                  \"Type\": \"Signature\",\n                  \"Rule\": \"OR('Org2MSP.admin', 'Org2MSP.client')\"\n                },\n                \"Admins\": {\n                  \"Type\": \"Signature\",\n                  \"Rule\": \"OR('Org2MSP.admin')\"\n                }\n              },\n              \"AnchorPeers\": [\n                {\n                  \"Host\": \"peer0.org2.example.com\",\n                  \"Port\": 7051\n                }\n              ]\n            }\n          ]\n        }\n      }\n    },\n    \"OneOrgsChannel\": {\n      \"Consortium\": \"SampleConsortium\",\n      \"Application\": {\n        \"Organizations\": [\n          {\n            \"Name\": \"Org1MSP\",\n            \"ID\": \"Org1MSP\",\n            \"MSPDir\": \"crypto-config/peerOrganizations/org1.example.com/msp\",\n            \"Policies\": {\n              \"Readers\": {\n                \"Type\": \"Signature\",\n                \"Rule\": \"OR('Org1MSP.admin', 'Org1MSP.peer', 'Org1MSP.client')\"\n              },\n              \"Writers\": {\n                \"Type\": \"Signature\",\n                \"Rule\": \"OR('Org1MSP.admin', 'Org1MSP.client')\"\n              },\n              \"Admins\": {\n                \"Type\": \"Signature\",\n                \"Rule\": \"OR('Org1MSP.admin')\"\n              }\n            },\n            \"AnchorPeers\": [\n              {\n                \"Host\": \"peer0.org1.example.com\",\n                \"Port\": 7051\n              }\n            ]\n          },\n          {\n            \"Name\": \"Org2MSP\",\n            \"ID\": \"Org2MSP\",\n            \"MSPDir\": \"crypto-config/peerOrganizations/org2.example.com/msp\",\n            \"Policies\": {\n              \"Readers\": {\n                \"Type\": \"Signature\",\n                \"Rule\": \"OR('Org2MSP.admin', 'Org2MSP.peer', 'Org2MSP.client')\"\n              },\n              \"Writers\": {\n                \"Type\": \"Signature\",\n                \"Rule\": \"OR('Org2MSP.admin', 'Org2MSP.client')\"\n              },\n              \"Admins\": {\n                \"Type\": \"Signature\",\n                \"Rule\": \"OR('Org2MSP.admin')\"\n              }\n            },\n            \"AnchorPeers\": [\n              {\n                \"Host\": \"peer0.org2.example.com\",\n                \"Port\": 7051\n              }\n            ]\n          }\n        ],\n        \"Policies\": {\n          \"Readers\": {\n            \"Type\": \"ImplicitMeta\",\n            \"Rule\": \"ANY Readers\"\n          },\n          \"Writers\": {\n            \"Type\": \"ImplicitMeta\",\n            \"Rule\": \"ANY Writers\"\n          },\n          \"Admins\": {\n            \"Type\": \"ImplicitMeta\",\n            \"Rule\": \"MAJORITY Admins\"\n          }\n        },\n        \"Capabilities\": {\n          \"V1_3\": true,\n          \"V1_2\": false,\n          \"V1_1\": false\n        }\n      }\n    },\n    \"SampleDevModeKafka\": {\n      \"Policies\": {\n        \"Readers\": {\n          \"Type\": \"ImplicitMeta\",\n          \"Rule\": \"ANY Readers\"\n        },\n        \"Writers\": {\n          \"Type\": \"ImplicitMeta\",\n          \"Rule\": \"ANY Writers\"\n        },\n        \"Admins\": {\n          \"Type\": \"ImplicitMeta\",\n          \"Rule\": \"MAJORITY Admins\"\n        }\n      },\n      \"Capabilities\": {\n        \"V1_3\": true\n      },\n      \"Orderer\": {\n        \"OrdererType\": \"kafka\",\n        \"Addresses\": [\n          \"orderer.example.com:7050\"\n        ],\n        \"BatchTimeout\": \"2s\",\n        \"BatchSize\": {\n          \"MaxMessageCount\": 10,\n          \"AbsoluteMaxBytes\": \"99 MB\",\n          \"PreferredMaxBytes\": \"512\"\n        },\n        \"Kafka\": {\n          \"Brokers\": [\n            \"kafka.example.com:9092\"\n          ]\n        },\n        \"Organizations\": [\n          {\n            \"Name\": \"OrdererOrg\",\n            \"ID\": \"OrdererMSP\",\n            \"MSPDir\": \"crypto-config/ordererOrganizations/example.com/msp\",\n            \"Policies\": {\n              \"Readers\": {\n                \"Type\": \"Signature\",\n                \"Rule\": \"OR('OrdererMSP.member')\"\n              },\n              \"Writers\": {\n                \"Type\": \"Signature\",\n                \"Rule\": \"OR('OrdererMSP.member')\"\n              },\n              \"Admins\": {\n                \"Type\": \"Signature\",\n                \"Rule\": \"OR('OrdererMSP.admin')\"\n              }\n            }\n          }\n        ],\n        \"Policies\": {\n          \"Readers\": {\n            \"Type\": \"ImplicitMeta\",\n            \"Rule\": \"ANY Readers\"\n          },\n          \"Writers\": {\n            \"Type\": \"ImplicitMeta\",\n            \"Rule\": \"ANY Writers\"\n          },\n          \"Admins\": {\n            \"Type\": \"ImplicitMeta\",\n            \"Rule\": \"MAJORITY Admins\"\n          },\n          \"BlockValidation\": {\n            \"Type\": \"ImplicitMeta\",\n            \"Rule\": \"ANY Writers\"\n          }\n        },\n        \"Capabilities\": {\n          \"V1_1\": true\n        }\n      },\n      \"Application\": {\n        \"Organizations\": [\n          {\n            \"Name\": \"OrdererOrg\",\n            \"ID\": \"OrdererMSP\",\n            \"MSPDir\": \"crypto-config/ordererOrganizations/example.com/msp\",\n            \"Policies\": {\n              \"Readers\": {\n                \"Type\": \"Signature\",\n                \"Rule\": \"OR('OrdererMSP.member')\"\n              },\n              \"Writers\": {\n                \"Type\": \"Signature\",\n                \"Rule\": \"OR('OrdererMSP.member')\"\n              },\n              \"Admins\": {\n                \"Type\": \"Signature\",\n                \"Rule\": \"OR('OrdererMSP.admin')\"\n              }\n            }\n          }\n        ],\n        \"Policies\": {\n          \"Readers\": {\n            \"Type\": \"ImplicitMeta\",\n            \"Rule\": \"ANY Readers\"\n          },\n          \"Writers\": {\n            \"Type\": \"ImplicitMeta\",\n            \"Rule\": \"ANY Writers\"\n          },\n          \"Admins\": {\n            \"Type\": \"ImplicitMeta\",\n            \"Rule\": \"MAJORITY Admins\"\n          }\n        },\n        \"Capabilities\": {\n          \"V1_3\": true,\n          \"V1_2\": false,\n          \"V1_1\": false\n        }\n      },\n      \"Consortiums\": {\n        \"SampleConsortium\": {\n          \"Organizations\": [\n            {\n              \"Name\": \"Org1MSP\",\n              \"ID\": \"Org1MSP\",\n              \"MSPDir\": \"crypto-config/peerOrganizations/org1.example.com/msp\",\n              \"Policies\": {\n                \"Readers\": {\n                  \"Type\": \"Signature\",\n                  \"Rule\": \"OR('Org1MSP.admin', 'Org1MSP.peer', 'Org1MSP.client')\"\n                },\n                \"Writers\": {\n                  \"Type\": \"Signature\",\n                  \"Rule\": \"OR('Org1MSP.admin', 'Org1MSP.client')\"\n                },\n                \"Admins\": {\n                  \"Type\": \"Signature\",\n                  \"Rule\": \"OR('Org1MSP.admin')\"\n                }\n              },\n              \"AnchorPeers\": [\n                {\n                  \"Host\": \"peer0.org1.example.com\",\n                  \"Port\": 7051\n                }\n              ]\n            },\n            {\n              \"Name\": \"Org2MSP\",\n              \"ID\": \"Org2MSP\",\n              \"MSPDir\": \"crypto-config/peerOrganizations/org2.example.com/msp\",\n              \"Policies\": {\n                \"Readers\": {\n                  \"Type\": \"Signature\",\n                  \"Rule\": \"OR('Org2MSP.admin', 'Org2MSP.peer', 'Org2MSP.client')\"\n                },\n                \"Writers\": {\n                  \"Type\": \"Signature\",\n                  \"Rule\": \"OR('Org2MSP.admin', 'Org2MSP.client')\"\n                },\n                \"Admins\": {\n                  \"Type\": \"Signature\",\n                  \"Rule\": \"OR('Org2MSP.admin')\"\n                }\n              },\n              \"AnchorPeers\": [\n                {\n                  \"Host\": \"peer0.org2.example.com\",\n                  \"Port\": 7051\n                }\n              ]\n            }\n          ]\n        }\n      }\n    }\n  }\n}"
	err  :=json.Unmarshal([]byte(configstr),&topLevelConfig)
	fmt.Println("err",err)

//}


	var genProfile *genesisconfig.Profile
	var c1 *genesisconfig.Profile

	genProfile = topLevelConfig.Profiles["OneOrgsOrdererGenesis"]
	if genProfile != nil{
		fmt.Println("genProfile",*genProfile)
	}else{
		fmt.Println("nil")
	}
	c1 = topLevelConfig.Profiles["OneOrgsChannel"]

	if c1 != nil{
		fmt.Println("c1",*c1)
	}else{
		fmt.Println("nil")
	}


	if outputBlock != "" {
		if err := doOutputBlock(genProfile, channelID, outputBlock); err != nil {
			logger.Fatalf("Error on outputBlock: %s", err)
		}
	}

	if outputChannelCreateTx != "" {
		if err := doOutputChannelCreateTx(c1, channelID, outputChannelCreateTx); err != nil {
			logger.Fatalf("Error on outputChannelCreateTx: %s", err)
		}
	}

	if inspectBlock != "" {
		if err := doInspectBlock(inspectBlock); err != nil {
			logger.Fatalf("Error on inspectBlock: %s", err)
		}
	}

	if inspectChannelCreateTx != "" {
		if err := doInspectChannelCreateTx(inspectChannelCreateTx); err != nil {
			logger.Fatalf("Error on inspectChannelCreateTx: %s", err)
		}
	}

	if outputAnchorPeersUpdate != "" {
		if err := doOutputAnchorPeersUpdate(profileConfig, channelID, outputAnchorPeersUpdate, asOrg); err != nil {
			logger.Fatalf("Error on inspectChannelCreateTx: %s", err)
		}
	}

	if printOrg != "" {
		if err := doPrintOrg(topLevelConfig, printOrg); err != nil {
			logger.Fatalf("Error on printOrg: %s", err)
		}
	}
}

func printVersion() {
	fmt.Println(metadata.GetVersionInfo())
}
