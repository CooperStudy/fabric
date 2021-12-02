/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cceventmgmt

import (
	"sync"

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/ledger"
)

var logger = flogging.MustGetLogger("cceventmgmt")

var mgr *Mgr

// Initialize initializes event mgmt
func Initialize(ccInfoProvider ChaincodeInfoProvider) {
	logger.Info("========Initialize=================")
	initialize(ccInfoProvider)
}

func initialize(ccInfoProvider ChaincodeInfoProvider) {
	logger.Info("========initialize=================")
	mgr = newMgr(ccInfoProvider)
}

// GetMgr returns the reference to singleton event manager
func GetMgr() *Mgr {
	logger.Info("========GetMgr=================")
	return mgr
}

// Mgr encapsulate important interactions for events related to the interest of ledger
type Mgr struct {
	// rwlock is mainly used to synchronize across deploy transaction, chaincode install, and channel creation
	// Ideally, different services in the peer should be designed such that they expose locks for different important
	// events so that a code on top can synchronize across if needs to. However, in the lack of any such system-wide design,
	// we use this lock for contextual use
	rwlock               sync.RWMutex
	infoProvider         ChaincodeInfoProvider
	ccLifecycleListeners map[string][]ChaincodeLifecycleEventListener
	callbackStatus       *callbackStatus
}

func newMgr(chaincodeInfoProvider ChaincodeInfoProvider) *Mgr {
	logger.Info("========newMgr=================")
	return &Mgr{
		infoProvider:         chaincodeInfoProvider,
		ccLifecycleListeners: make(map[string][]ChaincodeLifecycleEventListener),
		callbackStatus:       newCallbackStatus()}
}

// Register registers a ChaincodeLifecycleEventListener for given ledgerid
// Since, `Register` is expected to be invoked when creating/opening a ledger instance
func (m *Mgr) Register(ledgerid string, l ChaincodeLifecycleEventListener) {
	logger.Info("====Mgr====Register=================")
	// write lock to synchronize concurrent 'chaincode install' operations with ledger creation/open
	m.rwlock.Lock()
	defer m.rwlock.Unlock()
	m.ccLifecycleListeners[ledgerid] = append(m.ccLifecycleListeners[ledgerid], l)
}

// HandleChaincodeDeploy is expected to be invoked when a chaincode is deployed via a deploy transaction
// The `chaincodeDefinitions` parameter contains all the chaincodes deployed in a block
// We need to store the last received `chaincodeDefinitions` because this function is expected to be invoked
// after the deploy transactions validation is performed but not committed yet to the ledger. Further, we
// release the read lock after this function. This leaves a small window when a `chaincode install` can happen
// before the deploy transaction is committed and hence the function `HandleChaincodeInstall` may miss finding
// the deployed chaincode. So, in function `HandleChaincodeInstall`, we explicitly check for chaincode deployed
// in this stored `chaincodeDefinitions`
func (m *Mgr) HandleChaincodeDeploy(chainid string, chaincodeDefinitions []*ChaincodeDefinition) error {
	logger.Info("====Mgr====HandleChaincodeDeploy=================")
	logger.Debugf("Channel [%s]: Handling chaincode deploy event for chaincode [%s]", chainid, chaincodeDefinitions)
	// Read lock to allow concurrent deploy on multiple channels but to synchronize concurrent `chaincode install` operation
	m.rwlock.RLock()
	for _, chaincodeDefinition := range chaincodeDefinitions {
		installed, dbArtifacts, err := m.infoProvider.RetrieveChaincodeArtifacts(chaincodeDefinition)
		if err != nil {
			return err
		}
		if !installed {
			logger.Infof("Channel [%s]: Chaincode [%s] is not installed hence no need to create chaincode artifacts for endorsement",
				chainid, chaincodeDefinition)
			continue
		}
		m.callbackStatus.setDeployPending(chainid)
		if err := m.invokeHandler(chainid, chaincodeDefinition, dbArtifacts); err != nil {
			logger.Warningf("Channel [%s]: Error while invoking a listener for handling chaincode install event: %s", chainid, err)
			return err
		}
		logger.Debugf("Channel [%s]: Handled chaincode deploy event for chaincode [%s]", chainid, chaincodeDefinitions)
	}
	return nil
}

// ChaincodeDeployDone is expected to be called when the deploy transaction state is committed
func (m *Mgr) ChaincodeDeployDone(chainid string) {
	logger.Info("====Mgr====ChaincodeDeployDone=================")
	// release the lock aquired in function `HandleChaincodeDeploy`
	defer m.rwlock.RUnlock()
	if m.callbackStatus.isDeployPending(chainid) {
		m.invokeDoneOnHandlers(chainid, true)
		m.callbackStatus.unsetDeployPending(chainid)
	}
}

// HandleChaincodeInstall is expected to get invoked during installation of a chaincode package
func (m *Mgr) HandleChaincodeInstall(chaincodeDefinition *ChaincodeDefinition, dbArtifacts []byte) error {
	logger.Info("====Mgr====HandleChaincodeInstall=================")
	logger.Infof("HandleChaincodeInstall() - chaincodeDefinition=%#v", chaincodeDefinition)
	/*
	HandleChaincodeInstall() - chaincodeDefinition=&cceventmgmt.ChaincodeDefinition{Name:"acb", Hash:[]uint8{0x38, 0x6d, 0x86, 0x13, 0x9f, 0x29, 0x73, 0x2c, 0x45, 0xed, 0xf8, 0x98, 0xf7, 0x31, 0xc7, 0x74, 0xb, 0xf2, 0xbf, 0x2d, 0x50, 0x5, 0x2d, 0x6e, 0xac, 0x22, 0x19, 0xf2, 0x57, 0xf1, 0xa3, 0x90}, Version:"0", CollectionConfigs:(*common.CollectionConfigPackage)(nil)}
	*/
	// Write lock prevents concurrent deploy operations
	m.rwlock.Lock()
	for chainid := range m.ccLifecycleListeners {
		logger.Infof("Channel [%s]: Handling chaincode install event for chaincode [%s]", chainid, chaincodeDefinition)
		/*
		Channel [mychannel]: Handling chaincode install event for chaincode [Name=acb, Version=0, Hash=[]byte{0x38, 0x6d, 0x86, 0x13, 0x9f, 0x29, 0x73, 0x2c, 0x45, 0xed, 0xf8, 0x98, 0xf7, 0x31, 0xc7, 0x74, 0xb, 0xf2, 0xbf, 0x2d, 0x50, 0x5, 0x2d, 0x6e, 0xac, 0x22, 0x19, 0xf2, 0x57, 0xf1, 0xa3, 0x90}]
		*/
		var deployedCCInfo *ledger.DeployedChaincodeInfo
		var err error
		if deployedCCInfo, err = m.infoProvider.GetDeployedChaincodeInfo(chainid, chaincodeDefinition); err != nil {
			logger.Warningf("Channel [%s]: Error while getting the deployment status of chaincode: %s", chainid, err)
			return err
		}
		if deployedCCInfo == nil {
			logger.Debugf("Channel [%s]: Chaincode [%s] is not deployed on channel hence not creating chaincode artifacts.",
				chainid, chaincodeDefinition)
			continue
		}
		m.callbackStatus.setInstallPending(chainid)
		chaincodeDefinition.CollectionConfigs = deployedCCInfo.CollectionConfigPkg
		if err := m.invokeHandler(chainid, chaincodeDefinition, dbArtifacts); err != nil {
			logger.Warningf("Channel [%s]: Error while invoking a listener for handling chaincode install event: %s", chainid, err)
			return err
		}
		logger.Infof("Channel [%s]: Handled chaincode install event for chaincode [%s]", chainid, chaincodeDefinition)
	}
	return nil
}

// ChaincodeInstallDone is expected to get invoked when chaincode install finishes
func (m *Mgr) ChaincodeInstallDone(succeeded bool) {
	logger.Info("====Mgr====ChaincodeInstallDone=================")
	// release the lock acquired in function `HandleChaincodeInstall`
	defer m.rwlock.Unlock()
	logger.Info("==============m.callbackStatus.installPending  len============",len(m.callbackStatus.installPending))
	for chainid := range m.callbackStatus.installPending {
		logger.Info("======chainid=====",chainid)
		m.invokeDoneOnHandlers(chainid, succeeded)
		m.callbackStatus.unsetInstallPending(chainid)
	}
}

func (m *Mgr) invokeHandler(chainid string, chaincodeDefinition *ChaincodeDefinition, dbArtifactsTar []byte) error {
	logger.Info("====Mgr====invokeHandler=================")
	listeners := m.ccLifecycleListeners[chainid]
	for _, listener := range listeners {
		if err := listener.HandleChaincodeDeploy(chaincodeDefinition, dbArtifactsTar); err != nil {
			return err
		}
	}
	return nil
}

func (m *Mgr) invokeDoneOnHandlers(chainid string, succeeded bool) {
	logger.Info("====Mgr====invokeDoneOnHandlers=================")
	listeners := m.ccLifecycleListeners[chainid]
	for _, listener := range listeners {
		logger.Info("===listener====",listener)
		listener.ChaincodeDeployDone(succeeded)
	}
}

type callbackStatus struct {
	l              sync.Mutex
	deployPending  map[string]bool
	installPending map[string]bool
}

func newCallbackStatus() *callbackStatus {
	logger.Info("====newCallbackStatus=================")
	return &callbackStatus{
		deployPending:  make(map[string]bool),
		installPending: make(map[string]bool)}
}

func (s *callbackStatus) setDeployPending(channelID string) {
	logger.Info("====callbackStatus=====setDeployPending============")
	s.l.Lock()
	defer s.l.Unlock()
	s.deployPending[channelID] = true
}

func (s *callbackStatus) unsetDeployPending(channelID string) {
	logger.Info("====callbackStatus=====unsetDeployPending============")
	s.l.Lock()
	defer s.l.Unlock()
	delete(s.deployPending, channelID)
}

func (s *callbackStatus) isDeployPending(channelID string) bool {
	logger.Info("====callbackStatus=====isDeployPending============")
	s.l.Lock()
	defer s.l.Unlock()
	return s.deployPending[channelID]
}

func (s *callbackStatus) setInstallPending(channelID string) {
	logger.Info("====callbackStatus=====setInstallPending============")
	s.l.Lock()
	defer s.l.Unlock()
	s.installPending[channelID] = true
}

func (s *callbackStatus) unsetInstallPending(channelID string) {
	logger.Info("====callbackStatus=====unsetInstallPending============")
	s.l.Lock()
	defer s.l.Unlock()
	delete(s.installPending, channelID)
}

func (s *callbackStatus) isInstallPending(channelID string) bool {
	logger.Info("====callbackStatus=====isInstallPending============")
	s.l.Lock()
	defer s.l.Unlock()
	return s.installPending[channelID]
}
