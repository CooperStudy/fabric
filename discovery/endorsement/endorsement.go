/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package endorsement

import (
	"fmt"

	"github.com/hyperledger/fabric/common/chaincode"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/common/graph"
	"github.com/hyperledger/fabric/common/policies"
	"github.com/hyperledger/fabric/common/policies/inquire"
	"github.com/hyperledger/fabric/gossip/api"
	"github.com/hyperledger/fabric/gossip/common"
	. "github.com/hyperledger/fabric/gossip/discovery"
	"github.com/hyperledger/fabric/protos/discovery"
	"github.com/hyperledger/fabric/protos/msp"
	"github.com/pkg/errors"
)

var (
	logger = flogging.MustGetLogger("discovery.endorsement")
)

type principalEvaluator interface {
	// SatisfiesPrincipal returns whether a given peer identity satisfies a certain principal
	// on a given channel
	SatisfiesPrincipal(channel string, identity []byte, principal *msp.MSPPrincipal) error
}

type chaincodeMetadataFetcher interface {
	// ChaincodeMetadata returns the metadata of the chaincode as appears in the ledger,
	// or nil if the channel doesn't exist, or the chaincode isn't found in the ledger
	Metadata(channel string, cc string, loadCollections bool) *chaincode.Metadata
}

type policyFetcher interface {
	// PolicyByChaincode returns a policy that can be inquired which identities
	// satisfy it
	PolicyByChaincode(channel string, cc string) policies.InquireablePolicy
}

type gossipSupport interface {
	// IdentityInfo returns identity information about peers
	IdentityInfo() api.PeerIdentitySet

	// PeersOfChannel returns the NetworkMembers considered alive
	// and also subscribed to the channel given
	PeersOfChannel(common.ChainID) Members

	// Peers returns the NetworkMembers considered alive
	Peers() Members
}

type endorsementAnalyzer struct {
	gossipSupport
	principalEvaluator
	policyFetcher
	chaincodeMetadataFetcher
}

// NewEndorsementAnalyzer constructs an NewEndorsementAnalyzer out of the given support
func NewEndorsementAnalyzer(gs gossipSupport, pf policyFetcher, pe principalEvaluator, mf chaincodeMetadataFetcher) *endorsementAnalyzer {
	fmt.Println("===========NewEndorsementAnalyzer========")
	return &endorsementAnalyzer{
		gossipSupport:            gs,
		policyFetcher:            pf,
		principalEvaluator:       pe,
		chaincodeMetadataFetcher: mf,
	}
}

type peerPrincipalEvaluator func(member NetworkMember, principal *msp.MSPPrincipal) bool

// PeersForEndorsement returns an EndorsementDescriptor for a given set of peers, channel, and chaincode
func (ea *endorsementAnalyzer) PeersForEndorsement(chainID common.ChainID, interest *discovery.ChaincodeInterest) (*discovery.EndorsementDescriptor, error) {
	fmt.Println("==endorsementAnalyzer===PeersForEndorsement===")
	fmt.Println("=========chainID=======",chainID)//[109 121 99 104 97 110 110 101 108]
	fmt.Println("=========interest=======",interest)//chaincodes:<name:"mycc" >
	chanMembership, err := ea.PeersAuthorizedByCriteria(chainID, interest)
	fmt.Println("===========chanMembership================",chanMembership) //[]
	fmt.Println("===========err================",err)//nil
	if err != nil {
		return nil, errors.WithStack(err)
	}

	channelMembersById := chanMembership.ByID()
	fmt.Println("=========channelMembersById=============",channelMembersById)//map[]
	// Choose only the alive messages of those that have joined the channel
	aliveMembership := ea.Peers().Intersect(chanMembership)
	fmt.Println("===============aliveMembership=================",aliveMembership) //[]
	membersById := aliveMembership.ByID()
	fmt.Println("================membersById==============",membersById)//map[]
	// Compute a mapping between the PKI-IDs of members to their identities
	fmt.Println("=============ea.IdentityInfo()=========",ea.IdentityInfo())
	fmt.Println("=============membersById=========",membersById)
	identitiesOfMembers := computeIdentitiesOfMembers(ea.IdentityInfo(), membersById)
	principalsSets, err := ea.computePrincipalSets(chainID, interest)
	if err != nil {
		logger.Warningf("Principal set computation failed: %v", err)
		return nil, errors.WithStack(err)
	}


	return ea.computeEndorsementResponse(&context{
		chaincode:           interest.Chaincodes[0].Name,
		channel:             string(chainID),
		principalsSets:      principalsSets,
		channelMembersById:  channelMembersById,
		aliveMembership:     aliveMembership,
		identitiesOfMembers: identitiesOfMembers,
	})
}

func (ea *endorsementAnalyzer) PeersAuthorizedByCriteria(chainID common.ChainID, interest *discovery.ChaincodeInterest) (Members, error) {
	fmt.Println("==endorsementAnalyzer===PeersAuthorizedByCriteria===")
	fmt.Println("=====chainID====", chainID)
	peersOfChannel := ea.PeersOfChannel(chainID)
	fmt.Println("======peersOfChannel====", peersOfChannel)
	if interest == nil || len(interest.Chaincodes) == 0 {
		return peersOfChannel, nil
	}
	identities := ea.IdentityInfo()
	fmt.Println("====identities======", identities)
	identitiesByID := identities.ByID()
	fmt.Println("====identities======", identitiesByID)
	metadataAndCollectionFilters, err := loadMetadataAndFilters(metadataAndFilterContext{
		identityInfoByID: identitiesByID,
		interest:         interest,
		chainID:          chainID,
		evaluator:        ea,
		fetch:            ea,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	metadata := metadataAndCollectionFilters.md
	// Filter out peers that don't have the chaincode installed on them
	chanMembership := peersOfChannel.Filter(peersWithChaincode(metadata...))
	// Filter out peers that aren't authorized by the collection configs of the chaincode invocation chain
	return chanMembership.Filter(metadataAndCollectionFilters.isMemberAuthorized), nil
}

type context struct {
	chaincode           string
	channel             string
	aliveMembership     Members
	principalsSets      []policies.PrincipalSet
	channelMembersById  map[string]NetworkMember
	identitiesOfMembers memberIdentities
}

func (ea *endorsementAnalyzer) computeEndorsementResponse(ctx *context) (*discovery.EndorsementDescriptor, error) {
	fmt.Println("==endorsementAnalyzer===computeEndorsementResponse===")
	// mapPrincipalsToGroups returns a mapping from principals to their corresponding groups.
	// groups are just human readable representations that mask the principals behind them
	principalGroups := mapPrincipalsToGroups(ctx.principalsSets)
	// principalsToPeersGraph computes a bipartite graph (V1 U V2 , E)
	// such that V1 is the peers, V2 are the principals,
	// and each e=(peer,principal) is in E if the peer satisfies the principal
	satGraph := principalsToPeersGraph(principalAndPeerData{
		members: ctx.aliveMembership,
		pGrps:   principalGroups,
	}, ea.satisfiesPrincipal(ctx.channel, ctx.identitiesOfMembers))

	layouts := computeLayouts(ctx.principalsSets, principalGroups, satGraph)
	if len(layouts) == 0 {
		return nil, errors.New("cannot satisfy any principal combination")
	}

	fmt.Println("==========possibleLayouts===========", layouts)
	fmt.Println("==========satGraph===========", satGraph)
	fmt.Println("==========chanMemberById===========", ctx.channelMembersById)
	fmt.Println("==========idOfMembers===========", ctx.identitiesOfMembers)

	criteria := &peerMembershipCriteria{
		possibleLayouts: layouts,
		satGraph:        satGraph,
		chanMemberById:  ctx.channelMembersById,
		idOfMembers:     ctx.identitiesOfMembers,
	}

	return &discovery.EndorsementDescriptor{
		Chaincode:         ctx.chaincode,
		Layouts:           layouts,
		EndorsersByGroups: endorsersByGroup(criteria),
	}, nil
}

func (ea *endorsementAnalyzer) computePrincipalSets(chainID common.ChainID, interest *discovery.ChaincodeInterest) (policies.PrincipalSets, error) {
	fmt.Println("==endorsementAnalyzer===computePrincipalSets===")
	var inquireablePolicies []policies.InquireablePolicy
	for _, chaincode := range interest.Chaincodes {
		fmt.Println("===========chaincode=========", chaincode)// name:"mycc"
		fmt.Println("==========string(chainID)=======", string(chainID))//mychannel
		fmt.Println("======chaincode.Name====", chaincode.Name) //mycc
		pol := ea.PolicyByChaincode(string(chainID), chaincode.Name)
		fmt.Println("======pol=======",pol)//&{[] [[A B] [C] [A D]]}
		if pol == nil {
			logger.Debug("Policy for chaincode '", chaincode, "'doesn't exist")
			return nil, errors.New("policy not found")
		}
		inquireablePolicies = append(inquireablePolicies, pol)
	}

	fmt.Println("=========inquireablePolicies===========",inquireablePolicies)
	var cpss []inquire.ComparablePrincipalSets

	for _, policy := range inquireablePolicies {
		fmt.Println("==========policy=======",policy)
		var cmpsets inquire.ComparablePrincipalSets
		for _, ps := range policy.SatisfiedBy() {
			fmt.Println("===========")//&{[] [[A B] [C] [A D]]}
			cps := inquire.NewComparablePrincipalSet(ps)
			if cps == nil {
				return nil, errors.New("failed creating a comparable principal set")
			}
			cmpsets = append(cmpsets, cps)
		}
		if len(cmpsets) == 0 {
			return nil, errors.New("chaincode isn't installed on sufficient organizations required by the endorsement policy")
		}
		cpss = append(cpss, cmpsets)
	}

	cps, err := mergePrincipalSets(cpss)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return cps.ToPrincipalSets(), nil
}

type metadataAndFilterContext struct {
	chainID          common.ChainID
	interest         *discovery.ChaincodeInterest
	fetch            chaincodeMetadataFetcher
	identityInfoByID map[string]api.PeerIdentityInfo
	evaluator        principalEvaluator
}

// metadataAndColFilter holds metadata and member filters
type metadataAndColFilter struct {
	md                 []*chaincode.Metadata
	isMemberAuthorized memberFilter
}

func loadMetadataAndFilters(ctx metadataAndFilterContext) (*metadataAndColFilter, error) {
	fmt.Println("==loadMetadataAndFilters===")
	var metadata []*chaincode.Metadata
	var filters []identityFilter

	for _, chaincode := range ctx.interest.Chaincodes {
		fmt.Println("===========chaincode===========", chaincode)
		ccMD := ctx.fetch.Metadata(string(ctx.chainID), chaincode.Name, len(chaincode.CollectionNames) > 0)
		if ccMD == nil {
			return nil, errors.Errorf("No metadata was found for chaincode %s in channel %s", chaincode.Name, string(ctx.chainID))
		}
		metadata = append(metadata, ccMD)
		if len(chaincode.CollectionNames) == 0 {
			continue
		}
		principalSetByCollections, err := principalsFromCollectionConfig(ccMD.CollectionsConfig)
		if err != nil {
			logger.Warningf("Failed initializing collection filter for chaincode %s: %v", chaincode.Name, err)
			return nil, errors.WithStack(err)
		}
		filter, err := principalSetByCollections.toIdentityFilter(string(ctx.chainID), ctx.evaluator, chaincode)
		if err != nil {
			logger.Warningf("Failed computing collection principal sets for chaincode %s due to %v", chaincode.Name, err)
			return nil, errors.WithStack(err)
		}
		filters = append(filters, filter)
	}

	return computeFiltersWithMetadata(filters, metadata, ctx.identityInfoByID), nil
}

func computeFiltersWithMetadata(filters identityFilters, metadata []*chaincode.Metadata, identityInfoByID map[string]api.PeerIdentityInfo) *metadataAndColFilter {
	fmt.Println("==computeFiltersWithMetadata===")
	fmt.Println("===len(filters)=====", len(filters))
	if len(filters) == 0 {
		return &metadataAndColFilter{
			md:                 metadata,
			isMemberAuthorized: noopMemberFilter,
		}
	}
	filter := filters.combine().toMemberFilter(identityInfoByID)
	return &metadataAndColFilter{
		isMemberAuthorized: filter,
		md:                 metadata,
	}
}

// identityFilter accepts or rejects peer identities
type identityFilter func(api.PeerIdentityType) bool

// identityFilters aggregates multiple identityFilters
type identityFilters []identityFilter

// memberFilter accepts or rejects NetworkMembers
type memberFilter func(member NetworkMember) bool

// noopMemberFilter accepts every NetworkMember
func noopMemberFilter(_ NetworkMember) bool {
	return true
}

// combine combines all identityFilters into a single identityFilter which only accepts identities
// which all the original filters accept
func (filters identityFilters) combine() identityFilter {
	fmt.Println("==identityFilters===combine===")
	return func(identity api.PeerIdentityType) bool {
		for _, f := range filters {
			if !f(identity) {
				return false
			}
		}
		return true
	}
}

// toMemberFilter converts this identityFilter to a memberFilter based on the given mapping
// from PKI-ID as strings, to PeerIdentityInfo which holds the peer identities
func (idf identityFilter) toMemberFilter(identityInfoByID map[string]api.PeerIdentityInfo) memberFilter {
	fmt.Println("==========identityFilter=====toMemberFilter===============")
	return func(member NetworkMember) bool {

		identity, exists := identityInfoByID[string(member.PKIid)]
		fmt.Println("===============member.PKIid==============", string(member.PKIid))
		fmt.Println("=======identity============", identity)
		fmt.Println("=======exists============", exists)
		if !exists {
			return false
		}
		return idf(identity.Identity)
	}
}

func (ea *endorsementAnalyzer) satisfiesPrincipal(channel string, identitiesOfMembers memberIdentities) peerPrincipalEvaluator {
	fmt.Println("==========endorsementAnalyzer=====satisfiesPrincipal===============")
	return func(member NetworkMember, principal *msp.MSPPrincipal) bool {
		err := ea.SatisfiesPrincipal(channel, identitiesOfMembers.identityByPKIID(member.PKIid), principal)
		if err == nil {
			// TODO: log the principals in a human readable form
			logger.Debug(member, "satisfies principal", principal)
			return true
		}
		logger.Debug(member, "doesn't satisfy principal", principal, ":", err)
		return false
	}
}

type peerMembershipCriteria struct {
	satGraph        *principalPeerGraph
	idOfMembers     memberIdentities
	chanMemberById  map[string]NetworkMember
	possibleLayouts layouts
}

// endorsersByGroup computes a map from groups -> peers.
// Each group included, is found in some layout, which means
// that there is some principal combination that includes the corresponding
// group.
// This means that if a group isn't included in the result, there is no
// principal combination (that includes the principal corresponding to the group),
// such that there are enough peers to satisfy the principal combination.
func endorsersByGroup(criteria *peerMembershipCriteria) map[string]*discovery.Peers {
	fmt.Println("==========endorsersByGroup==============")
	satGraph := criteria.satGraph
	fmt.Println("===========satGraph := criteria.satGraph============", satGraph)
	idOfMembers := criteria.idOfMembers
	fmt.Println("===========idOfMembers===========", idOfMembers)
	chanMemberById := criteria.chanMemberById
	fmt.Println("===========chanMemberById===========", chanMemberById)
	includedGroups := criteria.possibleLayouts.groupsSet()
	fmt.Println("===========includedGroups===========", includedGroups)
	res := make(map[string]*discovery.Peers)
	// Map endorsers to their corresponding groups.
	// Iterate the principals, and put the peers into each group that corresponds with a principal vertex
	for grp, principalVertex := range satGraph.principalVertices {
		fmt.Println("=======grp=========", grp)
		fmt.Println("=======principalVertex=========", principalVertex)
		if _, exists := includedGroups[grp]; !exists {
			// If the current group is not found in any layout, skip the corresponding principal
			// 如果在任何layout中都没有找到当前group，则跳过相应的principal
			continue
		}
		peerList := &discovery.Peers{}
		res[grp] = peerList
		for _, peerVertex := range principalVertex.Neighbors() {
			fmt.Println("===========peerVertex=============", peerVertex)
			member := peerVertex.Data.(NetworkMember)
			fmt.Println("===============member===========", member)
			fmt.Println("==========Identity=========", idOfMembers.identityByPKIID(member.PKIid))
			fmt.Println("==========StateInfo=========", chanMemberById[string(member.PKIid)].Envelope)
			fmt.Println("==========MembershipInfo=========", member.Envelope)
			peerList.Peers = append(peerList.Peers, &discovery.Peer{
				Identity:       idOfMembers.identityByPKIID(member.PKIid),
				StateInfo:      chanMemberById[string(member.PKIid)].Envelope,
				MembershipInfo: member.Envelope,
			})
		}
	}
	return res
}

// computeLayouts computes all possible principal combinations
// that can be used to satisfy the endorsement policy, given a graph
// of available peers that maps each peer to a principal it satisfies.
// Each such a combination is called a layout, because it maps
// a group (alias for a principal) to a threshold of peers that need to endorse,
// and that satisfy the corresponding principal.
// computeLayouts 计算所有可能的principal组合
// 可以用来满足背书策略，给定一个图
// 将每个对等点映射到它满足的主体的可用对等点。
// 每个这样的组合称为一个布局，因为它映射
// 一组（principal的别名）到需要背书的阈值，
// 并且满足相应的principal。
func computeLayouts(principalsSets []policies.PrincipalSet, principalGroups principalGroupMapper, satGraph *principalPeerGraph) []*discovery.Layout {
	fmt.Println("==========computeLayouts==============")
	fmt.Println("====================非常重要===========")
	fmt.Println("=========principalsSets===========", principalsSets)
	//[[principal:"\n\001A"  principal:"\n\001B" ] [principal:"\n\001C" ] [principal:"\n\001A"  principal:"\n\001D" ]]
	fmt.Println("=========principalGroups===================", principalGroups)
	/*
	========principalGroups=================== map[{0
	B}:G0 {0
	C}:G1 {0
	D}:G2 {0
	A}:G3]
	 */
	fmt.Println("=========satGraph==================", *satGraph)
	//=========satGraph================== {[] map[G1:0xc00037cdb0 G2:0xc00037ce10 G3:0xc00037ce70 G0:0xc00037ced0]}
	var layouts []*discovery.Layout
	// principalsSets is a collection of combinations of principals,
	// such that each combination (given enough peers) satisfies the endorsement policy.
	for _, principalSet := range principalsSets {
		fmt.Println("=======principalSet=======", principalSet)
		//[principal:"\n\001A"  principal:"\n\001B" ]      // principal:"\n\001C"
		layout := &discovery.Layout{
			QuantitiesByGroup: make(map[string]uint32),
		}
		// Since principalsSet has repetitions, we first
		// compute a mapping from the principal to repetitions in the set.
		/*
			// 由于 principalsSet 有重复，我们首先
			// 计算从主体到集合中重复的映射。
		*/
		for principal, plurality := range principalSet.UniqueSet() {
			fmt.Println("========principal===", principal)//principal:"\n\001A" principal:"\n\001B"    principal:"\n\001C"
			fmt.Println("========plurality=======", plurality)//1 1  1
			key := principalKey{
				cls:       int32(principal.PrincipalClassification),
				principal: string(principal.Principal),
			}
			// We map the principal to a group, which is an alias for the principal.
			// 我们将principal映射到一个group，这是principal的别名。
			a := principalGroups.group(key)
			fmt.Println("==========principalGroups.group(key)==========", a)//G3  G0   G1
			fmt.Println("==========uint32(plurality)==========", uint32(plurality))//1 1
			layout.QuantitiesByGroup[a] = uint32(plurality)
			fmt.Println("=========layout.QuantitiesByGroup=======================", layout.QuantitiesByGroup)//map[G3:1 G0:1]  map[G1:1] map[G2:1 G3:1]
		}
		// Check that the layout can be satisfied with the current known peers
		// 检查layout是否可以满足当前已知的peers
		// This is done by iterating the current layout, and ensuring that
		// 这是通过迭代当前layout来完成的，并确保
		// each principal vertex is connected to at least <plurality> peer vertices.
		//每个principal vertex至少连接到 <plurality> peer vertices

		b := isLayoutSatisfied(layout.QuantitiesByGroup, satGraph)
		fmt.Println("========isLayoutSatisfied(layout.QuantitiesByGroup, satGraph)========", b)
		if b {
			// If so, then add the layout to the layouts, since we have enough peers to satisfy the principal combination
			//如果是，则将layout添加到layouts中，因为我们有足够的peers来满足principal combination
			layouts = append(layouts, layout)
		}
	}
	return layouts
}

func isLayoutSatisfied(layout map[string]uint32, satGraph *principalPeerGraph) bool {
	fmt.Println("==========isLayoutSatisfied==============")
	for grp, plurality := range layout {
		fmt.Println("=============grp===========", grp) //G3
		fmt.Println("=============plurality===========", plurality)//1
		// Do we have more than <plurality> peers connected to the principal?
		a := satGraph.principalVertices[grp]
		b := satGraph.principalVertices[grp].Neighbors()

		fmt.Println("=============satGraph.principalVertices[grp]===============", a) //&{G3 principal:"\n\001A"  map[]} &{G3 principal:"\n\001A"  map[]} &{G1 principal:"\n\001C"  map[]}  &{G2 principal:"\n\001D"  map[]}
		fmt.Println("=============satGraph.principalVertices[grp].Neighbors()===============", b)//[]
		fmt.Println("=============satGraph.principalVertices[grp].Neighbors() len===============", len(satGraph.principalVertices[grp].Neighbors()))//0
		if len(satGraph.principalVertices[grp].Neighbors()) < int(plurality) {
			fmt.Println("==============================len(satGraph.principalVertices[grp].Neighbors()) < int(plurality)==========")
			return false
		}
		fmt.Println("=============满足条件==================")
	}
	return true
}

type principalPeerGraph struct {
	peerVertices      []*graph.Vertex
	principalVertices map[string]*graph.Vertex
}

type principalAndPeerData struct {
	members Members
	pGrps   principalGroupMapper
}

func principalsToPeersGraph(data principalAndPeerData, satisfiesPrincipal peerPrincipalEvaluator) *principalPeerGraph {
	fmt.Println("==========principalsToPeersGraph==============")
	// Create the peer vertices
	peerVertices := make([]*graph.Vertex, len(data.members))
	for i, member := range data.members {
		fmt.Println("===========i",)
		fmt.Println("===========member,member")
		fmt.Println("============string(member.PKIid)==============",string(member.PKIid))
		fmt.Println("============member=============",member)
		peerVertices[i] = graph.NewVertex(string(member.PKIid), member)

	}
	fmt.Println("==============peerVertices=======",peerVertices)
	// Create the principal vertices
	principalVertices := make(map[string]*graph.Vertex)
	for pKey, grp := range data.pGrps {
		fmt.Println("======pKey==",pKey)
		fmt.Println("======grp==",grp)
		principalVertices[grp] = graph.NewVertex(grp, pKey.toPrincipal())
	}

	// Connect principals and peers
	for _, principalVertex := range principalVertices {
		for _, peerVertex := range peerVertices {
			// If the current peer satisfies the principal, connect their corresponding vertices with an edge
			principal := principalVertex.Data.(*msp.MSPPrincipal)
			member := peerVertex.Data.(NetworkMember)
			if satisfiesPrincipal(member, principal) {
				peerVertex.AddNeighbor(principalVertex)
			}
		}
	}
	return &principalPeerGraph{
		peerVertices:      peerVertices,
		principalVertices: principalVertices,
	}
}

func mapPrincipalsToGroups(principalsSets []policies.PrincipalSet) principalGroupMapper {
	fmt.Println("==========mapPrincipalsToGroups==============")
	fmt.Println("============principalsSets========", principalsSets)
	//[[principal:"\n\001A"  principal:"\n\001B" ] [principal:"\n\001C" ] [principal:"\n\001A"  principal:"\n\001D" ]]
	groupMapper := make(principalGroupMapper)
	totalPrincipals := make(map[principalKey]struct{})
	for _, principalSet := range principalsSets {
		fmt.Println("=====principalSet===", principalSet)//[principal:"\n\001A"  principal:"\n\001B" ]
		for _, principal := range principalSet {
			fmt.Println("=====principal===", principal)
			/*
			=====principal=== principal:"\n\001A"
			=====principal=== principal:"\n\001B"
			*/
			totalPrincipals[principalKey{
				principal: string(principal.Principal),
				cls:       int32(principal.PrincipalClassification),
			}] = struct{}{}
		}
	}
	for principal := range totalPrincipals {
		groupMapper.group(principal)
	}
	return groupMapper
}

type memberIdentities map[string]api.PeerIdentityType

func (m memberIdentities) identityByPKIID(id common.PKIidType) api.PeerIdentityType {
	fmt.Println("=======memberIdentities===identityByPKIID==============")
	return m[string(id)]
}

func computeIdentitiesOfMembers(identitySet api.PeerIdentitySet, members map[string]NetworkMember) memberIdentities {
	fmt.Println("=======computeIdentitiesOfMembers=============")
	fmt.Println("=================identitySet===================",identitySet)//=================identitySet=================== [{7030 [10 1 65 18 2 112 48] [65]} {7031 [10 1 65 18 2 112 49] [65]} {7032 [10 1 66 18 2 112 50] [66]} {7033 [10 1 66 18 2 112 51] [66]} {7034 [10 1 67 18 2 112 52] [67]} {7035 [10 1 67 18 2 112 53] [67]} {7036 [10 1 68 18 2 112 54] [68]} {7037 [10 1 68 18 2 112 55] [68]} {7038 [10 1 65 18 2 112 56] [65]} {7039 [10 1 65 18 2 112 57] [65]} {703130 [10 1 66 18 3 112 49 48] [66]} {703131 [10 1 66 18 3 112 49 49] [66]} {703132 [10 1 67 18 3 112 49 50] [67]} {703133 [10 1 67 18 3 112 49 51] [67]} {703134 [10 1 68 18 3 112 49 52] [68]} {703135 [10 1 68 18 3 112 49 53] [68]}]
	fmt.Println("=================members===================",members)//map[]

	identitiesByPKIID := make(map[string]api.PeerIdentityType)
	identitiesOfMembers := make(map[string]api.PeerIdentityType, len(members))
	for _, identity := range identitySet {
		fmt.Println("================identity================",identity) // {7030 [10 1 65 18 2 112 48] [65]}
		fmt.Println("===============string(identity.PKIId)================",string(identity.PKIId))//p0
		fmt.Println("===============identity.Identity================",identity.Identity)//[10 1 65 18 2 112 48]
		identitiesByPKIID[string(identity.PKIId)] = identity.Identity
	}
	fmt.Println("===============identitiesByPKIID================",identitiesByPKIID)
	//map[p8:[10 1 65 18 2 112 56] p12:[10 1 67 18 3 112 49 50] p0:[10 1 65 18 2 112 48] p9:[10 1 65 18 2 112 57] p6:[10 1 68 18 2 112 54] p7:[10 1 68 18 2 112 55] p13:[10 1 67 18 3 112 49 51] p14:[10 1 68 18 3 112 49 52] p3:[10 1 66 18 2 112 51] p2:[10 1 66 18 2 112 50] p4:[10 1 67 18 2 112 52] p5:[10 1 67 18 2 112 53] p10:[10 1 66 18 3 112 49 48] p11:[10 1 66 18 3 112 49 49] p15:[10 1 68 18 3 112 49 53] p1:[10 1 65 18 2 112 49]]
	for _, member := range members {
		fmt.Println("==========member===========",member)
		if identity, exists := identitiesByPKIID[string(member.PKIid)]; exists {
			fmt.Println("============identity==========================",identity)
			fmt.Println("=========string(member.PKIid)========================",string(member.PKIid))

			identitiesOfMembers[string(member.PKIid)] = identity
		}
	}
	fmt.Println("==========identitiesOfMembers==============",identitiesOfMembers)
	return identitiesOfMembers
}

// principalGroupMapper maps principals to names of groups
type principalGroupMapper map[principalKey]string

func (mapper principalGroupMapper) group(principal principalKey) string {
	fmt.Println("=======principalGroupMapper===group==========")
	if grp, exists := mapper[principal]; exists {
		fmt.Println("======grp======", grp) //G3  G0 G1
		fmt.Println("======exists======", exists)//true true
		return grp
	}
	grp := fmt.Sprintf("G%d", len(mapper))
	fmt.Println("=====grp====", grp)//G0 G1 G2 G3 G0
	mapper[principal] = grp
	fmt.Println("======mapper====", mapper)
	/*
	map[{0
	B}:G0]

	=====grp==== G1
	======mapper==== map[{0
	B}:G0 {0
	C}:G1]
	 */
	/*
	map[{0 B}:G0 {0 C}:G1] {0 D}:G2 {0 A}:G3


	=======principalGroupMapper===group==========
	=====grp==== G2
	======mapper==== map[{0
	B}:G0 {0
	C}:G1 {0
	D}:G2]


	=====grp==== G3
	======mapper==== map[{0
	B}:G0 {0
	C}:G1 {0
	D}:G2 {0
	A}:G3]
	 */
	return grp
}

type principalKey struct {
	cls       int32
	principal string
}

func (pk principalKey) toPrincipal() *msp.MSPPrincipal {
	fmt.Println("=======principalKey===toPrincipal==========")
	fmt.Println("=========pk.cls=======", pk.cls)//0
	fmt.Println("========msp.MSPPrincipal_Classification(pk.cls)======", msp.MSPPrincipal_Classification(pk.cls))//ROLE

	fmt.Println("=========pk.principal======", pk.principal)//C D A B
	return &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_Classification(pk.cls),
		Principal:               []byte(pk.principal),
	}
}

// layouts is an aggregation of several layouts
type layouts []*discovery.Layout

// groupsSet returns a set of groups that the layouts contain
func (l layouts) groupsSet() map[string]struct{} {
	fmt.Println("=======layouts===groupsSet==========")
	m := make(map[string]struct{})
	for _, layout := range l {
		for grp := range layout.QuantitiesByGroup {
			m[grp] = struct{}{}
		}
	}
	return m
}

func peersWithChaincode(metadata ...*chaincode.Metadata) func(member NetworkMember) bool {
	fmt.Println("=======peersWithChaincode==========")
	fmt.Println("======metadata============", metadata)
	return func(member NetworkMember) bool {
		fmt.Println("====member.Properties=====", member.Properties)
		if member.Properties == nil {
			return false
		}
		for _, ccMD := range metadata {
			fmt.Println("====ccMD=======", ccMD)
			var found bool
			for _, cc := range member.Properties.Chaincodes {
				fmt.Println("===========cc=====", cc)
				fmt.Println("===========cc.Name=====", cc.Name)
				fmt.Println("===========ccMD.Name=====", ccMD.Name)
				fmt.Println("===========cc.Version=====", cc.Version)
				fmt.Println("===========ccMD.Version=====", ccMD.Version)
				if cc.Name == ccMD.Name && cc.Version == ccMD.Version {
					found = true
				}
			}
			//fmt.Println("====found===", found)
			if !found {
				return false
			}
		}
		return true
	}
}

func mergePrincipalSets(cpss []inquire.ComparablePrincipalSets) (inquire.ComparablePrincipalSets, error) {
	fmt.Println("=======mergePrincipalSets==========")
	// Obtain the first ComparablePrincipalSet first
	var cps inquire.ComparablePrincipalSets
	cps, cpss, err := popComparablePrincipalSets(cpss)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	for _, cps2 := range cpss {
		cps = inquire.Merge(cps, cps2)
	}
	return cps, nil
}

func popComparablePrincipalSets(sets []inquire.ComparablePrincipalSets) (inquire.ComparablePrincipalSets, []inquire.ComparablePrincipalSets, error) {
	fmt.Println("=======popComparablePrincipalSets==========")
	if len(sets) == 0 {
		return nil, nil, errors.New("no principal sets remained after filtering")
	}
	cps, cpss := sets[0], sets[1:]
	fmt.Println("====sets", sets)  //sets [[[A.MEMBER, B.MEMBER] [C.MEMBER] [A.MEMBER, D.MEMBER]]]
	fmt.Println("=====cps", cps)//[[A.MEMBER, B.MEMBER] [C.MEMBER] [A.MEMBER, D.MEMBER]]
	fmt.Println("=====cpss", cpss) //[]
	return cps, cpss, nil
}
