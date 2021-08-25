package container

import (
	"bytes"
	"fmt"

	"golang.org/x/net/context"

	"github.com/fsouza/go-dockerclient"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/chaincode/platforms"
	cutil "github.com/hyperledger/fabric/core/container/util"
	pb "github.com/hyperledger/fabric/protos/peer"
)

// VM implemenation of VM management functionality.
type VM struct {
	Client *docker.Client
}

// NewVM creates a new VM instance.
func NewVM() (*VM, error) {
	client, err := cutil.NewDockerClient()
	if err != nil {
		return nil, err
	}
	VM := &VM{Client: client}
	return VM, nil
}

var vmLogger = flogging.MustGetLogger("container")

// ListImages list the images available
func (vm *VM) ListImages(context context.Context) error {
	imgs, err := vm.Client.ListImages(docker.ListImagesOptions{All: false})
	if err != nil {
		return err
	}
	for _, img := range imgs {
		fmt.Println("ID: ", img.ID)
		fmt.Println("RepoTags: ", img.RepoTags)
		fmt.Println("Created: ", img.Created)
		fmt.Println("Size: ", img.Size)
		fmt.Println("VirtualSize: ", img.VirtualSize)
		fmt.Println("ParentId: ", img.ParentID)
	}

	return nil
}

// BuildChaincodeContainer builds the container for the supplied chaincode specification
func (vm *VM) BuildChaincodeContainer(spec *pb.ChaincodeSpec) error {
	/*
		选则go平台的goPlatform对象后，最终调用core/chaincode/platforms/golang/platform.go
	*/
	codePackage, err := GetChaincodePackageBytes(spec)
	if err != nil {
		return fmt.Errorf("Error getting chaincode package bytes: %s", err)
	}

	cds := &pb.ChaincodeDeploymentSpec{ChaincodeSpec: spec, CodePackage: codePackage}
	dockerSpec, err := platforms.GenerateDockerBuild(cds)
	if err != nil {
		return fmt.Errorf("Error getting chaincode docker image: %s", err)
	}

	output := bytes.NewBuffer(nil)

	err = vm.Client.BuildImage(docker.BuildImageOptions{
		Name:         spec.ChaincodeId.Name,
		InputStream:  dockerSpec,
		OutputStream: output,
	})
	if err != nil {
		return fmt.Errorf("Error building docker: %s (output = %s)", err, output.String())
	}

	return nil
}

// GetChaincodePackageBytes creates bytes for docker container generation using the supplied chaincode specification
func GetChaincodePackageBytes(spec *pb.ChaincodeSpec) ([]byte, error) {
	if spec == nil || spec.ChaincodeId == nil {
		return nil, fmt.Errorf("invalid chaincode spec")
	}

	return platforms.GetDeploymentPayload(spec)
}
