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
	fmt.Println("====NewVertex================")
	fmt.Println("=========id",id)//G1 G2 G3 G0
	fmt.Printf("=========data:%T\n",data)//*msp.MSPPrincipal
	fmt.Println("===============图=Vertex============")
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
	fmt.Println("====Vertex=====NeighborById===========")
	fmt.Println("===id==",id)
	a:= v.neighbors[id]
	fmt.Println("======v.neighbors[id]===============",v.neighbors[id])
	return a
}

// Neighbors returns the neighbors of the vertex
func (v *Vertex) Neighbors() []*Vertex {
	fmt.Println("====Vertex=====Neighbors===========")
	var res []*Vertex
	for k, u := range v.neighbors {
		fmt.Println("==========k",k)
		fmt.Println("==========v",v)
		res = append(res, u)
	}
	fmt.Println("=====res",res)//[]
	return res
}

// AddNeighbor adds the given vertex as a neighbor
// of the vertex
func (v *Vertex) AddNeighbor(u *Vertex) {
	fmt.Println("====Vertex=====AddNeighbor===========")
	fmt.Println("==u.id==",u.Id)
	fmt.Println("==u==",u)

	fmt.Println("==v.id==",v.Id)
	fmt.Println("==v==",v)
	v.neighbors[u.Id] = u
	u.neighbors[v.Id] = v
}
