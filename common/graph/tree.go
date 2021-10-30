/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package graph

import "fmt"

// Iterator defines an iterator that can be used to traverse vertices
// of a graph
type Iterator interface {
	// Next returns the next element in the iteration order,
	// or nil if there is no such an element
	Next() *TreeVertex
}

// TreeVertex defines a vertex of a tree
type TreeVertex struct {
	Id          string        // id identifies uniquely the TreeVertex in the Tree
	Data        interface{}   // data holds arbitrary data, to be used by the user of the package
	Descendants []*TreeVertex // descendants are the vertices that this TreeVertex is their parent in the tree
	Threshold   int           // threshold symbols the count of sub-trees / leaves to pick when creating tree permutations
}

// NewTreeVertex creates a new vertex with a given unique id and a given arbitrary data
func NewTreeVertex(id string, data interface{}, descendants ...*TreeVertex) *TreeVertex {
	return &TreeVertex{
		Id:          id,
		Data:        data,
		Descendants: descendants,
	}
}

// IsLeaf returns whether the given vertex is a leaf
func (v *TreeVertex) IsLeaf() bool {
	fmt.Println("====TreeVertex====IsLeaf=======")
	return len(v.Descendants) == 0
}

// AddDescendant creates a new vertex who's parent is the invoker vertex,
// with a given id and data. Returns the new vertex
func (v *TreeVertex) AddDescendant(u *TreeVertex) *TreeVertex {
	fmt.Println("====TreeVertex====AddDescendant=======")
	v.Descendants = append(v.Descendants, u)
	return u
}

// ToTree creates a Tree who's root vertex is the current vertex
func (v *TreeVertex) ToTree() *Tree {
	fmt.Println("====TreeVertex====ToTree=======")
	return &Tree{
		Root: v,
	}
}

// Find searches for a vertex who's id is the given id.
// Returns the first vertex it finds with such an Id, or nil if not found
func (v *TreeVertex) Find(id string) *TreeVertex {
	fmt.Println("====TreeVertex====Find=======")
	if v.Id == id {
		return v
	}
	for _, u := range v.Descendants {
		if r := u.Find(id); r != nil {
			return r
		}
	}
	return nil
}

// Exists searches for a vertex who's id is the given id,
// and returns whether such a vertex was found or not.
func (v *TreeVertex) Exists(id string) bool {
	fmt.Println("====TreeVertex====Exists=======")
	return v.Find(id) != nil
}

// Clone clones the tree who's root vertex is the current vertex.
func (v *TreeVertex) Clone() *TreeVertex {
	fmt.Println("====TreeVertex====Clone=======")
	var descendants []*TreeVertex
	for _, u := range v.Descendants {
		descendants = append(descendants, u.Clone())
	}
	copy := &TreeVertex{
		Id:          v.Id,
		Descendants: descendants,
		Data:        v.Data,
	}
	return copy
}

// replace replaces the sub-tree of the vertex who's id is the given id
// with a sub-tree who's root vertex is r.
func (v *TreeVertex) replace(id string, r *TreeVertex) {
	fmt.Println("====TreeVertex====replace=======")
	if v.Id == id {
		v.Descendants = r.Descendants
		return
	}
	for _, u := range v.Descendants {
		u.replace(id, r)
	}
}

// Tree defines a Tree of vertices of type TreeVertex
type Tree struct {
	Root *TreeVertex
}

// Permute returns Trees that their vertices and edges all exist in the original tree.
// The permutations are calculated according to the thresholds of all vertices.
func (t *Tree) Permute() []*Tree {
	fmt.Println("====Tree====Permute=======")
	return newTreePermutation(t.Root).permute()
}

// BFS returns an iterator that iterates the vertices
// in a Breadth-First-Search order
func (t *Tree) BFS() Iterator {
	fmt.Println("====Tree====BFS=======")
	return newBFSIterator(t.Root)
}

type bfsIterator struct {
	*queue
}

func newBFSIterator(v *TreeVertex) *bfsIterator {
	fmt.Println("====newBFSIterator======")
	return &bfsIterator{
		queue: &queue{
			arr: []*TreeVertex{v},
		},
	}
}

// Next returns the next element in the iteration order,
// or nil if there is no such an element
func (bfs *bfsIterator) Next() *TreeVertex {
	fmt.Println("====newBFSIterator==Next====")
	if len(bfs.arr) == 0 {
		return nil
	}
	v := bfs.dequeue()
	for _, u := range v.Descendants {
		bfs.enqueue(u)
	}
	return v
}

// a primitive implementation of a queue backed by a slice
type queue struct {
	arr []*TreeVertex
}

func (q *queue) enqueue(v *TreeVertex) {
	fmt.Println("====queue==enqueue====")
	q.arr = append(q.arr, v)
}

func (q *queue) dequeue() *TreeVertex {
	fmt.Println("====queue==dequeue====")
	v := q.arr[0]
	q.arr = q.arr[1:]
	return v
}
