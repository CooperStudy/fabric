package policyprovider

import (
	"github.com/hyperledger/fabric/core/peer"
	"github.com/hyperledger/fabric/core/policy"
	"github.com/hyperledger/fabric/msp/mgmt"
)

// init is called when this package is loaded. This implementation registers the factory
func init() {
	policy.RegisterPolicyCheckerFactory(&defaultFactory{})
}

//实现了PolicyCheckerFactory接口defaultFactory并且初始化了一个实例对象
type defaultFactory struct{}

func (f *defaultFactory) NewPolicyChecker() policy.PolicyChecker {
	return policy.NewPolicyChecker(
		peer.NewChannelPolicyManagerGetter(),
		mgmt.GetLocalMSP(),
		mgmt.NewLocalMSPPrincipalGetter(),
	)
}

// GetPolicyChecker returns instances of PolicyChecker;
func GetPolicyChecker() policy.PolicyChecker {
	return policy.GetPolicyChecker()
}
