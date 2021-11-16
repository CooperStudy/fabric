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
package factory

import (
	"sync"

	"github.com/hyperledger/fabric/bccsp"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/pkg/errors"
)

var (
	// Default BCCSP
	defaultBCCSP bccsp.BCCSP

	// when InitFactories has not been called yet (should only happen
	// in test cases), use this BCCSP temporarily
	bootBCCSP bccsp.BCCSP

	// BCCSP Factories
	bccspMap map[string]bccsp.BCCSP

	// factories' Sync on Initialization
	factoriesInitOnce sync.Once
	bootBCCSPInitOnce sync.Once

	// Factories' Initialization Error
	factoriesInitError error

	logger = flogging.MustGetLogger("bccsp")
)

// BCCSPFactory is used to get instances of the BCCSP interface.
// A Factory has name used to address it.
type BCCSPFactory interface {

	// Name returns the name of this factory
	Name() string

	// Get returns an instance of BCCSP using opts.
	Get(opts *FactoryOpts) (bccsp.BCCSP, error)
}

// GetDefault returns a non-ephemeral (long-term) BCCSP
func GetDefault() bccsp.BCCSP {
	//fmt.Println("====fabric-factory-factory.go===")
	//logger.Info("=======GetDefault=================defaultBCCSP",defaultBCCSP)//0xc000114b90
	if defaultBCCSP == nil {
		logger.Warning("Before using BCCSP, please call InitFactories(). Falling back to bootBCCSP.")
		bootBCCSPInitOnce.Do(func() {
			var err error
			f := &SWFactory{}
			bootBCCSP, err = f.Get(GetDefaultOpts())
			if err != nil {
				panic("BCCSP Internal error, failed initialization with GetDefaultOpts!")
			}
		})
		return bootBCCSP
	}
	//fmt.Printf("=====defaultBCCSP type:%T===\n",defaultBCCSP)
	//*bccsp.SHA256Opts
	//========s========= =======GetDefault=================defaultBCCSP &{0x17da2d8 map[*bccsp.AES192KeyGenOpts:0xc00003cf98 *bccsp.RSAKeyGenOpts:0xc00003cfb8 *bccsp.RSA2048KeyGenOpts:0xc00003cfd8 *bccsp.RSA3072KeyGenOpts:0xc00003cfe8 *bccsp.RSA4096KeyGenOpts:0xc00003cff8 *bccsp.ECDSAP384KeyGenOpts:0xc0003476d0 *bccsp.AESKeyGenOpts:0xc00003cf78 *bccsp.AES256KeyGenOpts:0xc00003cf88 *bccsp.RSA1024KeyGenOpts:0xc00003cfc8 *bccsp.ECDSAKeyGenOpts:0xc0003476b0 *bccsp.ECDSAP256KeyGenOpts:0xc0003476c0 *bccsp.AES128KeyGenOpts:0xc00003cfa8] map[*sw.ecdsaPrivateKey:0x17da2d8 *sw.ecdsaPublicKey:0x17da2d8 *sw.aesPrivateKey:0xc00000e510] map[*bccsp.AES256ImportKeyOpts:0x17da2d8 *bccsp.HMACImportKeyOpts:0x17da2d8 *bccsp.ECDSAPKIXPublicKeyImportOpts:0x17da2d8 *bccsp.ECDSAPrivateKeyImportOpts:0x17da2d8 *bccsp.ECDSAGoPublicKeyImportOpts:0x17da2d8 *bccsp.RSAGoPublicKeyImportOpts:0x17da2d8 *bccsp.X509PublicKeyImportOpts:0xc00000e518] map[*sw.aesPrivateKey:0x17da2d8] map[*sw.aesPrivateKey:0x17da2d8] map[*sw.ecdsaPrivateKey:0x17da2d8 *sw.rsaPrivateKey:0x17da2d8] map[*sw.ecdsaPrivateKey:0x17da2d8 *sw.ecdsaPublicKey:0x17da2d8 *sw.rsaPrivateKey:0x17da2d8 *sw.rsaPublicKey:0x17da2d8] map[*bccsp.SHA3_384Opts:0xc00000e4f8 *bccsp.SHAOpts:0xc00000e4d8 *bccsp.SHA256Opts:0xc00000e4e0 *bccsp.SHA384Opts:0xc00000e4e8 *bccsp.SHA3_256Opts:0xc00000e4f0]}
	return defaultBCCSP
}

// GetBCCSP returns a BCCSP created according to the options passed in input.
func GetBCCSP(name string) (bccsp.BCCSP, error) {
	//logger.Info("=======GetBCCSP=================")
	csp, ok := bccspMap[name]
	if !ok {
		return nil, errors.Errorf("Could not find BCCSP, no '%s' provider", name)
	}
	return csp, nil
}

func initBCCSP(f BCCSPFactory, config *FactoryOpts) error {
	//logger.Info("=======initBCCSP=================")
	csp, err := f.Get(config)
	if err != nil {
		return errors.Errorf("Could not initialize BCCSP %s [%s]", f.Name(), err)
	}

	logger.Debugf("Initialize BCCSP [%s]", f.Name())
	bccspMap[f.Name()] = csp
	return nil
}
