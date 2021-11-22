/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package discovery

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/core/comm"
	common2 "github.com/hyperledger/fabric/gossip/common"
	discovery2 "github.com/hyperledger/fabric/gossip/discovery"
	"github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/discovery"
	"github.com/pkg/errors"
)

var (
	logger = flogging.MustGetLogger("discovery")
)

var accessDenied = wrapError(errors.New("access denied"))

// certHashExtractor extracts the TLS certificate from a given context
// and returns its hash
type certHashExtractor func(ctx context.Context) []byte

// dispatcher defines a function that dispatches a query
type dispatcher func(q *discovery.Query) *discovery.QueryResult

type service struct {
	config             Config
	channelDispatchers map[discovery.QueryType]dispatcher
	localDispatchers   map[discovery.QueryType]dispatcher
	auth               *authCache
	Support
}

// Config defines the configuration of the discovery service
type Config struct {
	TLS                          bool
	AuthCacheEnabled             bool
	AuthCacheMaxSize             int
	AuthCachePurgeRetentionRatio float64
}

// String returns a string representation of this Config
func (c Config) String() string {
	fmt.Println("=======Config==String==")
	//[Created with config TLS: true, auth cache disabled]
	a := ""
	if c.AuthCacheEnabled {
		a = fmt.Sprintf("TLS: %t, authCacheMaxSize: %d, authCachePurgeRatio: %f", c.TLS, c.AuthCacheMaxSize, c.AuthCachePurgeRetentionRatio)
		fmt.Println("==c.AuthCacheEnabled======", a)
		return a
	}
	a = fmt.Sprintf("TLS: %t, auth cache disabled", c.TLS)
	fmt.Println("==a======", a) //TLS: true, auth cache disabled
	return a
}

// peerMapping maps PKI-IDs to Peers
type peerMapping map[string]*discovery.Peer

// NewService creates a new discovery service instance
func NewService(config Config, sup Support) *service {
	fmt.Println("======NewService==")
	s := &service{
		auth: newAuthCache(sup, authCacheConfig{
			enabled:             config.AuthCacheEnabled,
			maxCacheSize:        config.AuthCacheMaxSize,
			purgeRetentionRatio: config.AuthCachePurgeRetentionRatio,
		}),
		Support: sup,
	}
	s.channelDispatchers = map[discovery.QueryType]dispatcher{
		discovery.ConfigQueryType:         s.configQuery,
		discovery.ChaincodeQueryType:      s.chaincodeQuery,
		discovery.PeerMembershipQueryType: s.channelMembershipResponse,
	}
	s.localDispatchers = map[discovery.QueryType]dispatcher{
		discovery.LocalMembershipQueryType: s.localMembershipResponse,
	}
	logger.Info("Created with config", config)
	return s
}

func (s *service) Discover(ctx context.Context, request *discovery.SignedRequest) (*discovery.Response, error) {
	fmt.Println("===service===Discover==")
	addr := util.ExtractRemoteAddress(ctx)
	req, err := validateStructure(ctx, request, s.config.TLS, comm.ExtractCertificateHashFromContext)
	if err != nil {
		logger.Warningf("Request from %s is malformed or invalid: %v", addr, err)
		return nil, err
	}
	logger.Debugf("Processing request from %s: %v", addr, req)
	var res []*discovery.QueryResult
	for _, q := range req.Queries {
		res = append(res, s.processQuery(q, request, req.Authentication.ClientIdentity, addr))
	}
	logger.Debugf("Returning to %s a response containing: %v", addr, res)
	return &discovery.Response{
		Results: res,
	}, nil
}

func (s *service) processQuery(query *discovery.Query, request *discovery.SignedRequest, identity []byte, addr string) *discovery.QueryResult {
	fmt.Println("===service===processQuery==")
	//fmt.Println("========query.Channel==========", query.Channel)//mychannel
	//fmt.Println("===========!s.ChannelExists(query.Channel)=============", s.ChannelExists(query.Channel)) // true
	if query.Channel != "" && !s.ChannelExists(query.Channel) {
		logger.Warning("got query for channel", query.Channel, "from", addr, "but it doesn't exist")
		return accessDenied
	}

	//fmt.Println("=========request.Payload=======",request.Payload)
	/*
	[10 219 6 10 182 6 10 7 79 114 103 49 77 83 80 18 170 6 45 45 45 45 45 66 69 71 73 78 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10 77 73 73 67 75 106 67 67 65 100 71 103 65 119 73 66 65 103 73 82 65 76 118 56 78 100 104 116 73 121 74 71 72 107 49 68 79 43 99 90 102 67 56 119 67 103 89 73 75 111 90 73 122 106 48 69 65 119 73 119 99 122 69 76 10 77 65 107 71 65 49 85 69 66 104 77 67 86 86 77 120 69 122 65 82 66 103 78 86 66 65 103 84 67 107 78 104 98 71 108 109 98 51 74 117 97 87 69 120 70 106 65 85 66 103 78 86 66 65 99 84 68 86 78 104 98 105 66 71 10 99 109 70 117 89 50 108 122 89 50 56 120 71 84 65 88 66 103 78 86 66 65 111 84 69 71 57 121 90 122 69 117 90 88 104 104 98 88 66 115 90 83 53 106 98 50 48 120 72 68 65 97 66 103 78 86 66 65 77 84 69 50 78 104 10 76 109 57 121 90 122 69 117 90 88 104 104 98 88 66 115 90 83 53 106 98 50 48 119 72 104 99 78 77 106 69 120 77 84 69 49 77 68 107 121 78 122 65 119 87 104 99 78 77 122 69 120 77 84 69 122 77 68 107 121 78 122 65 119 10 87 106 66 115 77 81 115 119 67 81 89 68 86 81 81 71 69 119 74 86 85 122 69 84 77 66 69 71 65 49 85 69 67 66 77 75 81 50 70 115 97 87 90 118 99 109 53 112 89 84 69 87 77 66 81 71 65 49 85 69 66 120 77 78 10 85 50 70 117 73 69 90 121 89 87 53 106 97 88 78 106 98 122 69 80 77 65 48 71 65 49 85 69 67 120 77 71 89 50 120 112 90 87 53 48 77 82 56 119 72 81 89 68 86 81 81 68 68 66 90 66 90 71 49 112 98 107 66 118 10 99 109 99 120 76 109 86 52 89 87 49 119 98 71 85 117 89 50 57 116 77 70 107 119 69 119 89 72 75 111 90 73 122 106 48 67 65 81 89 73 75 111 90 73 122 106 48 68 65 81 99 68 81 103 65 69 47 67 67 106 77 72 99 120 10 118 102 113 76 66 51 122 53 86 107 50 114 101 79 97 73 49 80 82 97 97 89 111 114 112 119 47 77 112 55 67 108 110 121 69 83 104 50 122 78 81 90 115 53 112 103 72 74 57 89 118 79 99 108 100 69 109 66 112 104 115 70 112 120 10 119 78 77 90 122 76 115 122 86 108 43 114 72 54 78 78 77 69 115 119 68 103 89 68 86 82 48 80 65 81 72 47 66 65 81 68 65 103 101 65 77 65 119 71 65 49 85 100 69 119 69 66 47 119 81 67 77 65 65 119 75 119 89 68 10 86 82 48 106 66 67 81 119 73 111 65 103 75 50 65 49 104 114 82 54 113 120 108 50 69 66 57 56 119 55 74 102 50 121 109 47 53 116 103 49 118 103 97 101 88 101 74 51 102 76 98 87 76 50 81 119 67 103 89 73 75 111 90 73 10 122 106 48 69 65 119 73 68 82 119 65 119 82 65 73 103 97 100 54 116 65 100 43 111 89 109 105 84 80 66 66 50 106 83 76 77 84 88 115 85 76 121 108 102 71 121 68 69 73 97 108 55 81 87 103 97 102 48 103 67 73 68 67 76 10 106 114 89 71 104 79 113 110 49 100 98 116 47 89 82 76 67 118 68 101 79 100 101 120 85 74 70 106 47 48 114 70 110 74 57 116 87 75 114 108 10 45 45 45 45 45 69 78 68 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10 18 32 227 176 196 66 152 252 28 20 154 251 244 200 153 111 185 36 39 174 65 228 100 155 147 76 164 149 153 27 120 82 184 85 18 15 10 9 109 121 99 104 97 110 110 101 108 26 2 10 0]
	*/
	//fmt.Println("===========request.Signature========",request.Signature)
	/*
	[48 68 2 32 59 75 227 206 194 85 89 136 36 87 187 9 253 64 190 2 48 76 195 177 120 177 234 77 208 63 137 226 162 148 232 75 2 32 63 135 247 243 229 139 0 87 36 115 171 20 156 204 212 45 253 7 10 180 98 53 192 68 37 241 209 3 151 190 133 67]
	*/
	//fmt.Println("===========request.identity========",identity)
	if err := s.auth.EligibleForService(query.Channel, common.SignedData{
		Data:      request.Payload,
		Signature: request.Signature,
		Identity:  identity,
	}); err != nil {
		logger.Warning("got query for channel", query.Channel, "from", addr, "but it isn't eligible:", err)
		return accessDenied
	}
	return s.dispatch(query)
}

func (s *service) dispatch(q *discovery.Query) *discovery.QueryResult {
	fmt.Println("===service===dispatch==")
	fmt.Println("==========q=======", q)
	//channel:"mychannel" config_query:<>   local_peers:<>
	//channel:"mychannel" peer_query:<filter:<> >
	dispatchers := s.channelDispatchers
	fmt.Println("=======dispatchers============", dispatchers)
	//map[1:0xd54ce0 3:0xd54d30 2:0xd54d80]
	//map[1:0xd54ce0 3:0xd54d30 2:0xd54d80]
	//map[2:0xe436a0 1:0xe43600 3:0xe43650]
	// Ensure local queries are routed only to channel-less dispatchers
	if q.Channel == "" {
		dispatchers = s.localDispatchers
		fmt.Println("=========s.localDispatchers=====", s.localDispatchers)
	}
	fmt.Println("========q.GetType()======", q.GetType()) //1
	dispatchQuery, exists := dispatchers[q.GetType()]
	fmt.Println("=========dispatchQuery==================", dispatchQuery) //0xd54d80
	fmt.Println("================exists==============", exists)            //true
	if !exists {
		return wrapError(errors.New("unknown or missing request type"))
	}
	return dispatchQuery(q)
}

func (s *service) chaincodeQuery(q *discovery.Query) *discovery.QueryResult {
	fmt.Println("===service===chaincodeQuery==")
	err := validateCCQuery(q.GetCcQuery())
	fmt.Println("======err======",err)
	if err != nil {
		return wrapError(err)
	}
	var descriptors []*discovery.EndorsementDescriptor
	c := q.GetCcQuery().Interests
	fmt.Println("=============c=",c)
	for _, interest := range c {
		fmt.Println("======interest=======",interest)//chaincodes:<name:"mycc" >
		//fmt.Println("=====================q.Channel==============",q.Channel)//mychannel
		//fmt.Println("=====================common2.ChainID(q.Channel)==============",common2.ChainID(q.Channel))//[109 121 99 104 97 110 110 101 108]
		desc, err := s.PeersForEndorsement(common2.ChainID(q.Channel), interest)
		fmt.Println("===============desc================",desc)
		/*

		 */
		if err != nil {
			logger.Errorf("dddc Failed constructing descriptor for chaincode %s,: %v", interest, err)
			return wrapError(errors.Errorf("failed constructing descriptor for %v", interest))
		}
		descriptors = append(descriptors, desc)
	}

	fmt.Println("==============descriptors==================",descriptors)

	return &discovery.QueryResult{
		Result: &discovery.QueryResult_CcQueryRes{
			CcQueryRes: &discovery.ChaincodeQueryResult{
				Content: descriptors,
			},
		},
	}
}

func (s *service) configQuery(q *discovery.Query) *discovery.QueryResult {
	fmt.Println("===service===configQuery==")
	fmt.Println("===q.Channel=====",q.Channel)
	conf, err := s.Config(q.Channel)
	if err != nil {
		logger.Errorf("Failed fetching config for channel %s: %v", q.Channel, err)
		return wrapError(errors.Errorf("failed fetching config for channel %s", q.Channel))
	}
	return &discovery.QueryResult{
		Result: &discovery.QueryResult_ConfigResult{
			ConfigResult: conf,
		},
	}
}

func wrapPeerResponse(peersByOrg map[string]*discovery.Peers) *discovery.QueryResult {
	fmt.Println("===wrapPeerResponse==")
	fmt.Println("=============peersByOrg=============", peersByOrg)
	return &discovery.QueryResult{
		Result: &discovery.QueryResult_Members{
			Members: &discovery.PeerMembershipResult{
				PeersByOrg: peersByOrg,
			},
		},
	}
}

func (s *service) channelMembershipResponse(q *discovery.Query) *discovery.QueryResult {
	fmt.Println("==service===channelMembershipResponse==")

	//fmt.Println("============common2.ChainID(q.Channel)==========", common2.ChainID(q.Channel))//[109 121 99 104 97 110 110 101 108]
	fmt.Println("==========q.GetPeerQuery().Filter==", q.GetPeerQuery().Filter)
	chanPeers, err := s.PeersAuthorizedByCriteria(common2.ChainID(q.Channel), q.GetPeerQuery().Filter)
	//fmt.Println("==========chanPeers=========", chanPeers)
	if err != nil {
		return wrapError(err)
	}
	membersByOrgs := make(map[string]*discovery.Peers)
	chanPeerByID := discovery2.Members(chanPeers).ByID()
	//fmt.Println("=====chanPeerByID======",chanPeerByID)
	for org, ids2Peers := range s.computeMembership(q) {

		//fmt.Println("==========org========",org)//Org1MSP
		//fmt.Println("==========ids2Peers========",ids2Peers)

		membersByOrgs[org] = &discovery.Peers{}
		for id, peer := range ids2Peers {
			//fmt.Println("================id",id)
			//fmt.Println("================peer",peer)
			//state_info:<payload:"\030\005zh\022\024\010\375\262\225\210\231\306\266\334\026\020\377\317\337\237\327\307\266\334\026\032 o$\242\205\020\032\336\250\3065\r\354\274\242\3071\360yx\243*\\\250v\303\362\314\247\265\263\340\220\" \037\255\332h\003\255)\252\027>\367 \260\354\030\350f\023\235\252\3379\274\206\247\355\340\230\202>\226\270*\014\010\002\032\010\n\003acb\022\0010" signature:"0D\002 ,\243\026)\216\264\017x\035\2408\206\363\250\316DX\t~\017N,\277O\000\254\206o5\211\261\351\002 ri\271\014\353\377\371wj\300\335nN\215\372\322/!f\320mQ6O\264u5\373y\0306\307" > membership_info:<payload:"\030\001*O\n?\n\033peer0.org1.example.com:7051\032 o$\242\205\020\032\336\250\3065\r\354\274\242\3071\360yx\243*\\\250v\303\362\314\247\265\263\340\220\022\014\010\361\201\273\253\204\305\266\334\026\020=" > identity:"\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKDCCAc+gAwIBAgIRANK9zc4nQKREVVN48zXNtMowCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTE1MDkyNzAwWhcNMzExMTEzMDkyNzAw\nWjBqMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzENMAsGA1UECxMEcGVlcjEfMB0GA1UEAxMWcGVlcjAub3Jn\nMS5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABJVk3ASHPPR/\nE1Ck8C6nSVcsrvaKmosjgebHbFJ3UR8Inj2usdCkahwokVKicFhkb2uNtkOJMbx0\nLrBETLF1z7ejTTBLMA4GA1UdDwEB/wQEAwIHgDAMBgNVHRMBAf8EAjAAMCsGA1Ud\nIwQkMCKAICtgNYa0eqsZdhAffMOyX9spv+bYNb4Gnl3id3y21i9kMAoGCCqGSM49\nBAMCA0cAMEQCIBNoTRxY343xRn3mIxRdjiOjUjqLJr88TLXfAPFz0IblAiBQB955\nBxMDO386jKLcIr5s3J5Dz7sQqyoxUZiyBPt3Og==\n-----END CERTIFICATE-----\n"
			// Check if the peer is in the channel view
			stateInfoMsg, exists := chanPeerByID[string(id)]
			//fmt.Println("=========stateInfoMsg========",stateInfoMsg)
			//payload:"\030\005zh\022\024\010\375\262\225\210\231\306\266\334\026\020\377\317\337\237\327\307\266\334\026\032 o$\242\205\020\032\336\250\3065\r\354\274\242\3071\360yx\243*\\\250v\303\362\314\247\265\263\340\220\" \037\255\332h\003\255)\252\027>\367 \260\354\030\350f\023\235\252\3379\274\206\247\355\340\230\202>\226\270*\014\010\002\032\010\n\003acb\022\0010" signature:"0D\002 ,\243\026)\216\264\017x\035\2408\206\363\250\316DX\t~\017N,\277O\000\254\206o5\211\261\351\002 ri\271\014\353\377\371wj\300\335nN\215\372\322/!f\320mQ6O\264u5\373y\0306\307"
			/*
			=========stateInfoMsg======== Endpoint: peer1.org1.example.com:7051, InternalEndpoint: peer1.org1.example.com:7051, PKI-ID: 500f76e570c22e91343ac9c28db55d6948faa44da710db6fa7971641d4ea285b, Metadata:

			*/
			//fmt.Println("=========exists========",exists)
			// If the peer isn't in the channel view, skip it and don't include it in the response
			if !exists {
				continue
			}
			peer.StateInfo = stateInfoMsg.Envelope
			//fmt.Println("===========peer.StateInfo===========",peer.StateInfo)
			//fmt.Println("=============org============",org)
			//fmt.Println("=============peer============",peer)
			membersByOrgs[org].Peers = append(membersByOrgs[org].Peers, peer)
		}
	}
//	fmt.Println("=======membersByOrgs===============",membersByOrgs)
/*
   map[Org1MSP:peers:<state_info:<payload:"\030\005zh\022\024\010\225\336\270\211\231\306\266\334\026\020\343\252\365\245\327\307\266\334\026\032 P\017v\345p\302.\2214:\311\302\215\265]iH\372\244M\247\020\333o\247\227\026A\324\352([\" \004;\274/\345\264|~\030%!E\210\003\024O\032!W\246)\374\343\364\333$4\354\223O*&*\014\010\002\032\010\n\003acb\022\0010" signature:"0E\002!\000\341\277\007\305\0076I\031.y\243\031W\200\034le\213\362V\3562W\273\221\352Z~\200\003\037\255\002 /e\201/\030\345\214\204\273/l\251\203\3419\177\033G\\\256\244\200\256OK,\031\307\345L\247\310" > membership_info:<payload:"\030\001*O\n?\n\033peer1.org1.example.com:7051\032 P\017v\345p\302.\2214:\311\302\215\265]iH\372\244M\247\020\333o\247\227\026A\324\352([\022\014\010\250\321\214\204\205\305\266\334\026\020:" signature:"0E\002!\000\242\363\002\252\322J\263\225o\312|\200z\221}R\373\314\363\246\375\010\221\270\035<gw\032\024t\021\002 v\205\265\346@\367\330Hn\3279\233L\307Yt\221\177S\247\034I,\254\320\202`\n\341\024\004s" > identity:"\n\007Org1MSP\022\246\006-----BEGIN CERTIFICATE-----\nMIICJzCCAc6gAwIBAgIQQvARRWH5VmQEplYF5bwPfjAKBggqhkjOPQQDAjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMS5leGFtcGxlLmNvbTEcMBoGA1UEAxMTY2Eu\nb3JnMS5leGFtcGxlLmNvbTAeFw0yMTExMTUwOTI3MDBaFw0zMTExMTMwOTI3MDBa\nMGoxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMQ0wCwYDVQQLEwRwZWVyMR8wHQYDVQQDExZwZWVyMS5vcmcx\nLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEJIU0IFliBGC7\n9bM1tJS8E2RpAbkQprzOqZoT4KxgMDo5DPCQj7HCElkA/gmNVzE2TIvJzDKsT+VC\nW2OTBWUNhKNNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYDVR0j\nBCQwIoAgK2A1hrR6qxl2EB98w7Jf2ym/5tg1vgaeXeJ3fLbWL2QwCgYIKoZIzj0E\nAwIDRwAwRAIgJ/UAs7uDgIgoQgTEYMgs6eSveqPHKLRuY/0cCih3iE8CIBkwoWxU\nYLpXBdqc7mQhMCq9tfqJ35j+6XGYhxpiGGiL\n-----END CERTIFICATE-----\n" > peers:<state_info:<payload:"\030\005zh\022\024\010\375\262\225\210\231\306\266\334\026\020\377\317\337\237\327\307\266\334\026\032 o$\242\205\020\032\336\250\3065\r\354\274\242\3071\360yx\243*\\\250v\303\362\314\247\265\263\340\220\" \037\255\332h\003\255)\252\027>\367 \260\354\030\350f\023\235\252\3379\274\206\247\355\340\230\202>\226\270*\014\010\002\032\010\n\003acb\022\0010" signature:"0D\002 ,\243\026)\216\264\017x\035\2408\206\363\250\316DX\t~\017N,\277O\000\254\206o5\211\261\351\002 ri\271\014\353\377\371wj\300\335nN\215\372\322/!f\320mQ6O\264u5\373y\0306\307" > membership_info:<payload:"\030\001*O\n?\n\033peer0.org1.example.com:7051\032 o$\242\205\020\032\336\250\3065\r\354\274\242\3071\360yx\243*\\\250v\303\362\314\247\265\263\340\220\022\014\010\361\201\273\253\204\305\266\334\026\020=" > identity:"\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKDCCAc+gAwIBAgIRANK9zc4nQKREVVN48zXNtMowCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTE1MDkyNzAwWhcNMzExMTEzMDkyNzAw\nWjBqMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzENMAsGA1UECxMEcGVlcjEfMB0GA1UEAxMWcGVlcjAub3Jn\nMS5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABJVk3ASHPPR/\nE1Ck8C6nSVcsrvaKmosjgebHbFJ3UR8Inj2usdCkahwokVKicFhkb2uNtkOJMbx0\nLrBETLF1z7ejTTBLMA4GA1UdDwEB/wQEAwIHgDAMBgNVHRMBAf8EAjAAMCsGA1Ud\nIwQkMCKAICtgNYa0eqsZdhAffMOyX9spv+bYNb4Gnl3id3y21i9kMAoGCCqGSM49\nBAMCA0cAMEQCIBNoTRxY343xRn3mIxRdjiOjUjqLJr88TLXfAPFz0IblAiBQB955\nBxMDO386jKLcIr5s3J5Dz7sQqyoxUZiyBPt3Og==\n-----END CERTIFICATE-----\n" > ]

 */
	return wrapPeerResponse(membersByOrgs)
}

func (s *service) localMembershipResponse(q *discovery.Query) *discovery.QueryResult {
	fmt.Println("==service===localMembershipResponse==")
	membersByOrgs := make(map[string]*discovery.Peers)
	for org, ids2Peers := range s.computeMembership(q) {

		membersByOrgs[org] = &discovery.Peers{}
		for _, peer := range ids2Peers {
			membersByOrgs[org].Peers = append(membersByOrgs[org].Peers, peer)
		}
	}
	return wrapPeerResponse(membersByOrgs)
}

func (s *service) computeMembership(_ *discovery.Query) map[string]peerMapping {
	fmt.Println("==service===computeMembership==")
	peersByOrg := make(map[string]peerMapping)

	p := s.Peers()
	fmt.Println("========s.Peers()====", p)
	/*
	========s.Peers()==== [Endpoint: peer1.org1.example.com:7051, InternalEndpoint: peer1.org1.example.com:7051, PKI-ID: 500f76e570c22e91343ac9c28db55d6948faa44da710db6fa7971641d4ea285b, Metadata:  Endpoint: peer0.org1.example.com:7051, InternalEndpoint: , PKI-ID: 6f24a285101adea8c6350decbca2c731f07978a32a5ca876c3f2cca7b5b3e090, Metadata: ]
	*/
	peerAliveInfo := discovery2.Members(p).ByID()
	fmt.Println("==========peerAliveInfo============",peerAliveInfo)
	for org, peerIdentities := range s.IdentityInfo().ByOrg() {
		//fmt.Println("===========org========",org)
		//fmt.Println("======peerIdentities======",peerIdentities)
		peersForCurrentOrg := make(peerMapping)
		peersByOrg[org] = peersForCurrentOrg
		for _, id := range peerIdentities {
			//fmt.Println("==================id",id)
			// Check peer exists in alive membership view
			//a := string(id.PKIId)
			//fmt.Println("===========id.PKIId===========",a)
			aliveInfo, exists := peerAliveInfo[string(id.PKIId)]

			//fmt.Println("==========aliveInfo=============",aliveInfo)
			//fmt.Println("==========exists=============",exists)
			/*
			==========aliveInfo============= Endpoint: peer1.org1.example.com:7051, InternalEndpoint: peer1.org1.example.com:7051, PKI-ID: 500f76e570c22e91343ac9c28db55d6948faa44da710db6fa7971641d4ea285b, Metadata:

			*/
			if !exists {
				continue
			}

			//fmt.Println("=========id.PKIId==========",string(id.PKIId))
			/*
			==================id {6f24a285101adea8c6350decbca2c731f07978a32a5ca876c3f2cca7b5b3e090 [10 7 79 114 103 49 77 83 80 18 170 6 45 45 45 45 45 66 69 71 73 78 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10 77 73 73 67 75 68 67 67 65 99 43 103 65 119 73 66 65 103 73 82 65 78 75 57 122 99 52 110 81 75 82 69 86 86 78 52 56 122 88 78 116 77 111 119 67 103 89 73 75 111 90 73 122 106 48 69 65 119 73 119 99 122 69 76 10 77 65 107 71 65 49 85 69 66 104 77 67 86 86 77 120 69 122 65 82 66 103 78 86 66 65 103 84 67 107 78 104 98 71 108 109 98 51 74 117 97 87 69 120 70 106 65 85 66 103 78 86 66 65 99 84 68 86 78 104 98 105 66 71 10 99 109 70 117 89 50 108 122 89 50 56 120 71 84 65 88 66 103 78 86 66 65 111 84 69 71 57 121 90 122 69 117 90 88 104 104 98 88 66 115 90 83 53 106 98 50 48 120 72 68 65 97 66 103 78 86 66 65 77 84 69 50 78 104 10 76 109 57 121 90 122 69 117 90 88 104 104 98 88 66 115 90 83 53 106 98 50 48 119 72 104 99 78 77 106 69 120 77 84 69 49 77 68 107 121 78 122 65 119 87 104 99 78 77 122 69 120 77 84 69 122 77 68 107 121 78 122 65 119 10 87 106 66 113 77 81 115 119 67 81 89 68 86 81 81 71 69 119 74 86 85 122 69 84 77 66 69 71 65 49 85 69 67 66 77 75 81 50 70 115 97 87 90 118 99 109 53 112 89 84 69 87 77 66 81 71 65 49 85 69 66 120 77 78 10 85 50 70 117 73 69 90 121 89 87 53 106 97 88 78 106 98 122 69 78 77 65 115 71 65 49 85 69 67 120 77 69 99 71 86 108 99 106 69 102 77 66 48 71 65 49 85 69 65 120 77 87 99 71 86 108 99 106 65 117 98 51 74 110 10 77 83 53 108 101 71 70 116 99 71 120 108 76 109 78 118 98 84 66 90 77 66 77 71 66 121 113 71 83 77 52 57 65 103 69 71 67 67 113 71 83 77 52 57 65 119 69 72 65 48 73 65 66 74 86 107 51 65 83 72 80 80 82 47 10 69 49 67 107 56 67 54 110 83 86 99 115 114 118 97 75 109 111 115 106 103 101 98 72 98 70 74 51 85 82 56 73 110 106 50 117 115 100 67 107 97 104 119 111 107 86 75 105 99 70 104 107 98 50 117 78 116 107 79 74 77 98 120 48 10 76 114 66 69 84 76 70 49 122 55 101 106 84 84 66 76 77 65 52 71 65 49 85 100 68 119 69 66 47 119 81 69 65 119 73 72 103 68 65 77 66 103 78 86 72 82 77 66 65 102 56 69 65 106 65 65 77 67 115 71 65 49 85 100 10 73 119 81 107 77 67 75 65 73 67 116 103 78 89 97 48 101 113 115 90 100 104 65 102 102 77 79 121 88 57 115 112 118 43 98 89 78 98 52 71 110 108 51 105 100 51 121 50 49 105 57 107 77 65 111 71 67 67 113 71 83 77 52 57 10 66 65 77 67 65 48 99 65 77 69 81 67 73 66 78 111 84 82 120 89 51 52 51 120 82 110 51 109 73 120 82 100 106 105 79 106 85 106 113 76 74 114 56 56 84 76 88 102 65 80 70 122 48 73 98 108 65 105 66 81 66 57 53 53 10 66 120 77 68 79 51 56 54 106 75 76 99 73 114 53 115 51 74 53 68 122 55 115 81 113 121 111 120 85 90 105 121 66 80 116 51 79 103 61 61 10 45 45 45 45 45 69 78 68 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10] [79 114 103 49 77 83 80]}

			*/
			//fmt.Println("==============id.Identity===========",id.Identity)
			/*
			==============id.Identity=========== [10 7 79 114 103 49 77 83 80 18 170 6 45 45 45 45 45 66 69 71 73 78 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10 77 73 73 67 75 68 67 67 65 99 43 103 65 119 73 66 65 103 73 82 65 78 75 57 122 99 52 110 81 75 82 69 86 86 78 52 56 122 88 78 116 77 111 119 67 103 89 73 75 111 90 73 122 106 48 69 65 119 73 119 99 122 69 76 10 77 65 107 71 65 49 85 69 66 104 77 67 86 86 77 120 69 122 65 82 66 103 78 86 66 65 103 84 67 107 78 104 98 71 108 109 98 51 74 117 97 87 69 120 70 106 65 85 66 103 78 86 66 65 99 84 68 86 78 104 98 105 66 71 10 99 109 70 117 89 50 108 122 89 50 56 120 71 84 65 88 66 103 78 86 66 65 111 84 69 71 57 121 90 122 69 117 90 88 104 104 98 88 66 115 90 83 53 106 98 50 48 120 72 68 65 97 66 103 78 86 66 65 77 84 69 50 78 104 10 76 109 57 121 90 122 69 117 90 88 104 104 98 88 66 115 90 83 53 106 98 50 48 119 72 104 99 78 77 106 69 120 77 84 69 49 77 68 107 121 78 122 65 119 87 104 99 78 77 122 69 120 77 84 69 122 77 68 107 121 78 122 65 119 10 87 106 66 113 77 81 115 119 67 81 89 68 86 81 81 71 69 119 74 86 85 122 69 84 77 66 69 71 65 49 85 69 67 66 77 75 81 50 70 115 97 87 90 118 99 109 53 112 89 84 69 87 77 66 81 71 65 49 85 69 66 120 77 78 10 85 50 70 117 73 69 90 121 89 87 53 106 97 88 78 106 98 122 69 78 77 65 115 71 65 49 85 69 67 120 77 69 99 71 86 108 99 106 69 102 77 66 48 71 65 49 85 69 65 120 77 87 99 71 86 108 99 106 65 117 98 51 74 110 10 77 83 53 108 101 71 70 116 99 71 120 108 76 109 78 118 98 84 66 90 77 66 77 71 66 121 113 71 83 77 52 57 65 103 69 71 67 67 113 71 83 77 52 57 65 119 69 72 65 48 73 65 66 74 86 107 51 65 83 72 80 80 82 47 10 69 49 67 107 56 67 54 110 83 86 99 115 114 118 97 75 109 111 115 106 103 101 98 72 98 70 74 51 85 82 56 73 110 106 50 117 115 100 67 107 97 104 119 111 107 86 75 105 99 70 104 107 98 50 117 78 116 107 79 74 77 98 120 48 10 76 114 66 69 84 76 70 49 122 55 101 106 84 84 66 76 77 65 52 71 65 49 85 100 68 119 69 66 47 119 81 69 65 119 73 72 103 68 65 77 66 103 78 86 72 82 77 66 65 102 56 69 65 106 65 65 77 67 115 71 65 49 85 100 10 73 119 81 107 77 67 75 65 73 67 116 103 78 89 97 48 101 113 115 90 100 104 65 102 102 77 79 121 88 57 115 112 118 43 98 89 78 98 52 71 110 108 51 105 100 51 121 50 49 105 57 107 77 65 111 71 67 67 113 71 83 77 52 57 10 66 65 77 67 65 48 99 65 77 69 81 67 73 66 78 111 84 82 120 89 51 52 51 120 82 110 51 109 73 120 82 100 106 105 79 106 85 106 113 76 74 114 56 56 84 76 88 102 65 80 70 122 48 73 98 108 65 105 66 81 66 57 53 53 10 66 120 77 68 79 51 56 54 106 75 76 99 73 114 53 115 51 74 53 68 122 55 115 81 113 121 111 120 85 90 105 121 66 80 116 51 79 103 61 61 10 45 45 45 45 45 69 78 68 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10]
			*/
			//fmt.Println("==============aliveInfo.Envelope===========",aliveInfo.Envelope)
			/*
			==========aliveInfo============= Endpoint: peer0.org1.example.com:7051, InternalEndpoint: , PKI-ID: 6f24a285101adea8c6350decbca2c731f07978a32a5ca876c3f2cca7b5b3e090, Metadata:

			*/
			peersForCurrentOrg[string(id.PKIId)] = &discovery.Peer{
				Identity:       id.Identity,
				MembershipInfo: aliveInfo.Envelope,
			}
			fmt.Println("=====peersForCurrentOrg=========",peersForCurrentOrg)
		}
	}

	return peersByOrg
}

// validateStructure validates that the request contains all the needed fields and that they are computed correctly
func validateStructure(ctx context.Context, request *discovery.SignedRequest, tlsEnabled bool, certHashFromContext certHashExtractor) (*discovery.Request, error) {
	fmt.Println("==validateStructure==")
	if request == nil {
		return nil, errors.New("nil request")
	}
	req, err := request.ToRequest()
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing request")
	}
	if req.Authentication == nil {
		return nil, errors.New("access denied, no authentication info in request")
	}
	if len(req.Authentication.ClientIdentity) == 0 {
		return nil, errors.New("access denied, client identity wasn't supplied")
	}
	if !tlsEnabled {
		return req, nil
	}
	computedHash := certHashFromContext(ctx)
	if len(computedHash) == 0 {
		return nil, errors.New("client didn't send a TLS certificate")
	}
	if !bytes.Equal(computedHash, req.Authentication.ClientTlsCertHash) {
		claimed := hex.EncodeToString(req.Authentication.ClientTlsCertHash)
		logger.Warningf("client claimed TLS hash %s doesn't match computed TLS hash from gRPC stream %s", claimed, hex.EncodeToString(computedHash))
		return nil, errors.New("client claimed TLS hash doesn't match computed TLS hash from gRPC stream")
	}
	return req, nil
}

func validateCCQuery(ccQuery *discovery.ChaincodeQuery) error {
	fmt.Println("==validateCCQuery==")
	if len(ccQuery.Interests) == 0 {
		return errors.New("chaincode query must have at least one chaincode interest")
	}
	for _, interest := range ccQuery.Interests {
		fmt.Println("=========interest=======")
		if interest == nil {
			fmt.Println("====interest == nil==========")
			return errors.New("chaincode interest is nil")
		}

		if len(interest.Chaincodes) == 0 {
			fmt.Println("====len(interest.Chaincodes) == 0=========")
			return errors.New("chaincode interest must contain at least one chaincode")
		}
		for _, cc := range interest.Chaincodes {
			fmt.Println("====cc=========",cc)
			fmt.Println("====cc.name=========",cc.Name)
			if cc.Name == "" {
				return errors.New("chaincode name in interest cannot be empty")
			}
		}
	}
	return nil
}

func wrapError(err error) *discovery.QueryResult {
	fmt.Println("==wrapError==")
	return &discovery.QueryResult{
		Result: &discovery.QueryResult_Error{
			Error: &discovery.Error{
				Content: err.Error(),
			},
		},
	}
}
