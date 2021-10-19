/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package viperutil

import (
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Shopify/sarama"
	version "github.com/hashicorp/go-version"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

var logger = flogging.MustGetLogger("viperutil")

type viperGetter func(key string) interface{}

func getKeysRecursively(base string, getKey viperGetter, nodeKeys map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for key := range nodeKeys {
		fqKey := base + key
		val := getKey(fqKey)
		if m, ok := val.(map[interface{}]interface{}); ok {
			logger.Debugf("Found map[interface{}]interface{} value for %s", fqKey)
			tmp := make(map[string]interface{})
			for ik, iv := range m {
				cik, ok := ik.(string)
				if !ok {
					panic("Non string key-entry")
				}
				tmp[cik] = iv
			}
			result[key] = getKeysRecursively(fqKey+".", getKey, tmp)
		} else if m, ok := val.(map[string]interface{}); ok {
			logger.Debugf("Found map[string]interface{} value for %s", fqKey)
			result[key] = getKeysRecursively(fqKey+".", getKey, m)
		} else if m, ok := unmarshalJSON(val); ok {
			logger.Debugf("Found real value for %s setting to map[string]string %v", fqKey, m)
			result[key] = m
		} else {
			if val == nil {
				fileSubKey := fqKey + ".File"
				fileVal := getKey(fileSubKey)
				if fileVal != nil {
					result[key] = map[string]interface{}{"File": fileVal}
					continue
				}
			}
			logger.Debugf("Found real value for %s setting to %T %v", fqKey, val, val)
			result[key] = val

		}
	}
	return result
}

func unmarshalJSON(val interface{}) (map[string]string, bool) {
	mp := map[string]string{}
	s, ok := val.(string)
	if !ok {
		logger.Debugf("Unmarshal JSON: value is not a string: %v", val)
		return nil, false
	}
	err := json.Unmarshal([]byte(s), &mp)
	if err != nil {
		logger.Debugf("Unmarshal JSON: value cannot be unmarshalled: %s", err)
		return nil, false
	}
	return mp, true
}

// customDecodeHook adds the additional functions of parsing durations from strings
// as well as parsing strings of the format "[thing1, thing2, thing3]" into string slices
// Note that whitespace around slice elements is removed
func customDecodeHook() mapstructure.DecodeHookFunc {
	durationHook := mapstructure.StringToTimeDurationHookFunc()
	return func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
		dur, err := mapstructure.DecodeHookExec(durationHook, f, t, data)
		if err == nil {
			if _, ok := dur.(time.Duration); ok {
				return dur, nil
			}
		}

		if f.Kind() != reflect.String {
			return data, nil
		}

		raw := data.(string)
		l := len(raw)
		if l > 1 && raw[0] == '[' && raw[l-1] == ']' {
			slice := strings.Split(raw[1:l-1], ",")
			for i, v := range slice {
				slice[i] = strings.TrimSpace(v)
			}
			return slice, nil
		}

		return data, nil
	}
}

func byteSizeDecodeHook() mapstructure.DecodeHookFunc {
	return func(f reflect.Kind, t reflect.Kind, data interface{}) (interface{}, error) {
		if f != reflect.String || t != reflect.Uint32 {
			return data, nil
		}
		raw := data.(string)
		if raw == "" {
			return data, nil
		}
		var re = regexp.MustCompile(`^(?P<size>[0-9]+)\s*(?i)(?P<unit>(k|m|g))b?$`)
		if re.MatchString(raw) {
			size, err := strconv.ParseUint(re.ReplaceAllString(raw, "${size}"), 0, 64)
			if err != nil {
				return data, nil
			}
			unit := re.ReplaceAllString(raw, "${unit}")
			switch strings.ToLower(unit) {
			case "g":
				size = size << 10
				fallthrough
			case "m":
				size = size << 10
				fallthrough
			case "k":
				size = size << 10
			}
			if size > math.MaxUint32 {
				return size, fmt.Errorf("value '%s' overflows uint32", raw)
			}
			return size, nil
		}
		return data, nil
	}
}

func stringFromFileDecodeHook() mapstructure.DecodeHookFunc {
	return func(f reflect.Kind, t reflect.Kind, data interface{}) (interface{}, error) {
		// "to" type should be string
		if t != reflect.String {
			return data, nil
		}
		// "from" type should be map
		if f != reflect.Map {
			return data, nil
		}
		v := reflect.ValueOf(data)
		switch v.Kind() {
		case reflect.String:
			return data, nil
		case reflect.Map:
			d := data.(map[string]interface{})
			fileName, ok := d["File"]
			if !ok {
				fileName, ok = d["file"]
			}
			switch {
			case ok && fileName != nil:
				bytes, err := ioutil.ReadFile(fileName.(string))
				if err != nil {
					return data, err
				}
				return string(bytes), nil
			case ok:
				// fileName was nil
				return nil, fmt.Errorf("Value of File: was nil")
			}
		}
		return data, nil
	}
}

func pemBlocksFromFileDecodeHook() mapstructure.DecodeHookFunc {
	return func(f reflect.Kind, t reflect.Kind, data interface{}) (interface{}, error) {
		// "to" type should be string
		if t != reflect.Slice {
			return data, nil
		}
		// "from" type should be map
		if f != reflect.Map {
			return data, nil
		}
		v := reflect.ValueOf(data)
		switch v.Kind() {
		case reflect.String:
			return data, nil
		case reflect.Map:
			var fileName string
			var ok bool
			switch d := data.(type) {
			case map[string]string:
				fileName, ok = d["File"]
				if !ok {
					fileName, ok = d["file"]
				}
			case map[string]interface{}:
				var fileI interface{}
				fileI, ok = d["File"]
				if !ok {
					fileI, _ = d["file"]
				}
				fileName, ok = fileI.(string)
			}

			switch {
			case ok && fileName != "":
				var result []string
				bytes, err := ioutil.ReadFile(fileName)
				if err != nil {
					return data, err
				}
				for len(bytes) > 0 {
					var block *pem.Block
					block, bytes = pem.Decode(bytes)
					if block == nil {
						break
					}
					if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
						continue
					}
					result = append(result, string(pem.EncodeToMemory(block)))
				}
				return result, nil
			case ok:
				// fileName was nil
				return nil, fmt.Errorf("Value of File: was nil")
			}
		}
		return data, nil
	}
}

var kafkaVersionConstraints map[sarama.KafkaVersion]version.Constraints

func init() {
	kafkaVersionConstraints = make(map[sarama.KafkaVersion]version.Constraints)
	kafkaVersionConstraints[sarama.V0_8_2_0], _ = version.NewConstraint(">=0.8.2,<0.8.2.1")
	kafkaVersionConstraints[sarama.V0_8_2_1], _ = version.NewConstraint(">=0.8.2.1,<0.8.2.2")
	kafkaVersionConstraints[sarama.V0_8_2_2], _ = version.NewConstraint(">=0.8.2.2,<0.9.0.0")
	kafkaVersionConstraints[sarama.V0_9_0_0], _ = version.NewConstraint(">=0.9.0.0,<0.9.0.1")
	kafkaVersionConstraints[sarama.V0_9_0_1], _ = version.NewConstraint(">=0.9.0.1,<0.10.0.0")
	kafkaVersionConstraints[sarama.V0_10_0_0], _ = version.NewConstraint(">=0.10.0.0,<0.10.0.1")
	kafkaVersionConstraints[sarama.V0_10_0_1], _ = version.NewConstraint(">=0.10.0.1,<0.10.1.0")
	kafkaVersionConstraints[sarama.V0_10_1_0], _ = version.NewConstraint(">=0.10.1.0,<0.10.2.0")
	kafkaVersionConstraints[sarama.V0_10_2_0], _ = version.NewConstraint(">=0.10.2.0,<0.11.0.0")
	kafkaVersionConstraints[sarama.V0_11_0_0], _ = version.NewConstraint(">=0.11.0.0,<1.0.0")
	kafkaVersionConstraints[sarama.V1_0_0_0], _ = version.NewConstraint(">=1.0.0")
}

func kafkaVersionDecodeHook() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
		if f.Kind() != reflect.String || t != reflect.TypeOf(sarama.KafkaVersion{}) {
			return data, nil
		}

		v, err := version.NewVersion(data.(string))
		if err != nil {
			return nil, fmt.Errorf("Unable to parse Kafka version: %s", err)
		}

		for kafkaVersion, constraints := range kafkaVersionConstraints {
			if constraints.Check(v) {
				return kafkaVersion, nil
			}
		}

		return nil, fmt.Errorf("Unsupported Kafka version: '%s'", data)
	}
}

// EnhancedExactUnmarshal is intended to unmarshal a config file into a structure
// producing error when extraneous variables are introduced and supporting
// the time.Duration type
/*
EnhancedExactUnmarshal 旨在将配置文件解组为在引入无关变量时产生错误的结构，并且
支持time.Duration类型
 */
func EnhancedExactUnmarshal(v *viper.Viper, output interface{}) error {
	// AllKeys doesn't actually return all keys, it only returns the base ones
	baseKeys := v.AllSettings()
	getterWithClass := func(key string) interface{} { return v.Get(key) } // hide receiver
	leafKeys := getKeysRecursively("", getterWithClass, baseKeys)
	logger.Info("===leafKeys===",leafKeys)
	/*
	===leafKeys=== map[operations:map[ListenAddress:127.0.0.1:8443 TLS:map[ClientAuthRequired:false RootCA
	s:[] Enabled:false Certificate:<nil> PrivateKey:<nil>]] metrics:map[Statsd:map[WriteInterval:30s Prefix:<nil> Network:udp Address:127.0.0.1:8125] Provider:disabled] consensus:ma
	p[WALDir:/var/hyperledger/production/orderer/etcdraft/wal SnapDir:/var/hyperledger/production/orderer/etcdraft/snapshot] general:map[GenesisProfile:SampleInsecureSolo LocalMSPID
	:OrdererMSP Profile:map[Address:0.0.0.0:6060 Enabled:false] GenesisMethod:file LedgerType:file TLS:map[ClientRootCAs:<nil> Enabled:true PrivateKey:/var/hyperledger/orderer/tls/s
	erver.key Certificate:/var/hyperledger/orderer/tls/server.crt RootCAs:[/var/hyperledger/orderer/tls/ca.crt] ClientAuthRequired:false] Authentication:map[TimeWindow:15m] ListenAd
	dress:0.0.0.0 Keepalive:map[ServerInterval:7200s ServerTimeout:20s ServerMinInterval:60s] BCCSP:map[SW:map[Hash:SHA2 Security:256 FileKeyStore:map[KeyStore:<nil>]] Default:SW] C
	luster:map[ReplicationBufferSize:20971520 ReplicationPullTimeout:5s ReplicationRetryTimeout:5s ClientCertificate:<nil> ClientPrivateKey:<nil> DialTimeout:5s RPCTimeout:7s RootCA
	s:[tls/ca.crt]] ListenPort:7050 GenesisFile:/var/hyperledger/orderer/orderer.genesis.block LocalMSPDir:/var/hyperledger/orderer/msp] fileledger:map[Location:/var/hyperledger/pro
	duction/orderer Prefix:hyperledger-fabric-ordererledger] ramledger:map[HistorySize:1000] kafka:map[TLS:map[PrivateKey:<nil> Certificate:<nil> RootCAs:<nil> Enabled:false] SASLPl
	ain:map[Enabled:false User:<nil> Password:<nil>] Version:<nil> Retry:map[ShortInterval:5s ShortTotal:10m LongInterval:5m LongTotal:12h NetworkTimeouts:map[WriteTimeout:10s DialT
	imeout:10s ReadTimeout:10s] Metadata:map[RetryBackoff:250ms RetryMax:3] Producer:map[RetryMax:3 RetryBackoff:100ms] Consumer:map[RetryBackoff:2s]] Topic:map[ReplicationFactor:1]
	 Verbose:true] debug:map[DeliverTraceDir:<nil> BroadcastTraceDir:<nil>]]
	*/
	logger.Debugf("%+v", leafKeys)
	config := &mapstructure.DecoderConfig{
		ErrorUnused:      true,
		Metadata:         nil,
		Result:           output,
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			customDecodeHook(),
			byteSizeDecodeHook(),
			stringFromFileDecodeHook(),
			pemBlocksFromFileDecodeHook(),
			kafkaVersionDecodeHook(),
		),
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}
	return decoder.Decode(leafKeys)
}

// EnhancedExactUnmarshalKey is intended to unmarshal a config file subtreee into a structure
func EnhancedExactUnmarshalKey(baseKey string, output interface{}) error {
	m := make(map[string]interface{})
	m[baseKey] = nil
	leafKeys := getKeysRecursively("", viper.Get, m)

	logger.Debugf("%+v", leafKeys)

	config := &mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           output,
		WeaklyTypedInput: true,
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}

	return decoder.Decode(leafKeys[baseKey])
}

// Decode is used to decode opaque field in configuration
func Decode(input interface{}, output interface{}) error {
	return mapstructure.Decode(input, output)
}
