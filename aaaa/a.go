package main

import (
	"fmt"
	cc "github.com/hyperledger/fabric/peer/chaincode"
	"github.com/spf13/viper"
	"os"
	"strings"
)

func dirExists(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
}

func AddConfigPath(v *viper.Viper, p string) {
	if v != nil {
		v.AddConfigPath(p)
	} else {
		viper.AddConfigPath(p)
	}
}

var configName = "core"

func main() {
	viper.SetConfigName(configName)
	viper.SetEnvPrefix("core")
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.Set("peer.address","peer0.org1.example.com:7051")
	cf, err := cc.InitCmdFactoryQuery("query", true, false)
	fmt.Println(cf)
	//err = cc.ChaincodeInvokeOrQuery1(cf)
	fmt.Println(err)
}


