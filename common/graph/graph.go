/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package graph

import "fmt"

// Vertex defines a vertex of a graph
type Vertex struct {
	Id        string
	Data      interface{}
	neighbors map[string]*Vertex
}

// NewVertex creates a new vertex with given id and data
func NewVertex(id string, data interface{}) *Vertex {
	logger.Info("====NewVertex================")
	logger.Info("=========id",id)//G1 G2 G3 G0
	fmt.Printf("=========data:%T\n",data)//*msp.MSPPrincipal
	logger.Info("===============图=Vertex============")
	a := Vertex{
		Id:        id,
		Data:      data,
		neighbors: make(map[string]*Vertex),
	}
	return &a
}

// NeighborById returns a neighbor vertex with the given id,
// or nil if no vertex with such an id is a neighbor
func (v *Vertex) NeighborById(id string) *Vertex {
	logger.Info("====Vertex=====NeighborById===========")
	logger.Info("===id==",id)
	a:= v.neighbors[id]
	logger.Info("======v.neighbors[id]===============",v.neighbors[id])
	return a
}

// Neighbors returns the neighbors of the vertex
func (v *Vertex) Neighbors() []*Vertex {
	logger.Info("====Vertex=====Neighbors===========")
	var res []*Vertex
	for k, u := range v.neighbors {
		logger.Info("==========k",k)
		logger.Info("==========v",v)
		res = append(res, u)
	}
	logger.Info("=====res",res)
	/*
	 [0xc002411950 0xc0024118f0]g1MSP\020\003"  map[Pv�p�.�4:�
	*/
	return res
}

// AddNeighbor adds the given vertex as a neighbor
// of the vertex
func (v *Vertex) AddNeighbor(u *Vertex) {
	logger.Info("====Vertex=====AddNeighbor===========")
	logger.Info("==u.id==",u.Id)
	logger.Info("==u==",u)

	logger.Info("==v.id==",v.Id)
	logger.Info("==v==",v)
	v.neighbors[u.Id] = u
	u.neighbors[v.Id] = v
}
