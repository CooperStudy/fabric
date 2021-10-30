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
	return &Vertex{
		Id:        id,
		Data:      data,
		neighbors: make(map[string]*Vertex),
	}
}

// NeighborById returns a neighbor vertex with the given id,
// or nil if no vertex with such an id is a neighbor
func (v *Vertex) NeighborById(id string) *Vertex {
	fmt.Println("====Vertex=====NeighborById===========")
	return v.neighbors[id]
}

// Neighbors returns the neighbors of the vertex
func (v *Vertex) Neighbors() []*Vertex {
	fmt.Println("====Vertex=====Neighbors===========")
	var res []*Vertex
	for _, u := range v.neighbors {
		res = append(res, u)
	}
	return res
}

// AddNeighbor adds the given vertex as a neighbor
// of the vertex
func (v *Vertex) AddNeighbor(u *Vertex) {
	fmt.Println("====Vertex=====AddNeighbor===========")
	v.neighbors[u.Id] = u
	u.neighbors[v.Id] = v
}
