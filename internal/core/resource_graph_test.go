package core

import (
	"os"
	"path/filepath"
	"testing"

	utils "github.com/inoxlang/inox/internal/utils/common"
	"github.com/stretchr/testify/assert"
)

func TestAddModuleTreeToResourceGraph(t *testing.T) {
	t.Run("empty module", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
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
		dir := t.TempDir()
		mainModPath := filepath.Join(dir, "main.ix")
		includeFilePath := filepath.Join(dir, "chunk.ix")

		utils.PanicIfErr(os.WriteFile(mainModPath, []byte("manifest {}; import ./chunk.ix"), 0600))
		utils.PanicIfErr(os.WriteFile(includeFilePath, []byte("includable-file"), 0600))

		ctx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
		}, nil)
		defer ctx.CancelGracefully()

		mod := utils.Must(ParseLocalModule(mainModPath, ModuleParsingConfig{
			Context: ctx,
		}))

		g := NewResourceGraph()
		AddModuleTreeToResourceGraph(mod, g, ctx, false)

		//check nodes
		moduleNode, ok := g.GetNode(ResourceNameFrom(mainModPath))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, INOX_MODULE_RES_KIND, moduleNode.Kind())

		roots := g.Roots()
		if !assert.Len(t, roots, 1) {
			return
		}

		assert.Same(t, moduleNode, roots[0])

		chunkNode, ok := g.GetNode(ResourceNameFrom(includeFilePath))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, INOX_INCLUDED_CHUNK_RES_KIND, chunkNode.Kind())

		//check edge

		edge, ok := g.GetEdge(ResourceNameFrom(mainModPath), ResourceNameFrom(includeFilePath))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, CHUNK_INCLUDE_REL, edge.Data)

		//reversed edge should not exist
		_, ok = g.GetEdge(ResourceNameFrom(includeFilePath), ResourceNameFrom(mainModPath))
		assert.False(t, ok)
	})

	t.Run("module includes two chunks", func(t *testing.T) {
		dir := t.TempDir()
		mainModPath := filepath.Join(dir, "main.ix")
		includeFile1Path := filepath.Join(dir, "chunk1.ix")
		includeFile2Path := filepath.Join(dir, "chunk2.ix")

		utils.PanicIfErr(os.WriteFile(mainModPath, []byte("manifest {}; import ./chunk1.ix; import ./chunk2.ix"), 0600))
		utils.PanicIfErr(os.WriteFile(includeFile1Path, []byte("includable-file"), 0600))
		utils.PanicIfErr(os.WriteFile(includeFile2Path, []byte("includable-file"), 0600))

		ctx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
		}, nil)
		defer ctx.CancelGracefully()

		mod := utils.Must(ParseLocalModule(mainModPath, ModuleParsingConfig{
			Context: ctx,
		}))

		g := NewResourceGraph()
		AddModuleTreeToResourceGraph(mod, g, ctx, false)

		//check nodes
		moduleNode, ok := g.GetNode(ResourceNameFrom(mainModPath))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, INOX_MODULE_RES_KIND, moduleNode.Kind())

		roots := g.Roots()
		if !assert.Len(t, roots, 1) {
			return
		}

		assert.Same(t, moduleNode, roots[0])

		chunkNode1, ok := g.GetNode(ResourceNameFrom(includeFile1Path))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, INOX_INCLUDED_CHUNK_RES_KIND, chunkNode1.Kind())

		chunkNode2, ok := g.GetNode(ResourceNameFrom(includeFile2Path))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, INOX_INCLUDED_CHUNK_RES_KIND, chunkNode2.Kind())

		//check edges

		edge1, ok := g.GetEdge(ResourceNameFrom(mainModPath), ResourceNameFrom(includeFile1Path))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, CHUNK_INCLUDE_REL, edge1.Data)

		//reversed edge should not exist
		_, ok = g.GetEdge(ResourceNameFrom(includeFile1Path), ResourceNameFrom(mainModPath))
		assert.False(t, ok)

		edge2, ok := g.GetEdge(ResourceNameFrom(mainModPath), ResourceNameFrom(includeFile2Path))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, CHUNK_INCLUDE_REL, edge2.Data)

		//reversed edge should not exist
		_, ok = g.GetEdge(ResourceNameFrom(includeFile2Path), ResourceNameFrom(mainModPath))
		assert.False(t, ok)
	})

	t.Run("module includes a chunk that includes a chunk", func(t *testing.T) {
		dir := t.TempDir()
		mainModPath := filepath.Join(dir, "main.ix")
		includeFile1Path := filepath.Join(dir, "chunk1.ix")
		includeFile2Path := filepath.Join(dir, "chunk2.ix")

		utils.PanicIfErr(os.WriteFile(mainModPath, []byte("manifest {}; import ./chunk1.ix"), 0600))
		utils.PanicIfErr(os.WriteFile(includeFile1Path, []byte("includable-file\n import ./chunk2.ix"), 0600))
		utils.PanicIfErr(os.WriteFile(includeFile2Path, []byte("includable-file"), 0600))

		ctx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
		}, nil)
		defer ctx.CancelGracefully()

		mod := utils.Must(ParseLocalModule(mainModPath, ModuleParsingConfig{
			Context: ctx,
		}))

		g := NewResourceGraph()
		AddModuleTreeToResourceGraph(mod, g, ctx, false)

		//check nodes
		moduleNode, ok := g.GetNode(ResourceNameFrom(mainModPath))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, INOX_MODULE_RES_KIND, moduleNode.Kind())

		roots := g.Roots()
		if !assert.Len(t, roots, 1) {
			return
		}

		assert.Same(t, moduleNode, roots[0])

		chunkNode1, ok := g.GetNode(ResourceNameFrom(includeFile1Path))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, INOX_INCLUDED_CHUNK_RES_KIND, chunkNode1.Kind())

		chunkNode2, ok := g.GetNode(ResourceNameFrom(includeFile2Path))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, INOX_INCLUDED_CHUNK_RES_KIND, chunkNode2.Kind())

		//check edges

		edge1, ok := g.GetEdge(ResourceNameFrom(mainModPath), ResourceNameFrom(includeFile1Path))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, CHUNK_INCLUDE_REL, edge1.Data)

		//reversed edge should not exist
		_, ok = g.GetEdge(ResourceNameFrom(includeFile1Path), ResourceNameFrom(mainModPath))
		assert.False(t, ok)

		edge2, ok := g.GetEdge(ResourceNameFrom(includeFile1Path), ResourceNameFrom(includeFile2Path))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, CHUNK_INCLUDE_REL, edge2.Data)

		//reversed edge should not exist
		_, ok = g.GetEdge(ResourceNameFrom(includeFile2Path), ResourceNameFrom(includeFile1Path))
		assert.False(t, ok)
	})

	t.Run("module imports another module", func(t *testing.T) {
		dir := t.TempDir()
		mainModPath := filepath.Join(dir, "main.ix")
		libPath := filepath.Join(dir, "lib.ix")

		utils.PanicIfErr(os.WriteFile(mainModPath, []byte("manifest {}; import lib "+libPath+" {}"), 0600))
		utils.PanicIfErr(os.WriteFile(libPath, []byte("manifest {}"), 0600))

		ctx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
		}, nil)
		defer ctx.CancelGracefully()

		mod := utils.Must(ParseLocalModule(mainModPath, ModuleParsingConfig{
			Context: ctx,
		}))

		g := NewResourceGraph()
		AddModuleTreeToResourceGraph(mod, g, ctx, false)

		//check nodes
		moduleNode1, ok := g.GetNode(ResourceNameFrom(mainModPath))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, INOX_MODULE_RES_KIND, moduleNode1.Kind())

		roots := g.Roots()
		if !assert.Len(t, roots, 1) {
			return
		}

		assert.Same(t, moduleNode1, roots[0])

		moduleNode2, ok := g.GetNode(ResourceNameFrom(libPath))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, INOX_MODULE_RES_KIND, moduleNode2.Kind())

		//check edge

		edge, ok := g.GetEdge(ResourceNameFrom(mainModPath), ResourceNameFrom(libPath))
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, CHUNK_IMPORT_MOD_REL, edge.Data)

		//reversed edge should not exist
		_, ok = g.GetEdge(ResourceNameFrom(libPath), ResourceNameFrom(mainModPath))
		assert.False(t, ok)
	})

}
