package fs_ns

import (
	"io"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestNewFilesystemSnapshot(t *testing.T) {

	t.Run("empty", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		desc := core.NewObjectFromMapNoInit(core.ValMap{
			FS_SNAPSHOT_SYMB_DESC_FILES_PROPNAME: core.NewDictionary(core.ValMap{}),
		})

		snapshot := NewFilesystemSnapshot(ctx, desc).Underlying()
		assert.Empty(t, snapshot.RootDirEntries())
	})

	t.Run("file in rootdir", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		desc := core.NewObjectFromMapNoInit(core.ValMap{
			FS_SNAPSHOT_SYMB_DESC_FILES_PROPNAME: core.NewDictionary(core.ValMap{
				core.GetJSONRepresentation(core.Path("./a.txt"), nil, nil): core.Str("a"),
			}),
		})

		snapshot := NewFilesystemSnapshot(ctx, desc).Underlying()

		if !assert.Len(t, snapshot.RootDirEntries(), 1) {
			return
		}

		metadata, err := snapshot.Metadata("/a.txt")
		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, metadata.IsRegularFile())
		assert.Equal(t, core.Path("/a.txt"), metadata.AbsolutePath)
		assert.Equal(t, core.ByteCount(1), metadata.Size)
		assert.Empty(t, metadata.ChildNames)

		addressableContent, err := snapshot.Content("/a.txt")
		if !assert.NoError(t, err) {
			return
		}
		content, err := io.ReadAll(addressableContent.Reader())
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, "a", string(content))
	})

	t.Run("empty subdir in root dir", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		desc := core.NewObjectFromMapNoInit(core.ValMap{
			FS_SNAPSHOT_SYMB_DESC_FILES_PROPNAME: core.NewDictionary(core.ValMap{
				core.GetJSONRepresentation(core.Path("./dir/"), nil, nil): core.NewWrappedValueList(),
			}),
		})

		snapshot := NewFilesystemSnapshot(ctx, desc).Underlying()

		if !assert.Len(t, snapshot.RootDirEntries(), 1) {
			return
		}

		metadata, err := snapshot.Metadata("/dir")
		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, metadata.IsDir())
		assert.Equal(t, core.Path("/dir/"), metadata.AbsolutePath)
		assert.Zero(t, metadata.Size)
		assert.Empty(t, metadata.ChildNames)
	})

	t.Run("subdir with file in root dir", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		desc := core.NewObjectFromMapNoInit(core.ValMap{
			FS_SNAPSHOT_SYMB_DESC_FILES_PROPNAME: core.NewDictionary(core.ValMap{
				core.GetJSONRepresentation(core.Path("./dir/"), nil, nil): core.NewWrappedValueList(core.Path("./file.txt")),
			}),
		})

		snapshot := NewFilesystemSnapshot(ctx, desc).Underlying()

		if !assert.Len(t, snapshot.RootDirEntries(), 1) {
			return
		}

		metadata, err := snapshot.Metadata("/dir")
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, metadata.IsDir()) {
			return
		}
		assert.Equal(t, core.Path("/dir/"), metadata.AbsolutePath)
		assert.Zero(t, metadata.Size)
		assert.Equal(t, []string{"file.txt"}, metadata.ChildNames)

		fileMetadata, err := snapshot.Metadata("/dir/file.txt")
		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, fileMetadata.IsRegularFile())
		assert.Equal(t, core.Path("/dir/file.txt"), fileMetadata.AbsolutePath)
		assert.Zero(t, fileMetadata.Size)
		assert.Empty(t, fileMetadata.ChildNames)

		addressableContent, err := snapshot.Content("/dir/file.txt")
		if !assert.NoError(t, err) {
			return
		}
		content, err := io.ReadAll(addressableContent.Reader())
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, string(content))
	})

	t.Run("empty subdir & file in root dir", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		desc := core.NewObjectFromMapNoInit(core.ValMap{
			FS_SNAPSHOT_SYMB_DESC_FILES_PROPNAME: core.NewDictionary(core.ValMap{
				core.GetJSONRepresentation(core.Path("./dir/"), nil, nil):  core.NewWrappedValueList(),
				core.GetJSONRepresentation(core.Path("./a.txt"), nil, nil): core.Str("a"),
			}),
		})

		snapshot := NewFilesystemSnapshot(ctx, desc).Underlying()

		if !assert.Len(t, snapshot.RootDirEntries(), 2) {
			return
		}

		metadata, err := snapshot.Metadata("/dir")
		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, metadata.IsDir())
		assert.Equal(t, core.Path("/dir/"), metadata.AbsolutePath)
		assert.Zero(t, metadata.Size)
		assert.Empty(t, metadata.ChildNames)

		fileMetadata, err := snapshot.Metadata("/a.txt")
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, fileMetadata.IsRegularFile()) {
			return
		}
		assert.Equal(t, core.Path("/a.txt"), fileMetadata.AbsolutePath)
		assert.Equal(t, core.ByteCount(1), fileMetadata.Size)
		assert.Empty(t, fileMetadata.ChildNames)

		addressableContent, err := snapshot.Content("/a.txt")
		if !assert.NoError(t, err) {
			return
		}
		content, err := io.ReadAll(addressableContent.Reader())
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, "a", string(content))
	})

	t.Run("subdir with empty subdir in root dir", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		desc := core.NewObjectFromMapNoInit(core.ValMap{
			FS_SNAPSHOT_SYMB_DESC_FILES_PROPNAME: core.NewDictionary(core.ValMap{
				core.GetJSONRepresentation(core.Path("./dir/"), nil, nil): core.NewDictionary(core.ValMap{
					core.GetJSONRepresentation(core.Path("./subdir/"), nil, nil): core.NewWrappedValueList(),
				}),
			}),
		})

		snapshot := NewFilesystemSnapshot(ctx, desc).Underlying()

		if !assert.Len(t, snapshot.RootDirEntries(), 1) {
			return
		}

		metadata, err := snapshot.Metadata("/dir")
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, metadata.IsDir()) {
			return
		}
		assert.Equal(t, core.Path("/dir/"), metadata.AbsolutePath)
		assert.Zero(t, metadata.Size)
		assert.Equal(t, []string{"subdir"}, metadata.ChildNames)

		subdirMetadata, err := snapshot.Metadata("/dir/subdir")
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, subdirMetadata.IsDir()) {
			return
		}
		assert.Equal(t, core.Path("/dir/subdir/"), subdirMetadata.AbsolutePath)
		assert.Zero(t, subdirMetadata.Size)
		assert.Empty(t, subdirMetadata.ChildNames)
	})

	t.Run("subdir with subdir containing empty file in root dir", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		desc := core.NewObjectFromMapNoInit(core.ValMap{
			FS_SNAPSHOT_SYMB_DESC_FILES_PROPNAME: core.NewDictionary(core.ValMap{
				core.GetJSONRepresentation(core.Path("./dir/"), nil, nil): core.NewDictionary(core.ValMap{
					core.GetJSONRepresentation(core.Path("./subdir/"), nil, nil): core.NewWrappedValueList(core.Path("./a.txt")),
				}),
			}),
		})

		snapshot := NewFilesystemSnapshot(ctx, desc).Underlying()

		if !assert.Len(t, snapshot.RootDirEntries(), 1) {
			return
		}

		metadata, err := snapshot.Metadata("/dir")
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, metadata.IsDir()) {
			return
		}
		assert.Equal(t, core.Path("/dir/"), metadata.AbsolutePath)
		assert.Zero(t, metadata.Size)
		assert.Equal(t, []string{"subdir"}, metadata.ChildNames)

		subdirMetadata, err := snapshot.Metadata("/dir/subdir")
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, subdirMetadata.IsDir()) {
			return
		}
		assert.Equal(t, core.Path("/dir/subdir/"), subdirMetadata.AbsolutePath)
		assert.Zero(t, subdirMetadata.Size)
		assert.Equal(t, []string{"a.txt"}, subdirMetadata.ChildNames)

		fileMetadata, err := snapshot.Metadata("/dir/subdir/a.txt")
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, fileMetadata.IsRegularFile()) {
			return
		}
		assert.Equal(t, core.Path("/dir/subdir/a.txt"), fileMetadata.AbsolutePath)
		assert.Zero(t, fileMetadata.Size)
		assert.Empty(t, fileMetadata.ChildNames)

		addressableContent, err := snapshot.Content("/dir/subdir/a.txt")
		if !assert.NoError(t, err) {
			return
		}
		content, err := io.ReadAll(addressableContent.Reader())
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, string(content))
	})
}
