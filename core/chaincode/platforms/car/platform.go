/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package car

import (
	"archive/tar"
	"bytes"
	"fmt"
	"github.com/hyperledger/fabric/common/flogging"
	"io"
	"io/ioutil"
	"strings"

	"github.com/hyperledger/fabric/core/chaincode/platforms"
	"github.com/hyperledger/fabric/core/chaincode/platforms/util"
	cutil "github.com/hyperledger/fabric/core/container/util"
	pb "github.com/hyperledger/fabric/protos/peer"
)

// Platform for the CAR type
type Platform struct {
}
var logger = flogging.MustGetLogger("core.chaincode.platforms.car")

// Name returns the name of this platform
func (carPlatform *Platform) Name() string {
	logger.Info("==========Platform=========Name======================")
	return pb.ChaincodeSpec_CAR.String()
}

// ValidatePath validates the chaincode path for CAR types to satisfy
// the platform interface.  This chaincode type currently doesn't
// require anything specific so we just implicitly approve any spec
func (carPlatform *Platform) ValidatePath(path string) error {
	logger.Info("==========Platform=========ValidatePath======================")
	return nil
}

func (carPlatform *Platform) ValidateCodePackage(codePackage []byte) error {
	logger.Info("==========Platform=========ValidateCodePackage======================")
	// CAR platform will validate the code package within chaintool
	return nil
}

func (carPlatform *Platform) GetDeploymentPayload(path string) ([]byte, error) {
	logger.Info("==========Platform=========GetDeploymentPayload======================")
	logger.Info("======path=====",path)
	return ioutil.ReadFile(path)
}

func (carPlatform *Platform) GenerateDockerfile() (string, error) {
	logger.Info("==========Platform=========GenerateDockerfile======================")
	var buf []string

	//let the executable's name be chaincode ID's name
	buf = append(buf, "FROM "+cutil.GetDockerfileFromConfig("chaincode.car.runtime"))
	buf = append(buf, "ADD binpackage.tar /usr/local/bin")

	dockerFileContents := strings.Join(buf, "\n")

	return dockerFileContents, nil
}

func (carPlatform *Platform) GenerateDockerBuild(path string, code []byte, tw *tar.Writer) error {
	logger.Info("==========Platform=========GenerateDockerBuild======================")
	// Bundle the .car file into a tar stream so it may be transferred to the builder container
	codepackage, output := io.Pipe()
	go func() {
		tw := tar.NewWriter(output)

		err := cutil.WriteBytesToPackage("codepackage.car", code, tw)

		tw.Close()
		output.CloseWithError(err)
	}()

	binpackage := bytes.NewBuffer(nil)
	err := util.DockerBuild(util.DockerBuildOptions{
		Cmd:          "java -jar /usr/local/bin/chaintool buildcar /chaincode/input/codepackage.car -o /chaincode/output/chaincode",
		InputStream:  codepackage,
		OutputStream: binpackage,
	})
	if err != nil {
		return fmt.Errorf("Error building CAR: %s", err)
	}

	return cutil.WriteBytesToPackage("binpackage.tar", binpackage.Bytes(), tw)
}

//GetMetadataProvider fetches metadata provider given deployment spec
func (carPlatform *Platform) GetMetadataProvider(code []byte) platforms.MetadataProvider {
	logger.Info("==========Platform=========GetMetadataProvider======================")
	return &MetadataProvider{}
}
