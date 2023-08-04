package core

import (
	"errors"
	"fmt"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
)

const (
	INOX_MODULE_RES_KIND         = "inox/module"
	INOX_INCLUDED_CHUNK_RES_KIND = "inox/included-chunk"

	CHUNK_IMPORT_MOD_REL = "inox/import-module"
	CHUNK_INCLUDE_REL    = "inox/include-chunk"
)

type ResourceGraph struct {
	directed     *simple.DirectedGraph
	resourceToId map[string]int64
	roots        map[int64]struct{}
}

func NewResourceGraph() *ResourceGraph {
	return &ResourceGraph{
		directed:     simple.NewDirectedGraph(),
		resourceToId: make(map[string]int64, 0),
		roots:        make(map[int64]struct{}, 0),
	}
}

func AddModuleTreeToResourceGraph(m *Module, g *ResourceGraph, ctx *Context, ignoreBadImports bool) error {
	modRes := ResourceNameFrom(m.MainChunk.Source.Name())

	g.AddResource(modRes, INOX_MODULE_RES_KIND)

	//add included chunks

	var addChunkTree func(c *IncludedChunk, parent any)
	addChunkTree = func(c *IncludedChunk, parent any) {
		res := ResourceNameFrom(c.Source.Name())

		g.AddResource(res, INOX_INCLUDED_CHUNK_RES_KIND)

		if _, ok := parent.(*Module); ok {
			g.AddEdge(modRes, res, CHUNK_INCLUDE_REL)
		} else {
			parentChunk := ResourceNameFrom(parent.(*IncludedChunk).Source.Name())
			g.AddEdge(parentChunk, res, CHUNK_INCLUDE_REL)
		}

		for _, childChunk := range c.IncludedChunkForest {
			addChunkTree(childChunk, c)
		}
	}

	for _, chunk := range m.IncludedChunkForest {
		addChunkTree(chunk, m)
	}

	//add imported modules

	for _, stmt := range m.ImportStatements() {
		src, ok := stmt.SourceString()
		if !ok {
			if !ignoreBadImports {
				return errors.New("an import has an invalid .Source node")
			}
			continue
		}

		importModRes := ResourceNameFrom(src)
		if _, ok := importModRes.(Path); !ok {
			if !ignoreBadImports {
				return errors.New("only paths are supported as module import sources")
			}
			continue
		}

		importedMod, err := ParseLocalModule(importModRes.(Path).UnderlyingString(), ModuleParsingConfig{
			Context:                             ctx,
			RecoverFromNonExistingIncludedFiles: ignoreBadImports,
		})

		if err != nil {
			if !ignoreBadImports {
				return err
			}
			if importedMod == nil {
				continue
			}
		}

		if err := AddModuleTreeToResourceGraph(importedMod, g, ctx, ignoreBadImports); err != nil {
			return err
		}

		g.AddEdge(modRes, importModRes, CHUNK_IMPORT_MOD_REL)
	}

	return nil
}

type ResourceNode struct {
	graph.Node
	kind string
	r    ResourceName
}

func (r ResourceNode) Kind() string {
	return r.kind
}

type ResourceGraphEdge struct {
	from, to *ResourceNode
	kind     string
}

func (r ResourceGraphEdge) Kind() string {
	return r.kind
}

func (e ResourceGraphEdge) From() graph.Node {
	return e.from
}

func (e ResourceGraphEdge) To() graph.Node {
	return e.to
}

func (e ResourceGraphEdge) ReversedEdge() graph.Edge {
	//graph.Edge:
	// ReversedEdge returns the edge reversal of the receiver
	// if a reversal is valid for the data type.
	// When a reversal is valid an edge of the same type as
	// the receiver with nodes of the receiver swapped should
	// be returned, otherwise the receiver should be returned
	// unaltered.
	return e
}

func (g *ResourceGraph) getResourceNode(r ResourceName) *ResourceNode {
	id, ok := g.resourceToId[r.ResourceName()]
	if !ok {
		panic(fmtResourceNotInGraph(r))
	}
	node := g.directed.Node(id)
	if node == nil {
		panic(ErrUnreachable)
	}

	return node.(*ResourceNode)
}

func (g *ResourceGraph) AddResource(r ResourceName, kind string) {
	name := r.ResourceName()

	if name == "" {
		panic(errors.New("empty resource name"))
	}

	_, ok := g.resourceToId[name]
	if ok {
		return
	}
	node := &ResourceNode{
		r:    r,
		kind: kind,
		Node: g.directed.NewNode(),
	}

	g.directed.AddNode(node)
	g.resourceToId[name] = node.ID()
	g.roots[node.ID()] = struct{}{}
}

func (g *ResourceGraph) AddEdge(from, to ResourceName, rel string) {
	fromNode := g.getResourceNode(from)
	toNode := g.getResourceNode(to)

	if g.directed.HasEdgeFromTo(fromNode.ID(), toNode.ID()) {
		return
	}

	edge := ResourceGraphEdge{from: fromNode, to: toNode, kind: rel}
	g.directed.SetEdge(edge)
	delete(g.roots, toNode.ID())
}

func (g *ResourceGraph) GetNode(r ResourceName) (*ResourceNode, bool) {
	id, ok := g.resourceToId[r.ResourceName()]
	if !ok {
		return nil, false
	}

	return g.directed.Node(id).(*ResourceNode), true
}

func (g *ResourceGraph) Roots() (roots []*ResourceNode) {
	for id := range g.roots {
		roots = append(roots, g.directed.Node(id).(*ResourceNode))
	}

	return
}

func (g *ResourceGraph) GetEdge(from, to ResourceName) (ResourceGraphEdge, bool) {
	fromNode := g.getResourceNode(from)
	toNode := g.getResourceNode(to)

	if !g.directed.HasEdgeFromTo(fromNode.ID(), toNode.ID()) {
		return ResourceGraphEdge{}, false
	}

	return g.directed.Edge(fromNode.ID(), toNode.ID()).(ResourceGraphEdge), true
}

func fmtResourceNotInGraph(r ResourceName) error {
	return fmt.Errorf("resource `%s` is not in the graph", r)
}
