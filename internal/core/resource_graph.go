package core

import (
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/in_mem_ds"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	INOX_MODULE_RES_KIND         = "inox/module"
	INOX_INCLUDED_CHUNK_RES_KIND = "inox/included-chunk"

	CHUNK_IMPORT_MOD_REL = ResourceRelationKind("inox/import-module")
	CHUNK_INCLUDE_REL    = ResourceRelationKind("inox/include-chunk")
)

type ResourceGraph struct {
	directed     *in_mem_ds.DirectedGraph[*ResourceNode, ResourceRelationKind]
	resourceToId map[string]in_mem_ds.NodeId
	roots        map[in_mem_ds.NodeId]struct{}
}

type ResourceRelationKind string

func NewResourceGraph() *ResourceGraph {
	return &ResourceGraph{
		directed:     in_mem_ds.NewDirectedGraph[*ResourceNode, ResourceRelationKind](in_mem_ds.ThreadUnsafe),
		resourceToId: make(map[string]in_mem_ds.NodeId, 0),
		roots:        make(map[in_mem_ds.NodeId]struct{}, 0),
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
	kind string
	r    ResourceName
	id   in_mem_ds.NodeId
}

func (r ResourceNode) Kind() string {
	return r.kind
}

func (g *ResourceGraph) getResourceNode(r ResourceName) *ResourceNode {
	id, ok := g.resourceToId[r.ResourceName()]
	if !ok {
		panic(fmtResourceNotInGraph(r))
	}
	node, ok := g.directed.Node(id)
	if !ok {
		panic(ErrUnreachable)
	}

	return node.Data
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
	}

	id := g.directed.AddNode(node)
	node.id = id

	g.resourceToId[name] = id
	g.roots[id] = struct{}{}
}

func (g *ResourceGraph) AddEdge(from, to ResourceName, rel ResourceRelationKind) {
	fromNode := g.getResourceNode(from)
	toNode := g.getResourceNode(to)

	if g.directed.HasEdgeFromTo(fromNode.id, toNode.id) {
		return
	}

	g.directed.SetEdge(fromNode.id, toNode.id, ResourceRelationKind(rel))
	delete(g.roots, toNode.id)
}

func (g *ResourceGraph) GetNode(r ResourceName) (*ResourceNode, bool) {
	id, ok := g.resourceToId[r.ResourceName()]
	if !ok {
		return nil, false
	}

	return g.directed.NodeData(id)
}

func (g *ResourceGraph) Roots() (roots []*ResourceNode) {
	for id := range g.roots {
		roots = append(roots, utils.MustGet(g.directed.NodeData(id)))
	}

	return
}

func (g *ResourceGraph) GetEdge(from, to ResourceName) (in_mem_ds.GraphEdge[ResourceRelationKind], bool) {
	fromNode := g.getResourceNode(from)
	toNode := g.getResourceNode(to)

	return g.directed.Edge(fromNode.id, toNode.id)
}

func fmtResourceNotInGraph(r ResourceName) error {
	return fmt.Errorf("resource `%s` is not in the graph", r)
}
