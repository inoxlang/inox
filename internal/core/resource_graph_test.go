package core

import (
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestAddModuleTreeToResourceGraph(t *testing.T) {
	t.Run("empty module", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		mod := utils.Must(ParseInMemoryModule("manifest {}", InMemoryModuleParsingConfig{
			Name:    "/main.ix",
			Context: ctx,
		}))

		g := NewResourceGraph()
		AddModuleTreeToResourceGraph(mod, g, ctx, false)

		node, ok := g.GetNode(ResourceNameFrom("/main.ix"))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, INOX_MODULE_RES_KIND, node.Kind())

		roots := g.Roots()
		if !assert.Len(t, roots, 1) {
			return
		}

		assert.Same(t, node, roots[0])
	})

	t.Run("module includes a chunk", func(t *testing.T) {
		fls := newMemFilesystem()
		utils.PanicIfErr(util.WriteFile(fls, "/main.ix", []byte("manifest {}; import ./chunk.ix"), 0600))
		utils.PanicIfErr(util.WriteFile(fls, "/chunk.ix", []byte("includable-chunk"), 0600))

		ctx := NewContexWithEmptyState(ContextConfig{
			Filesystem:  fls,
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
		}, nil)
		defer ctx.CancelGracefully()

		mod := utils.Must(ParseLocalModule("/main.ix", ModuleParsingConfig{
			Context: ctx,
		}))

		g := NewResourceGraph()
		AddModuleTreeToResourceGraph(mod, g, ctx, false)

		//check nodes
		moduleNode, ok := g.GetNode(ResourceNameFrom("/main.ix"))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, INOX_MODULE_RES_KIND, moduleNode.Kind())

		roots := g.Roots()
		if !assert.Len(t, roots, 1) {
			return
		}

		assert.Same(t, moduleNode, roots[0])

		chunkNode, ok := g.GetNode(ResourceNameFrom("/chunk.ix"))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, INOX_INCLUDED_CHUNK_RES_KIND, chunkNode.Kind())

		//check edge

		edge, ok := g.GetEdge(ResourceNameFrom("/main.ix"), ResourceNameFrom("/chunk.ix"))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, CHUNK_INCLUDE_REL, edge.Data)

		//reversed edge should not exist
		_, ok = g.GetEdge(ResourceNameFrom("/chunk.ix"), ResourceNameFrom("/main.ix"))
		assert.False(t, ok)
	})

	t.Run("module includes two chunks", func(t *testing.T) {
		fls := newMemFilesystem()
		utils.PanicIfErr(util.WriteFile(fls, "/main.ix", []byte("manifest {}; import ./chunk1.ix; import ./chunk2.ix"), 0600))
		utils.PanicIfErr(util.WriteFile(fls, "/chunk1.ix", []byte("includable-chunk"), 0600))
		utils.PanicIfErr(util.WriteFile(fls, "/chunk2.ix", []byte("includable-chunk"), 0600))

		ctx := NewContexWithEmptyState(ContextConfig{
			Filesystem:  fls,
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
		}, nil)
		defer ctx.CancelGracefully()

		mod := utils.Must(ParseLocalModule("/main.ix", ModuleParsingConfig{
			Context: ctx,
		}))

		g := NewResourceGraph()
		AddModuleTreeToResourceGraph(mod, g, ctx, false)

		//check nodes
		moduleNode, ok := g.GetNode(ResourceNameFrom("/main.ix"))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, INOX_MODULE_RES_KIND, moduleNode.Kind())

		roots := g.Roots()
		if !assert.Len(t, roots, 1) {
			return
		}

		assert.Same(t, moduleNode, roots[0])

		chunkNode1, ok := g.GetNode(ResourceNameFrom("/chunk1.ix"))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, INOX_INCLUDED_CHUNK_RES_KIND, chunkNode1.Kind())

		chunkNode2, ok := g.GetNode(ResourceNameFrom("/chunk2.ix"))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, INOX_INCLUDED_CHUNK_RES_KIND, chunkNode2.Kind())

		//check edges

		edge1, ok := g.GetEdge(ResourceNameFrom("/main.ix"), ResourceNameFrom("/chunk1.ix"))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, CHUNK_INCLUDE_REL, edge1.Data)

		//reversed edge should not exist
		_, ok = g.GetEdge(ResourceNameFrom("/chunk1.ix"), ResourceNameFrom("/main.ix"))
		assert.False(t, ok)

		edge2, ok := g.GetEdge(ResourceNameFrom("/main.ix"), ResourceNameFrom("/chunk2.ix"))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, CHUNK_INCLUDE_REL, edge2.Data)

		//reversed edge should not exist
		_, ok = g.GetEdge(ResourceNameFrom("/chunk2.ix"), ResourceNameFrom("/main.ix"))
		assert.False(t, ok)
	})

	t.Run("module includes a chunk that includes a chunk", func(t *testing.T) {
		fls := newMemFilesystem()
		utils.PanicIfErr(util.WriteFile(fls, "/main.ix", []byte("manifest {}; import ./chunk1.ix"), 0600))
		utils.PanicIfErr(util.WriteFile(fls, "/chunk1.ix", []byte("includable-chunk\n import ./chunk2.ix"), 0600))
		utils.PanicIfErr(util.WriteFile(fls, "/chunk2.ix", []byte("includable-chunk"), 0600))

		ctx := NewContexWithEmptyState(ContextConfig{
			Filesystem:  fls,
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
		}, nil)
		defer ctx.CancelGracefully()

		mod := utils.Must(ParseLocalModule("/main.ix", ModuleParsingConfig{
			Context: ctx,
		}))

		g := NewResourceGraph()
		AddModuleTreeToResourceGraph(mod, g, ctx, false)

		//check nodes
		moduleNode, ok := g.GetNode(ResourceNameFrom("/main.ix"))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, INOX_MODULE_RES_KIND, moduleNode.Kind())

		roots := g.Roots()
		if !assert.Len(t, roots, 1) {
			return
		}

		assert.Same(t, moduleNode, roots[0])

		chunkNode1, ok := g.GetNode(ResourceNameFrom("/chunk1.ix"))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, INOX_INCLUDED_CHUNK_RES_KIND, chunkNode1.Kind())

		chunkNode2, ok := g.GetNode(ResourceNameFrom("/chunk2.ix"))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, INOX_INCLUDED_CHUNK_RES_KIND, chunkNode2.Kind())

		//check edges

		edge1, ok := g.GetEdge(ResourceNameFrom("/main.ix"), ResourceNameFrom("/chunk1.ix"))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, CHUNK_INCLUDE_REL, edge1.Data)

		//reversed edge should not exist
		_, ok = g.GetEdge(ResourceNameFrom("/chunk1.ix"), ResourceNameFrom("/main.ix"))
		assert.False(t, ok)

		edge2, ok := g.GetEdge(ResourceNameFrom("/chunk1.ix"), ResourceNameFrom("/chunk2.ix"))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, CHUNK_INCLUDE_REL, edge2.Data)

		//reversed edge should not exist
		_, ok = g.GetEdge(ResourceNameFrom("/chunk2.ix"), ResourceNameFrom("/chunk1.ix"))
		assert.False(t, ok)
	})

	t.Run("module imports another module", func(t *testing.T) {
		fls := newMemFilesystem()
		utils.PanicIfErr(util.WriteFile(fls, "/main.ix", []byte("manifest {}; import lib /lib.ix {}"), 0600))
		utils.PanicIfErr(util.WriteFile(fls, "/lib.ix", []byte("manifest {}"), 0600))

		ctx := NewContexWithEmptyState(ContextConfig{
			Filesystem:  fls,
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
		}, nil)
		defer ctx.CancelGracefully()

		mod := utils.Must(ParseLocalModule("/main.ix", ModuleParsingConfig{
			Context: ctx,
		}))

		g := NewResourceGraph()
		AddModuleTreeToResourceGraph(mod, g, ctx, false)

		//check nodes
		moduleNode1, ok := g.GetNode(ResourceNameFrom("/main.ix"))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, INOX_MODULE_RES_KIND, moduleNode1.Kind())

		roots := g.Roots()
		if !assert.Len(t, roots, 1) {
			return
		}

		assert.Same(t, moduleNode1, roots[0])

		moduleNode2, ok := g.GetNode(ResourceNameFrom("/lib.ix"))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, INOX_MODULE_RES_KIND, moduleNode2.Kind())

		//check edge

		edge, ok := g.GetEdge(ResourceNameFrom("/main.ix"), ResourceNameFrom("/lib.ix"))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, CHUNK_IMPORT_MOD_REL, edge.Data)

		//reversed edge should not exist
		_, ok = g.GetEdge(ResourceNameFrom("/lib.ix"), ResourceNameFrom("/main.ix"))
		assert.False(t, ok)
	})

}
