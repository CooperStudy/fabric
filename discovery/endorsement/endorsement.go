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
	logger.Info("===========NewEndorsementAnalyzer========")
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
	logger.Info("==endorsementAnalyzer===PeersForEndorsement===")
	logger.Info("=========chainID=======",chainID)//[109 121 99 104 97 110 110 101 108]
	logger.Info("=========interest=======",interest)//chaincodes:<name:"mycc" >
	chanMembership, err := ea.PeersAuthorizedByCriteria(chainID, interest)
	logger.Info("===========chanMembership================",chanMembership) //[]
	/*
	[Endpoint: peer1.org1.example.com:7051,
	InternalEndpoint: peer1.org1.example.com:7051,
	PKI-ID: 500f76e570c22e91343ac9c28db55d6948faa44da710db6fa7971641d4ea285b,
	Metadata:  Endpoint: ,
	InternalEndpoint: ,
	PKI-ID: 6f24a285101adea8c6350decbca2c731f07978a32a5ca876c3f2cca7b5b3e090,
	Metadata: ]

	 */
	logger.Info("===========err================",err)//nil
	if err != nil {
		return nil, errors.WithStack(err)
	}

	channelMembersById := chanMembership.ByID()
	logger.Info("=========channelMembersById=============",channelMembersById)//map[]
	// Choose only the alive messages of those that have joined the channel
	aliveMembership := ea.Peers().Intersect(chanMembership)
	logger.Info("===============aliveMembership=================",aliveMembership) //[]
	membersById := aliveMembership.ByID()
	logger.Info("================membersById==============",membersById)//map[]
	// Compute a mapping between the PKI-IDs of members to their identities
	logger.Info("=============ea.IdentityInfo()=========",ea.IdentityInfo())
	logger.Info("=============membersById=========",membersById)
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
	logger.Info("==endorsementAnalyzer===PeersAuthorizedByCriteria===")
	//logger.Info("=====chainID====", chainID)
	peersOfChannel := ea.PeersOfChannel(chainID)
	logger.Info("======peersOfChannel====", peersOfChannel)
	/*
	 [Endpoint: peer1.org1.example.com:7051, InternalEndpoint: peer1.org1.example.com:7051, PKI-ID: 500f76e570c22e91343ac9c28db55d6948faa44da710db6fa7971641d4ea285b, Metadata:  Endpoint: , InternalEndpoint: , PKI-ID: 6f24a285101adea8c6350decbca2c731f07978a32a5ca876c3f2cca7b5b3e090, Metadata: ]
	*/
	if interest == nil || len(interest.Chaincodes) == 0 {
		return peersOfChannel, nil
	}
	identities := ea.IdentityInfo()
	logger.Info("====identities======", identities)
	identitiesByID := identities.ByID()
	logger.Info("====identities======", identitiesByID)
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
	logger.Info("==endorsementAnalyzer===computeEndorsementResponse===")
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

	logger.Info("==========possibleLayouts===========", layouts)
	// [quantities_by_group:<key:"G0" value:1 > ]
	logger.Info("==========satGraph===========", satGraph)
	/*
	 &{[0xc0024118f0 0xc0�]iH��M��o��A��([:Endpoint: peer1.org1.example.com:7051, InternalEndpoint: peer1.org1.example.com:7051, PKI-ID: 500f76e570c22e91343ac9c28db55d6948faa44d켢�1�yx�*\�v��̧����:Endpoint: , InternalEndpoint�]iH��M��o��A��([:[10 7 79 114 103 49 77 83 80 18 166 6 45 45 45 45 45 66 69 71 73 78 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10 77 73 73 67 74 122 67 67 65 99 54 103 65 119 73 66 65 103 73 81 81 118 65 82 82 87 72 53 86 109 81 69 112 108 89 70 53 98 119 80 102 106 65 75 66 103 103 113 104 107 106 79 80 81 81 68 65 106 66 122 77 81 115 119 10 67 81 89 68 86 81 81 71 69 119 74 86 85 122 69 84 77 66 69 71 65 49 85 69 67 66 77 75 81 50 70 115 97 87 90 118 99 109 53 112 89 84 69 87 77 66 81 71 65 49 85 69 66 120 77 78 85 50 70 117 73 69 90 121 10 89 87 53 106 97 88 78 106 98 122 69 90 77 66 99 71 65 49 85 69 67 104 77 81 98 51 74 110 77 83 53 108 101 71 70 116 99 71 120 108 76 109 78 118 98 84 69 99 77 66 111 71 65 49 85 69 65 120 77 84 89 50 69 117 10 98 51 74 110 77 83 53 108 101 71 70 116 99 71 120 108 76 109 78 118 98 84 65 101 70 119 48 121 77 84 69 120 77 84 85 119 79 84 73 51 77 68 66 97 70 119 48 122 77 84 69 120 77 84 77 119 79 84 73 51 77 68 66 97 10 77 71 111 120 67 122 65 74 66 103 78 86 66 65 89 84 65 108 86 84 77 82 77 119 69 81 89 68 86 81 81 73 69 119 112 68 89 87 120 112 90 109 57 121 98 109 108 104 77 82 89 119 70 65 89 68 86 81 81 72 69 119 49 84 10 89 87 52 103 82 110 74 104 98 109 78 112 99 50 78 118 77 81 48 119 67 119 89 68 86 81 81 76 69 119 82 119 90 87 86 121 77 82 56 119 72 81 89 68 86 81 81 68 69 120 90 119 90 87 86 121 77 83 53 118 99 109 99 120 10 76 109 86 52 89 87 49 119 98 71 85 117 89 50 57 116 77 70 107 119 69 119 89 72 75 111 90 73 122 106 48 67 65 81 89 73 75 111 90 73 122 106 48 68 65 81 99 68 81 103 65 69 74 73 85 48 73 70 108 105 66 71 67 55 10 57 98 77 49 116 74 83 56 69 50 82 112 65 98 107 81 112 114 122 79 113 90 111 84 52 75 120 103 77 68 111 53 68 80 67 81 106 55 72 67 69 108 107 65 47 103 109 78 86 122 69 50 84 73 118 74 122 68 75 115 84 43 86 67 10 87 50 79 84 66 87 85 78 104 75 78 78 77 69 115 119 68 103 89 68 86 82 48 80 65 81 72 47 66 65 81 68 65 103 101 65 77 65 119 71 65 49 85 100 69 119 69 66 47 119 81 67 77 65 65 119 75 119 89 68 86 82 48 106 10 66 67 81 119 73 111 65 103 75 50 65 49 104 114 82 54 113 120 108 50 69 66 57 56 119 55 74 102 50 121 109 47 53 116 103 49 118 103 97 101 88 101 74 51 102 76 98 87 76 50 81 119 67 103 89 73 75 111 90 73 122 106 48 69 10 65 119 73 68 82 119 65 119 82 65 73 103 74 47 85 65 115 55 117 68 103 73 103 111 81 103 84 69 89 77 103 115 54 101 83 118 101 113 80 72 75 76 82 117 89 47 48 99 67 105 104 51 105 69 56 67 73 66 107 119 111 87 120 85 10 89 76 112 88 66 100 113 99 55 109 81 104 77 67 113 57 116 102 113 74 51 53 106 43 54 88 71 89 104 120 112 105 71 71 105 76 1켢�1�yx�*\�v��̧����:[10 7 79 114 103 49 77 83 80 18 170 6 45 45 45 45 45 66 69 71 73 78 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10 77 73 73 67 75 68 67 67 65 99 43 103 65 119 73 66 65 103 73 82 65 78 75 57 122 99 52 110 81 75 82 69 86 86 78 52 56 122 88 78 116 77 111 119 67 103 89 73 75 111 90 73 122 106 48 69 65 119 73 119 99 122 69 76 10 77 65 107 71 65 49 85 69 66 104 77 67 86 86 77 120 69 122 65 82 66 103 78 86 66 65 103 84 67 107 78 104 98 71 108 109 98 51 74 117 97 87 69 120 70 106 65 85 66 103 78 86 66 65 99 84 68 86 78 104 98 105 66 71 10 99 109 70 117 89 50 108 122 89 50 56 120 71 84 65 88 66 103 78 86 66 65 111 84 69 71 57 121 90 122 69 117 90 88 104 104 98 88 66 115 90 83 53 106 98 50 48 120 72 68 65 97 66 103 78 86 66 65 77 84 69 50 78 104 10 76 109 57 121 90 122 69 117 90 88 104 104 98 88 66 115 90 83 53 106 98 50 48 119 72 104 99 78 77 106 69 120 77 84 69 49 77 68 107 121 78 122 65 119 87 104 99 78 77 122 69 120 77 84 69 122 77 68 107 121 78 122 65 119 10 87 106 66 113 77 81 115 119 67 81 89 68 86 81 81 71 69 119 74 86 85 122 69 84 77 66 69 71 65 49 85 69 67 66 77 75 81 50 70 115 97 87 90 118 99 109 53 112 89 84 69 87 77 66 81 71 65 49 85 69 66 120 77 78 10 85 50 70 117 73 69 90 121 89 87 53 106 97 88 78 106 98 122 69 78 77 65 115 71 65 49 85 69 67 120 77 69 99 71 86 108 99 106 69 102 77 66 48 71 65 49 85 69 65 120 77 87 99 71 86 108 99 106 65 117 98 51 74 110 10 77 83 53 108 101 71 70 116 99 71 120 108 76 109 78 118 98 84 66 90 77 66 77 71 66 121 113 71 83 77 52 57 65 103 69 71 67 67 113 71 83 77 52 57 65 119 69 72 65 48 73 65 66 74 86 107 51 65 83 72 80 80 82 47 10 69 49 67 107 56 67 54 110 83 86 99 115 114 118 97 75 109 111 115 106 103 101 98 72 98 70 74 51 85 82 56 73 110 106 50 117 115 100 67 107 97 104 119 111 107 86 75 105 99 70 104 107 98 50 117 78 116 107 79 74 77 98 120 48 10 76 114 66 69 84 76 70 49 122 55 101 106 84 84 66 76 77 65 52 71 65 49 85 100 68 119 69 66 47 119 81 69 65 119 73 72 103 68 65 77 66 103 78 86 72 82 77 66 65 102 56 69 65 106 65 65 77 67 115 71 65 49 85 100 10 73 119 81 107 77 67 75 65 73 67 116 103 78 89 97 48 101 113 115 90 100 104 65 102 102 77 79 121 88 57 115 112 118 43 98 89 78 98 52 71 110 108 51 105 100 51 121 50 49 105 57 107 77 65 111 71 67 67 113 71 83 77 52 57 10 66 65 77 67 65 48 99 65 77 69 81 67 73 66 78 111 84 82 120 89 51 52 51 120 82 110 51 109 73 120 82 100 106 105 79 106 85 106 113 76 74 114 56 56 84 76 88 102 65 80 70 122 48 73 98 108 65 105 66 81 66 57 53 53 10 66 120 77 68 79 51 56 54 106 75 76 99 73 114 53 115 51 74 53 68 122 55 115 81 113 121 111 120 85 90 105 121 66 80 116 51 79 103 61 61 10 45 45 45 45 45 69 78 68 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10]]
	*/
	logger.Info("==========chanMemberById===========", ctx.channelMembersById)
	/*

	 */
	logger.Info("==========idOfMembers===========", ctx.identitiesOfMembers)

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
	logger.Info("==endorsementAnalyzer===computePrincipalSets===")
	var inquireablePolicies []policies.InquireablePolicy
	for _, chaincode := range interest.Chaincodes {
		logger.Info("===========chaincode=========", chaincode)// name:"mycc"
		logger.Info("==========string(chainID)=======", string(chainID))//mychannel
		logger.Info("======chaincode.Name====", chaincode.Name) //mycc
		pol := ea.PolicyByChaincode(string(chainID), chaincode.Name)
		logger.Info("======pol=======",pol)//&{[] [[A B] [C] [A D]]}
		if pol == nil {
			logger.Debug("Policy for chaincode '", chaincode, "'doesn't exist")
			return nil, errors.New("policy not found")
		}
		inquireablePolicies = append(inquireablePolicies, pol)
	}

	logger.Info("=========inquireablePolicies===========",inquireablePolicies)
	var cpss []inquire.ComparablePrincipalSets

	for _, policy := range inquireablePolicies {
		logger.Info("==========policy=======",policy)
		var cmpsets inquire.ComparablePrincipalSets
		for _, ps := range policy.SatisfiedBy() {
			logger.Info("===========")//&{[] [[A B] [C] [A D]]}
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
	logger.Info("==loadMetadataAndFilters===")
	var metadata []*chaincode.Metadata
	var filters []identityFilter

	for _, chaincode := range ctx.interest.Chaincodes {
		logger.Info("===========chaincode===========", chaincode)
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
	logger.Info("==computeFiltersWithMetadata===")
	logger.Info("===len(filters)=====", len(filters))
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
	logger.Info("==identityFilters===combine===")
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
	logger.Info("==========identityFilter=====toMemberFilter===============")
	return func(member NetworkMember) bool {

		identity, exists := identityInfoByID[string(member.PKIid)]
		logger.Info("===============member.PKIid==============", string(member.PKIid))
		logger.Info("=======identity============", identity)
		logger.Info("=======exists============", exists)
		if !exists {
			return false
		}
		return idf(identity.Identity)
	}
}

func (ea *endorsementAnalyzer) satisfiesPrincipal(channel string, identitiesOfMembers memberIdentities) peerPrincipalEvaluator {
	logger.Info("==========endorsementAnalyzer=====satisfiesPrincipal===============")
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
	logger.Info("==========endorsersByGroup==============")
	satGraph := criteria.satGraph
	logger.Info("===========satGraph := criteria.satGraph============", satGraph)
	/*
	�]iH��M��o��A��([:[10 7 79 114 103 49 77 83 80 18 166 6 45 45 45 45 45 66 69 71 73 78 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10 77 73 73 67 74 122 67 67 65 99 54 103 65 119 73 66 65 103 73 81 81 118 65 82 82 87 72 53 86 109 81 69 112 108 89 70 53 98 119 80 102 106 65 75 66 103 103 113 104 107 106 79 80 81 81 68 65 106 66 122 77 81 115 119 10 67 81 89 68 86 81 81 71 69 119 74 86 85 122 69 84 77 66 69 71 65 49 85 69 67 66 77 75 81 50 70 115 97 87 90 118 99 109 53 112 89 84 69 87 77 66 81 71 65 49 85 69 66 120 77 78 85 50 70 117 73 69 90 121 10 89 87 53 106 97 88 78 106 98 122 69 90 77 66 99 71 65 49 85 69 67 104 77 81 98 51 74 110 77 83 53 108 101 71 70 116 99 71 120 108 76 109 78 118 98 84 69 99 77 66 111 71 65 49 85 69 65 120 77 84 89 50 69 117 10 98 51 74 110 77 83 53 108 101 71 70 116 99 71 120 108 76 109 78 118 98 84 65 101 70 119 48 121 77 84 69 120 77 84 85 119 79 84 73 51 77 68 66 97 70 119 48 122 77 84 69 120 77 84 77 119 79 84 73 51 77 68 66 97 10 77 71 111 120 67 122 65 74 66 103 78 86 66 65 89 84 65 108 86 84 77 82 77 119 69 81 89 68 86 81 81 73 69 119 112 68 89 87 120 112 90 109 57 121 98 109 108 104 77 82 89 119 70 65 89 68 86 81 81 72 69 119 49 84 10 89 87 52 103 82 110 74 104 98 109 78 112 99 50 78 118 77 81 48 119 67 119 89 68 86 81 81 76 69 119 82 119 90 87 86 121 77 82 56 119 72 81 89 68 86 81 81 68 69 120 90 119 90 87 86 121 77 83 53 118 99 109 99 120 10 76 109 86 52 89 87 49 119 98 71 85 117 89 50 57 116 77 70 107 119 69 119 89 72 75 111 90 73 122 106 48 67 65 81 89 73 75 111 90 73 122 106 48 68 65 81 99 68 81 103 65 69 74 73 85 48 73 70 108 105 66 71 67 55 10 57 98 77 49 116 74 83 56 69 50 82 112 65 98 107 81 112 114 122 79 113 90 111 84 52 75 120 103 77 68 111 53 68 80 67 81 106 55 72 67 69 108 107 65 47 103 109 78 86 122 69 50 84 73 118 74 122 68 75 115 84 43 86 67 10 87 50 79 84 66 87 85 78 104 75 78 78 77 69 115 119 68 103 89 68 86 82 48 80 65 81 72 47 66 65 81 68 65 103 101 65 77 65 119 71 65 49 85 100 69 119 69 66 47 119 81 67 77 65 65 119 75 119 89 68 86 82 48 106 10 66 67 81 119 73 111 65 103 75 50 65 49 104 114 82 54 113 120 108 50 69 66 57 56 119 55 74 102 50 121 109 47 53 116 103 49 118 103 97 101 88 101 74 51 102 76 98 87 76 50 81 119 67 103 89 73 75 111 90 73 122 106 48 69 10 65 119 73 68 82 119 65 119 82 65 73 103 74 47 85 65 115 55 117 68 103 73 103 111 81 103 84 69 89 77 103 115 54 101 83 118 101 113 80 72 75 76 82 117 89 47 48 99 67 105 104 51 105 69 56 67 73 66 107 119 111 87 120 85 10 89 76 112 88 66 100 113 99 55 109 81 104 77 67 113 57 116 102 113 74 51 53 106 43 54 88 71 89 104 120 112 105 71 71 105 76 켢�1�yx�*\�v��̧����:[10 7 79 114 103 49 77 83 80 18 170 6 45 45 45 45 45 66 69 71 73 78 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10 77 73 73 67 75 68 67 67 65 99 43 103 65 119 73 66 65 103 73 82 65 78 75 57 122 99 52 110 81 75 82 69 86 86 78 52 56 122 88 78 116 77 111 119 67 103 89 73 75 111 90 73 122 106 48 69 65 119 73 119 99 122 69 76 10 77 65 107 71 65 49 85 69 66 104 77 67 86 86 77 120 69 122 65 82 66 103 78 86 66 65 103 84 67 107 78 104 98 71 108 109 98 51 74 117 97 87 69 120 70 106 65 85 66 103 78 86 66 65 99 84 68 86 78 104 98 105 66 71 10 99 109 70 117 89 50 108 122 89 50 56 120 71 84 65 88 66 103 78 86 66 65 111 84 69 71 57 121 90 122 69 117 90 88 104 104 98 88 66 115 90 83 53 106 98 50 48 120 72 68 65 97 66 103 78 86 66 65 77 84 69 50 78 104 10 76 109 57 121 90 122 69 117 90 88 104 104 98 88 66 115 90 83 53 106 98 50 48 119 72 104 99 78 77 106 69 120 77 84 69 49 77 68 107 121 78 122 65 119 87 104 99 78 77 122 69 120 77 84 69 122 77 68 107 121 78 122 65 119 10 87 106 66 113 77 81 115 119 67 81 89 68 86 81 81 71 69 119 74 86 85 122 69 84 77 66 69 71 65 49 85 69 67 66 77 75 81 50 70 115 97 87 90 118 99 109 53 112 89 84 69 87 77 66 81 71 65 49 85 69 66 120 77 78 10 85 50 70 117 73 69 90 121 89 87 53 106 97 88 78 106 98 122 69 78 77 65 115 71 65 49 85 69 67 120 77 69 99 71 86 108 99 106 69 102 77 66 48 71 65 49 85 69 65 120 77 87 99 71 86 108 99 106 65 117 98 51 74 110 10 77 83 53 108 101 71 70 116 99 71 120 108 76 109 78 118 98 84 66 90 77 66 77 71 66 121 113 71 83 77 52 57 65 103 69 71 67 67 113 71 83 77 52 57 65 119 69 72 65 48 73 65 66 74 86 107 51 65 83 72 80 80 82 47 10 69 49 67 107 56 67 54 110 83 86 99 115 114 118 97 75 109 111 115 106 103 101 98 72 98 70 74 51 85 82 56 73 110 106 50 117 115 100 67 107 97 104 119 111 107 86 75 105 99 70 104 107 98 50 117 78 116 107 79 74 77 98 120 48 10 76 114 66 69 84 76 70 49 122 55 101 106 84 84 66 76 77 65 52 71 65 49 85 100 68 119 69 66 47 119 81 69 65 119 73 72 103 68 65 77 66 103 78 86 72 82 77 66 65 102 56 69 65 106 65 65 77 67 115 71 65 49 85 100 10 73 119 81 107 77 67 75 65 73 67 116 103 78 89 97 48 101 113 115 90 100 104 65 102 102 77 79 121 88 57 115 112 118 43 98 89 78 98 52 71 110 108 51 105 100 51 121 50 49 105 57 107 77 65 111 71 67 67 113 71 83 77 52 57 10 66 65 77 67 65 48 99 65 77 69 81 67 73 66 78 111 84 82 120 89 51 52 51 120 82 110 51 109 73 120 82 100 106 105 79 106 85 106 113 76 74 114 56 56 84 76 88 102 65 80 70 122 48 73 98 108 65 105 66 81 66 57 53 53 10 66 120 77 68 79 51 56 54 106 75 76 99 73 114 53 115 51 74 53 68 122 55 115 81 113 121 111 120 85 90 105 121 66 80 116 51 79 103 61 61 10 45 45 45 45 45 69 78 68 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10]]
	*/
	idOfMembers := criteria.idOfMembers
	logger.Info("===========idOfMembers===========", idOfMembers)
	//
	chanMemberById := criteria.chanMemberById
	logger.Info("===========chanMemberById===========", chanMemberById)
	includedGroups := criteria.possibleLayouts.groupsSet()
	logger.Info("===========includedGroups===========", includedGroups)
	/*
	map[G0:{}]
	 */
	res := make(map[string]*discovery.Peers)
	// Map endorsers to their corresponding groups.
	// Iterate the principals, and put the peers into each group that corresponds with a principal vertex
	for grp, principalVertex := range satGraph.principalVertices {
		logger.Info("=======grp=========", grp)
		logger.Info("=======principalVertex=========", principalVertex)
		if _, exists := includedGroups[grp]; !exists {
			// If the current group is not found in any layout, skip the corresponding principal
			// 如果在任何layout中都没有找到当前group，则跳过相应的principal
			continue
		}
		peerList := &discovery.Peers{}
		res[grp] = peerList
		for _, peerVertex := range principalVertex.Neighbors() {
			logger.Info("===========peerVertex=============", peerVertex)
			member := peerVertex.Data.(NetworkMember)
			logger.Info("===============member===========", member)
			logger.Info("==========Identity=========", idOfMembers.identityByPKIID(member.PKIid))
			logger.Info("==========StateInfo=========", chanMemberById[string(member.PKIid)].Envelope)
			logger.Info("==========MembershipInfo=========", member.Envelope)
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
	logger.Info("==========computeLayouts==============")
	logger.Info("====================非常重要===========")
	logger.Info("=========principalsSets===========", principalsSets)
	//[[principal:"\n\001A"  principal:"\n\001B" ] [principal:"\n\001C" ] [principal:"\n\001A"  principal:"\n\001D" ]]
	logger.Info("=========principalGroups===================", principalGroups)
	/*
	========principalGroups=================== map[{0
	B}:G0 {0
	C}:G1 {0
	D}:G2 {0
	A}:G3]
	 */
	logger.Info("=========satGraph==================", *satGraph)
	//=========satGraph================== {[] map[G1:0xc00037cdb0 G2:0xc00037ce10 G3:0xc00037ce70 G0:0xc00037ced0]}
	var layouts []*discovery.Layout
	// principalsSets is a collection of combinations of principals,
	// such that each combination (given enough peers) satisfies the endorsement policy.
	for _, principalSet := range principalsSets {
		logger.Info("=======principalSet=======", principalSet)
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
			logger.Info("========principal===", principal)//principal:"\n\001A" principal:"\n\001B"    principal:"\n\001C"
			logger.Info("========plurality=======", plurality)//1 1  1
			key := principalKey{
				cls:       int32(principal.PrincipalClassification),
				principal: string(principal.Principal),
			}
			// We map the principal to a group, which is an alias for the principal.
			// 我们将principal映射到一个group，这是principal的别名。
			a := principalGroups.group(key)
			logger.Info("==========principalGroups.group(key)==========", a)//G3  G0   G1
			logger.Info("==========uint32(plurality)==========", uint32(plurality))//1 1
			layout.QuantitiesByGroup[a] = uint32(plurality)
			logger.Info("=========layout.QuantitiesByGroup=======================", layout.QuantitiesByGroup)
			//map[G3:1 G0:1]  map[G1:1] map[G2:1 G3:1]
			//map[G0:1]
		}
		// Check that the layout can be satisfied with the current known peers
		// 检查layout是否可以满足当前已知的peers
		// This is done by iterating the current layout, and ensuring that
		// 这是通过迭代当前layout来完成的，并确保
		// each principal vertex is connected to at least <plurality> peer vertices.
		//每个principal vertex至少连接到 <plurality> peer vertices

		b := isLayoutSatisfied(layout.QuantitiesByGroup, satGraph)
		logger.Info("========isLayoutSatisfied(layout.QuantitiesByGroup, satGraph)========", b)
		//true
		if b {
			// If so, then add the layout to the layouts, since we have enough peers to satisfy the principal combination
			//如果是，则将layout添加到layouts中，因为我们有足够的peers来满足principal combination
			layouts = append(layouts, layout)
		}
	}
	return layouts
}

func isLayoutSatisfied(layout map[string]uint32, satGraph *principalPeerGraph) bool {
	logger.Info("==========isLayoutSatisfied==============")
	for grp, plurality := range layout {
		logger.Info("=============grp===========", grp) //G3
		logger.Info("=============plurality===========", plurality)//1
		// Do we have more than <plurality> peers connected to the principal?
		a := satGraph.principalVertices[grp]
		b := satGraph.principalVertices[grp].Neighbors()

		logger.Info("=============satGraph.principalVertices[grp]===============", a) //&{G3 principal:"\n\001A"  map[]} &{G3 principal:"\n\001A"  map[]} &{G1 principal:"\n\001C"  map[]}  &{G2 principal:"\n\001D"  map[]}
		logger.Info("=============satGraph.principalVertices[grp].Neighbors()===============", b)//[]
		//[0xc0024118f0 0xc002411950] map[Pv�p�.�4:�
		logger.Info("=============satGraph.principalVertices[grp].Neighbors() len===============", len(satGraph.principalVertices[grp].Neighbors()))//0
		if len(satGraph.principalVertices[grp].Neighbors()) < int(plurality) {
			logger.Info("==============================len(satGraph.principalVertices[grp].Neighbors()) < int(plurality)==========")
			return false
		}
		logger.Info("=============满足条件==================")
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
	logger.Info("==========principalsToPeersGraph==============")
	// Create the peer vertices
	peerVertices := make([]*graph.Vertex, len(data.members))
	for i, member := range data.members {
		logger.Info("===========i",)
		logger.Info("===========member,member")
		logger.Info("============string(member.PKIid)==============",string(member.PKIid))
		logger.Info("============member=============",member)
		peerVertices[i] = graph.NewVertex(string(member.PKIid), member)

	}
	logger.Info("==============peerVertices=======",peerVertices)
	// Create the principal vertices
	principalVertices := make(map[string]*graph.Vertex)
	for pKey, grp := range data.pGrps {
		logger.Info("======pKey==",pKey)
		logger.Info("======grp==",grp)
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
	logger.Info("==========mapPrincipalsToGroups==============")
	logger.Info("============principalsSets========", principalsSets)
	//[[principal:"\n\001A"  principal:"\n\001B" ] [principal:"\n\001C" ] [principal:"\n\001A"  principal:"\n\001D" ]]

	// [[principal:"\n\007Org1MSP\020\003" ]]
	groupMapper := make(principalGroupMapper)
	totalPrincipals := make(map[principalKey]struct{})
	for _, principalSet := range principalsSets {
		logger.Info("=====principalSet===", principalSet)//[principal:"\n\001A"  principal:"\n\001B" ]
		for _, principal := range principalSet {
			logger.Info("=====principal===", principal)
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
	logger.Info("=======memberIdentities===identityByPKIID==============")
	return m[string(id)]
}

func computeIdentitiesOfMembers(identitySet api.PeerIdentitySet, members map[string]NetworkMember) memberIdentities {
	logger.Info("=======computeIdentitiesOfMembers=============")
	logger.Info("=================identitySet===================",identitySet)//=================identitySet=================== [{7030 [10 1 65 18 2 112 48] [65]} {7031 [10 1 65 18 2 112 49] [65]} {7032 [10 1 66 18 2 112 50] [66]} {7033 [10 1 66 18 2 112 51] [66]} {7034 [10 1 67 18 2 112 52] [67]} {7035 [10 1 67 18 2 112 53] [67]} {7036 [10 1 68 18 2 112 54] [68]} {7037 [10 1 68 18 2 112 55] [68]} {7038 [10 1 65 18 2 112 56] [65]} {7039 [10 1 65 18 2 112 57] [65]} {703130 [10 1 66 18 3 112 49 48] [66]} {703131 [10 1 66 18 3 112 49 49] [66]} {703132 [10 1 67 18 3 112 49 50] [67]} {703133 [10 1 67 18 3 112 49 51] [67]} {703134 [10 1 68 18 3 112 49 52] [68]} {703135 [10 1 68 18 3 112 49 53] [68]}]
	logger.Info("=================members===================",members)//map[]

	identitiesByPKIID := make(map[string]api.PeerIdentityType)
	identitiesOfMembers := make(map[string]api.PeerIdentityType, len(members))
	for _, identity := range identitySet {
		logger.Info("================identity================",identity) // {7030 [10 1 65 18 2 112 48] [65]}
		logger.Info("===============string(identity.PKIId)================",string(identity.PKIId))//p0
		logger.Info("===============identity.Identity================",identity.Identity)//[10 1 65 18 2 112 48]
		identitiesByPKIID[string(identity.PKIId)] = identity.Identity
	}
	logger.Info("===============identitiesByPKIID================",identitiesByPKIID)
	//map[p8:[10 1 65 18 2 112 56] p12:[10 1 67 18 3 112 49 50] p0:[10 1 65 18 2 112 48] p9:[10 1 65 18 2 112 57] p6:[10 1 68 18 2 112 54] p7:[10 1 68 18 2 112 55] p13:[10 1 67 18 3 112 49 51] p14:[10 1 68 18 3 112 49 52] p3:[10 1 66 18 2 112 51] p2:[10 1 66 18 2 112 50] p4:[10 1 67 18 2 112 52] p5:[10 1 67 18 2 112 53] p10:[10 1 66 18 3 112 49 48] p11:[10 1 66 18 3 112 49 49] p15:[10 1 68 18 3 112 49 53] p1:[10 1 65 18 2 112 49]]
	for _, member := range members {
		logger.Info("==========member===========",member)
		if identity, exists := identitiesByPKIID[string(member.PKIid)]; exists {
			logger.Info("============identity==========================",identity)
			logger.Info("=========string(member.PKIid)========================",string(member.PKIid))

			identitiesOfMembers[string(member.PKIid)] = identity
		}
	}
	logger.Info("==========identitiesOfMembers==============",identitiesOfMembers)
	return identitiesOfMembers
}

// principalGroupMapper maps principals to names of groups
type principalGroupMapper map[principalKey]string

func (mapper principalGroupMapper) group(principal principalKey) string {
	logger.Info("=======principalGroupMapper===group==========")
	if grp, exists := mapper[principal]; exists {
		logger.Info("======grp======", grp) //G3  G0 G1
		logger.Info("======exists======", exists)//true true
		return grp
	}
	grp := fmt.Sprintf("G%d", len(mapper))
	logger.Info("=====grp====", grp)//G0 G1 G2 G3 G0
	mapper[principal] = grp
	logger.Info("======mapper====", mapper)
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
	logger.Info("=======principalKey===toPrincipal==========")
	logger.Info("=========pk.cls=======", pk.cls)//0
	logger.Info("========msp.MSPPrincipal_Classification(pk.cls)======", msp.MSPPrincipal_Classification(pk.cls))//ROLE

	logger.Info("=========pk.principal======", pk.principal)//C D A B
	return &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_Classification(pk.cls),
		Principal:               []byte(pk.principal),
	}
}

// layouts is an aggregation of several layouts
type layouts []*discovery.Layout

// groupsSet returns a set of groups that the layouts contain
func (l layouts) groupsSet() map[string]struct{} {
	logger.Info("=======layouts===groupsSet==========")
	m := make(map[string]struct{})
	for _, layout := range l {
		for grp := range layout.QuantitiesByGroup {
			m[grp] = struct{}{}
		}
	}
	return m
}

func peersWithChaincode(metadata ...*chaincode.Metadata) func(member NetworkMember) bool {
	logger.Info("=======peersWithChaincode==========")
	logger.Info("======metadata============", metadata)
	return func(member NetworkMember) bool {
		logger.Info("====member.Properties=====", member.Properties)
		//ledger_height:2 chaincodes:<name:"acb" version:"0" >
		if member.Properties == nil {
			return false
		}
		for _, ccMD := range metadata {
			logger.Info("====ccMD=======", ccMD)
			/*
			&{acb 0 [18 8 18 6 8 1 18 2 8 0 26 13 18 11 10 7 79 114 103 49 77 83 80 16 3] [56 109 134 19 159 41 115 44 69 237 248 152 247 49 199 116 11 242 191 45 80 5 45 110 172 34 25 242 87 241 163 144] []}

			*/
			var found bool
			for _, cc := range member.Properties.Chaincodes {
				logger.Info("===========cc=====", cc)
				logger.Info("===========cc.Name=====", cc.Name)//acb
				logger.Info("===========ccMD.Name=====", ccMD.Name)//acb
				logger.Info("===========cc.Version=====", cc.Version)//0
				logger.Info("===========ccMD.Version=====", ccMD.Version)//0
				if cc.Name == ccMD.Name && cc.Version == ccMD.Version {
					found = true
				}
			}
			//logger.Info("====found===", found)
			if !found {
				return false
			}
		}
		return true
	}
}

func mergePrincipalSets(cpss []inquire.ComparablePrincipalSets) (inquire.ComparablePrincipalSets, error) {
	logger.Info("=======mergePrincipalSets==========")
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
	logger.Info("=======popComparablePrincipalSets==========")
	if len(sets) == 0 {
		return nil, nil, errors.New("no principal sets remained after filtering")
	}
	cps, cpss := sets[0], sets[1:]
	logger.Info("====sets", sets)
	//sets [[[A.MEMBER, B.MEMBER] [C.MEMBER] [A.MEMBER, D.MEMBER]]]
	//[[[Org1MSP.PEER]]]
	logger.Info("=====cps", cps)//[[A.MEMBER, B.MEMBER] [C.MEMBER] [A.MEMBER, D.MEMBER]]
	//=====cps
	logger.Info("=====cpss", cpss) //[]
	return cps, cpss, nil
}
