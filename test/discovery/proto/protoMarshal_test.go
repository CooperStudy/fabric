package proto

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/protos/msp"
	"testing"
)

func Test(t *testing.T) {

	data, err := proto.Marshal(&msp.MSPRole{Role: 1, MspIdentifier:"1"})
	fmt.Println("data",data)
	fmt.Println("===errr===",err)

}