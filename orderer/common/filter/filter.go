/*
	几种filter可以组装到一起形成一个过滤器集合，orderer服务用此过滤集合（ruleSet）ruleset中的各个filter的Apply函数对收到的消息进行过滤，比如有些类型的消息
的一些字段不能为空，大小必须在配置的范围之内等。
	和filter对应的是一个提交对象committer，寄一个消息通过了filter的Apply，会返回一个committer，这个committer会在该消息写入账本之前执行Commit操作。
同时，committer的Isolated的返回值标识着一条消息是否要单独成块，一些比较特殊的消息可能在不满足一定大小的情况下也要单独作为一个区块，比如genesisBlock，一个filter集合，
就是一个合同，而一个envelope来到orderer中之后，会根据这个合同一条条地进行条款对照，如果符合，执行一个对应的committer工具，以使Envelope能拿到这个committer工具进一步，
完成一些事情。同时定义了三种基本的Apply过滤结果，Accept接收，reject拒绝，Forward表示需要进一步验证。filter有哦如下类型：1）emptyRejectRule 验证不能为空的filter，
在orderer/common/filter/filter.go中定义，2）signFilter，验证签名的filter，在orderer/common/sigfilter/sigfilter.go中定义，这里会使用公共签名策略（common/
policies中定义）来验证进入orderer的Envelope中所携带的签名是否满足要求， 3）sizeFilter，验证大小的filter，在orderer/common/sigfilter/sigfilter.go中定义。
在configtx.yaml中Orderer配置中有需要关于大小多少的配置项，讲在这个filter中验证。4）systemChainFilter，系统链filter，在orderer/multichain/systemchain.go中定义
，作用在于验证一个Envelop是否是一个用于升级的orderer配置的configUpdate类型消息，若是则会提供一个包含configtx工具的committer，在该Envelope写入账本前，执行committer，
，新建一条新配置的链。5）configtxfilter，在orderer/common/configtxfilter/filter.go中定义，作用与systemChainFilter类似，检测Envelope是否是一个Header_config类型的消息，
然后返回一个包含有configManager的工具的committer供之后对消息做进一步操作。
*/
package filter

import (
	"fmt"

	ab "github.com/hyperledger/fabric/protos/common"
)

// Action is used to express the output of a rule
type Action int

const (
	// Accept indicates that the message should be processed
	Accept = iota
	// Reject indicates that the message should not be processed
	Reject
	// Forward indicates that the rule could not determine the correct course of action
	Forward
)

// Rule defines a filter function which accepts, rejects, or forwards (to the next rule) an Envelope
type Rule interface {
	// Apply applies the rule to the given Envelope, replying with the Action to take for the message
	// If the filter Accepts a message, it should provide a committer to use when writing the message to the chain
	Apply(message *ab.Envelope) (Action, Committer)
}

// Committer is returned by postfiltering and should be invoked once the message has been written to the blockchain
type Committer interface {
	// Commit performs whatever action should be performed upon committing of a message
	Commit()

	// Isolated returns whether this transaction should have a block to itself or may be mixed with other transactions
	Isolated() bool
}

type noopCommitter struct{}

func (nc noopCommitter) Commit()        {}
func (nc noopCommitter) Isolated() bool { return false }

// NoopCommitter does nothing on commit and is not isolated
var NoopCommitter = Committer(noopCommitter{})

// EmptyRejectRule rejects empty messages
var EmptyRejectRule = Rule(emptyRejectRule{})

type emptyRejectRule struct{}

func (a emptyRejectRule) Apply(message *ab.Envelope) (Action, Committer) {
	if message.Payload == nil {
		return Reject, nil
	}
	return Forward, nil
}

// AcceptRule always returns Accept as a result for Apply
var AcceptRule = Rule(acceptRule{})

type acceptRule struct{}

func (a acceptRule) Apply(message *ab.Envelope) (Action, Committer) {
	return Accept, NoopCommitter
}

// RuleSet is used to apply a collection of rules
type RuleSet struct {
	rules []Rule
}

// NewRuleSet creates a new RuleSet with the given ordered list of Rules
func NewRuleSet(rules []Rule) *RuleSet {
	return &RuleSet{
		rules: rules,
	}
}

// Apply applies the rules given for this set in order, returning the committer, nil on valid, or nil, err on invalid
func (rs *RuleSet) Apply(message *ab.Envelope) (Committer, error) {
	for _, rule := range rs.rules {
		action, committer := rule.Apply(message)
		switch action {
		case Accept:
			return committer, nil
		case Reject:
			return nil, fmt.Errorf("Rejected by rule: %T", rule)
		default:
		}
	}
	return nil, fmt.Errorf("No matching filter found")
}
