/*
Copyright IBM Corp. 2017 All Rights Reserved.

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

package sw

import (
	"fmt"
	"hash"

	"github.com/hyperledger/fabric/bccsp"
)

type hasher struct {
	hash func() hash.Hash
}

func (c *hasher) Hash(msg []byte, opts bccsp.HashOpts) ([]byte, error) {
	//fmt.Println("===hasher==Hash==")
	h := c.hash()
	//fmt.Println("===h",h)
	//fmt.Printf("===h type:%T==\n",h)//*sha256.digest
	//fmt.Println("===msg",msg)//[48 130 2 58 48 130
	//fmt.Println("===opts",opts)//=&{}
	h.Write(msg)
	b:= h.Sum(nil)
	//fmt.Println("=======h.Sum(nil)=====",b)
	return b,nil
}

func (c *hasher) GetHash(opts bccsp.HashOpts) (hash.Hash, error) {
	fmt.Println("===hasher==GetHash==")
	return c.hash(), nil
}
