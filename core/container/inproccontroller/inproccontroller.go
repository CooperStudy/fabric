/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package inproccontroller

import (
	"context"
	"fmt"

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/core/container"
	"github.com/hyperledger/fabric/core/container/ccintf"
	pb "github.com/hyperledger/fabric/protos/peer"
)

// ContainerType is the string which the inproc container type
// is registered with the container.VMController
const ContainerType = "SYSTEM"

type inprocContainer struct {
	ChaincodeSupport ccintf.CCSupport
	chaincode        shim.Chaincode
	running          bool
	args             []string
	env              []string
	stopChan         chan struct{}
}

var (
	logger = flogging.MustGetLogger("inproccontroller")

	// TODO this is a very hacky way to do testing, we should find other ways
	// to test, or not statically inject these depenencies.
	_shimStartInProc    = shim.StartInProc
	_inprocLoggerErrorf = logger.Errorf
)

// errors

//SysCCRegisteredErr registered error
type SysCCRegisteredErr string

func (s SysCCRegisteredErr) Error() string {
	logger.Info("=SysCCRegisteredErr==Error==")
	return fmt.Sprintf("%s already registered", string(s))
}

// Registry stores registered system chaincodes.
// It implements container.VMProvider and scc.Registrar
type Registry struct {
	typeRegistry     map[string]*inprocContainer
	instRegistry     map[string]*inprocContainer
	ChaincodeSupport ccintf.CCSupport
}

// NewRegistry creates an initialized registry, ready to register system chaincodes.
// The returned *Registry is _not_ ready to use as is.  You must set the ChaincodeSupport
// as soon as one is available, before any chaincode invocations occur.  This is because
// the chaincode support used to be a latent dependency, snuck in on the context, but now
// it is being made an explicit part of the startup.
func NewRegistry() *Registry {
	logger.Info("=NewRegistry=")
	return &Registry{
		typeRegistry: make(map[string]*inprocContainer),
		instRegistry: make(map[string]*inprocContainer),
	}
}

// NewVM creates an inproc VM instance
func (r *Registry) NewVM() container.VM {
	logger.Info("=Registry===NewVM===")
	return NewInprocVM(r)
}

//Register registers system chaincode with given path. The deploy should be called to initialize
func (r *Registry) Register(ccid *ccintf.CCID, cc shim.Chaincode) error {
	logger.Info("=Registry===Register===")
	name := ccid.GetName()
	logger.Debugf("Registering chaincode instance: %s", name)
	tmp := r.typeRegistry[name]
	if tmp != nil {
		return SysCCRegisteredErr(name)
	}

	r.typeRegistry[name] = &inprocContainer{chaincode: cc}
	return nil
}

//InprocVM is a vm. It is identified by a executable name
type InprocVM struct {
	id       string
	registry *Registry
}

// NewInprocVM creates a new InprocVM
func NewInprocVM(r *Registry) *InprocVM {
	logger.Info("=NewInprocVM===")
	return &InprocVM{
		registry: r,
	}
}

func (vm *InprocVM) getInstance(ipctemplate *inprocContainer, instName string, args []string, env []string) (*inprocContainer, error) {
	logger.Info("==InprocVM===getInstance==")
	ipc := vm.registry.instRegistry[instName]
	if ipc != nil {
		logger.Warningf("chaincode instance exists for %s", instName)
		return ipc, nil
	}
	ipc = &inprocContainer{
		ChaincodeSupport: vm.registry.ChaincodeSupport,
		args:             args,
		env:              env,
		chaincode:        ipctemplate.chaincode,
		stopChan:         make(chan struct{}),
	}
	vm.registry.instRegistry[instName] = ipc
	logger.Debugf("chaincode instance created for %s", instName)
	return ipc, nil
}

func (ipc *inprocContainer) launchInProc(id string, args []string, env []string) error {
	logger.Info("==inprocContainer===launchInProc==")
	if ipc.ChaincodeSupport == nil {
		logger.Panicf("Chaincode support is nil, most likely you forgot to set it immediately after calling inproccontroller.NewRegsitry()")
	}

	peerRcvCCSend := make(chan *pb.ChaincodeMessage)
	ccRcvPeerSend := make(chan *pb.ChaincodeMessage)
	var err error
	ccchan := make(chan struct{}, 1)
	ccsupportchan := make(chan struct{}, 1)
	shimStartInProc := _shimStartInProc // shadow to avoid race in test
	go func() {
		defer close(ccchan)
		logger.Debugf("chaincode started for %s", id)
		if args == nil {
			args = ipc.args
		}
		if env == nil {
			env = ipc.env
		}
		err := shimStartInProc(env, args, ipc.chaincode, ccRcvPeerSend, peerRcvCCSend)
		if err != nil {
			err = fmt.Errorf("chaincode-support ended with err: %s", err)
			_inprocLoggerErrorf("%s", err)
		}
		logger.Debugf("chaincode ended for %s with err: %s", id, err)
	}()

	// shadow function to avoid data race
	inprocLoggerErrorf := _inprocLoggerErrorf
	go func() {
		defer close(ccsupportchan)
		inprocStream := newInProcStream(peerRcvCCSend, ccRcvPeerSend)
		logger.Debugf("chaincode-support started for  %s", id)
		err := ipc.ChaincodeSupport.HandleChaincodeStream(inprocStream)
		if err != nil {
			err = fmt.Errorf("chaincode ended with err: %s", err)
			inprocLoggerErrorf("%s", err)
		}
		logger.Debugf("chaincode-support ended for %s with err: %s", id, err)
	}()

	select {
	case <-ccchan:
		close(peerRcvCCSend)
		logger.Debugf("chaincode %s quit", id)
	case <-ccsupportchan:
		close(ccRcvPeerSend)
		logger.Debugf("chaincode support %s quit", id)
	case <-ipc.stopChan:
		close(ccRcvPeerSend)
		close(peerRcvCCSend)
		logger.Debugf("chaincode %s stopped", id)
	}
	return err
}

//Start starts a previously registered system codechain
func (vm *InprocVM) Start(ccid ccintf.CCID, args []string, env []string, filesToUpload map[string][]byte, builder container.Builder) error {
	logger.Info("==InprocVM===Start==")
	path := ccid.GetName()

	ipctemplate := vm.registry.typeRegistry[path]

	if ipctemplate == nil {
		return fmt.Errorf(fmt.Sprintf("%s not registered", path))
	}

	instName := vm.GetVMName(ccid)

	ipc, err := vm.getInstance(ipctemplate, instName, args, env)

	if err != nil {
		return fmt.Errorf(fmt.Sprintf("could not create instance for %s", instName))
	}

	if ipc.running {
		return fmt.Errorf(fmt.Sprintf("chaincode running %s", path))
	}

	ipc.running = true

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Criticalf("caught panic from chaincode  %s", instName)
			}
		}()
		ipc.launchInProc(instName, args, env)
	}()

	return nil
}

//Stop stops a system codechain
func (vm *InprocVM) Stop(ccid ccintf.CCID, timeout uint, dontkill bool, dontremove bool) error {
	logger.Info("==InprocVM===Stop==")
	path := ccid.GetName()

	ipctemplate := vm.registry.typeRegistry[path]
	if ipctemplate == nil {
		return fmt.Errorf("%s not registered", path)
	}

	instName := vm.GetVMName(ccid)

	ipc := vm.registry.instRegistry[instName]

	if ipc == nil {
		return fmt.Errorf("%s not found", instName)
	}

	if !ipc.running {
		return fmt.Errorf("%s not running", instName)
	}

	ipc.stopChan <- struct{}{}

	delete(vm.registry.instRegistry, instName)
	//TODO stop
	return nil
}

// HealthCheck is provided in order to implement the VMProvider interface.
// It always returns nil..
func (vm *InprocVM) HealthCheck(ctx context.Context) error {
	logger.Info("==InprocVM===HealthCheck==")
	return nil
}

// GetVMName ignores the peer and network name as it just needs to be unique in
// process.  It accepts a format function parameter to allow different
// formatting based on the desired use of the name.
func (vm *InprocVM) GetVMName(ccid ccintf.CCID) string {
	logger.Info("==InprocVM===GetVMName==")
	return ccid.GetName()
}
