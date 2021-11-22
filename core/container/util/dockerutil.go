/*
Copyright London Stock Exchange 2016 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package util

import (
	"fmt"
	"runtime"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hyperledger/fabric/common/metadata"
	"github.com/hyperledger/fabric/core/config"
	"github.com/spf13/viper"
)

//NewDockerClient creates a docker client
func NewDockerClient() (client *docker.Client, err error) {
	fmt.Println("==NewDockerClient==")
	endpoint := viper.GetString("vm.endpoint")//unix//
	tlsenabled := viper.GetBool("vm.docker.tls.enabled")
	fmt.Println("===========endpoint============",endpoint)
	//unix:///host/var/run/docker.sock
	fmt.Println("===========tlsenabled============",tlsenabled)//false
	if tlsenabled {
		cert := config.GetPath("vm.docker.tls.cert.file")
		key := config.GetPath("vm.docker.tls.key.file")
		ca := config.GetPath("vm.docker.tls.ca.file")
		client, err = docker.NewTLSClient(endpoint, cert, key, ca)
	} else {
		client, err = docker.NewClient(endpoint)
	}
	return
}

func ParseDockerfileTemplate(template string) string {
	fmt.Println("==ParseDockerfileTemplate==")
	r := strings.NewReplacer(
		"$(ARCH)", runtime.GOARCH,
		"$(PROJECT_VERSION)", metadata.Version,
		"$(BASE_VERSION)", metadata.BaseVersion,
		"$(DOCKER_NS)", metadata.DockerNamespace,
		"$(BASE_DOCKER_NS)", metadata.BaseDockerNamespace)

	return r.Replace(template)
}

func GetDockerfileFromConfig(path string) string {
	return ParseDockerfileTemplate(viper.GetString(path))
}
