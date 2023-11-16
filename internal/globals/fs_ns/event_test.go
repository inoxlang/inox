package fs_ns

import (
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/stretchr/testify/assert"
)

const SLEEP_DURATION = 100 * time.Millisecond

func TestEvents(t *testing.T) {

	t.Run("OS filesystem", func(t *testing.T) {
		testEvents(t, func(t *testing.T) (fls afs.Filesystem, tempDir string) {
			return GetOsFilesystem(), t.TempDir() + "/"
		})
	})
}

func testEvents(t *testing.T, setup func(t *testing.T) (fls afs.Filesystem, tempDir string)) {
	t.Run("prefix pattern", func(t *testing.T) {
		// create a temporary directory & a subdirectory in it
		fls, tempDir := setup(t)
		subdir := filepath.Join(tempDir, "subdir") + "/"
		assert.NoError(t, fls.MkdirAll(subdir, 0o700))

		dirPatt := core.PathPattern(tempDir + "...")

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: dirPatt},
			},
			Filesystem: GetOsFilesystem(),
		})
		defer ctx.CancelGracefully()

		// create the event source & add a callback function
		evs, err := NewEventSource(ctx, dirPatt)
		assert.NoError(t, err)
		defer evs.Close()

		callCount := int32(0)
		filepth := filepath.Join(string(tempDir), "file_in_dir.txt")
		subdirFilepth := filepath.Join(string(subdir), "file_in_subdir.txt")

		err = evs.OnEvent(func(event *core.Event) {
			count := atomic.AddInt32(&callCount, 1)

			switch count {
			case 1:
				assert.Equal(t, core.NewRecordFromMap(core.ValMap{
					"path":      core.Path(filepth),
					"write_op":  core.False,
					"create_op": core.True,
					"remove_op": core.False,
					"chmod_op":  core.False,
					"rename_op": core.False,
				}), event.Value())
			case 2:
				assert.Equal(t, core.NewRecordFromMap(core.ValMap{
					"path":      core.Path(filepth),
					"write_op":  core.True,
					"create_op": core.False,
					"remove_op": core.False,
					"chmod_op":  core.False,
					"rename_op": core.False,
				}), event.Value())
			case 3:
				assert.Equal(t, core.NewRecordFromMap(core.ValMap{
					"path":      core.Path(filepth),
					"write_op":  core.False,
					"create_op": core.False,
					"remove_op": core.True,
					"chmod_op":  core.False,
					"rename_op": core.False,
				}), event.Value())
			case 4:
				assert.Equal(t, core.NewRecordFromMap(core.ValMap{
					"path":      core.Path(subdirFilepth),
					"write_op":  core.False,
					"create_op": core.True,
					"remove_op": core.False,
					"chmod_op":  core.False,
					"rename_op": core.False,
				}), event.Value())
			}
		})
		assert.NoError(t, err)

		// create a file and write to it
		f, err := fls.Create(filepth)
		if assert.NoError(t, err) {
			f.Write([]byte("a"))
			if capable, ok := f.(afs.SyncCapable); ok {
				capable.Sync()
			}
			f.Close()
		}
		time.Sleep(SLEEP_DURATION)

		// delete the created file
		assert.NoError(t, fls.Remove(filepth))
		time.Sleep(100 * time.Millisecond)

		assert.EqualValues(t, 3, atomic.LoadInt32(&callCount))

		// create a file in the subdirectory
		f, err = fls.Create(subdirFilepth)
		if assert.NoError(t, err) {
			f.Close()
		}

		time.Sleep(SLEEP_DURATION)
		assert.EqualValues(t, 4, atomic.LoadInt32(&callCount))
	})

	t.Run("file path", func(t *testing.T) {
		// create a temporary directory & a file in it
		fls, tempDir := setup(t)
		filepth := core.Path(filepath.Join(tempDir, "file.txt"))

		f, err := fls.Create(string(filepth))
		if assert.NoError(t, err) {
			if capable, ok := f.(afs.SyncCapable); ok {
				capable.Sync()
			}
			f.Close()
		}

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: filepth},
			},
			Filesystem: GetOsFilesystem(),
		})
		defer ctx.CancelGracefully()

		// create the event source & add a callback function
		evs, err := NewEventSource(ctx, filepth)
		assert.NoError(t, err)
		defer evs.Close()

		callCount := int32(0)

		err = evs.OnEvent(func(event *core.Event) {
			count := atomic.AddInt32(&callCount, 1)

			switch count {
			case 1:
				assert.Equal(t, core.NewRecordFromMap(core.ValMap{
					"path":      filepth,
					"write_op":  core.False,
					"create_op": core.False,
					"remove_op": core.True,
					"chmod_op":  core.False,
					"rename_op": core.False,
				}), event.Value())
			}
		})
		assert.NoError(t, err)

		// create a file in the directory
		f, err = fls.Create(filepath.Join(tempDir, "other_file.txt"))
		if assert.NoError(t, err) {
			f.Close()
		}

		// delete the watched file
		assert.NoError(t, fls.Remove(string(filepth)))
		time.Sleep(100 * time.Millisecond)

		assert.EqualValues(t, 1, atomic.LoadInt32(&callCount))
	})

	t.Run("dir path", func(t *testing.T) {
		// create a temporary directory & a subdirectory in it
		fls, tempDir := setup(t)
		subdir := filepath.Join(tempDir, "subdir") + "/"
		assert.NoError(t, fls.MkdirAll(subdir, 0o700))

		dirPatt := core.PathPattern(tempDir + "...")

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: dirPatt},
			},
			Filesystem: GetOsFilesystem(),
		})
		defer ctx.CancelGracefully()

		// create the event source & add a callback function
		evs, err := NewEventSource(ctx, dirPatt)
		assert.NoError(t, err)
		defer evs.Close()

		callCount := int32(0)
		filepth := filepath.Join(string(tempDir), "file_in_dir.txt")
		subdirFilepth := filepath.Join(string(subdir), "file_in_subdir.txt")

		err = evs.OnEvent(func(event *core.Event) {
			count := atomic.AddInt32(&callCount, 1)

			switch count {
			case 1:
				assert.Equal(t, core.NewRecordFromMap(core.ValMap{
					"path":      core.Path(filepth),
					"write_op":  core.False,
					"create_op": core.True,
					"remove_op": core.False,
					"chmod_op":  core.False,
					"rename_op": core.False,
				}), event.Value())
			case 2:
				assert.Equal(t, core.NewRecordFromMap(core.ValMap{
					"path":      core.Path(filepth),
					"write_op":  core.True,
					"create_op": core.False,
					"remove_op": core.False,
					"chmod_op":  core.False,
					"rename_op": core.False,
				}), event.Value())
			case 3:
				assert.Equal(t, core.NewRecordFromMap(core.ValMap{
					"path":      core.Path(filepth),
					"write_op":  core.False,
					"create_op": core.False,
					"remove_op": core.True,
					"chmod_op":  core.False,
					"rename_op": core.False,
				}), event.Value())
			}
		})
		assert.NoError(t, err)

		// create a file and write to it
		f, err := fls.Create(filepth)
		if assert.NoError(t, err) {
			f.Write([]byte("a"))
			if capable, ok := f.(afs.SyncCapable); ok {
				capable.Sync()
			}
			f.Close()
		}
		time.Sleep(SLEEP_DURATION)

		// delete the created file
		assert.NoError(t, fls.Remove(filepth))
		time.Sleep(100 * time.Millisecond)

		assert.EqualValues(t, 3, atomic.LoadInt32(&callCount))

		// create a file in the subdirectory
		f, err = fls.Create(subdirFilepth)
		if assert.NoError(t, err) {
			f.Close()
		}

		time.Sleep(SLEEP_DURATION)
		assert.EqualValues(t, 4, atomic.LoadInt32(&callCount))
	})

	t.Run("dir path should end in '/'", func(t *testing.T) {
		// we create a temporary dir
		dir := core.Path(t.TempDir())

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: dir},
			},
			Filesystem: GetOsFilesystem(),
		})
		defer ctx.CancelGracefully()

		// we create an event source
		evs, err := NewEventSource(ctx, dir)
		assert.ErrorIs(t, err, core.ErrDirPathShouldEndInSlash)
		assert.Nil(t, evs)
	})

	t.Run("file path sould not end in '/'", func(t *testing.T) {
		// we create a temporary dir & a file in it
		fls, tempDir := setup(t)
		dirPatt := core.PathPattern(tempDir + "...")
		filepth := filepath.Join(string(tempDir), "file_in_dir.txt")

		assert.NoError(t, util.WriteFile(fls, filepth, nil, DEFAULT_FILE_FMODE))
		time.Sleep(SLEEP_DURATION)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: dirPatt},
			},
			Filesystem: GetOsFilesystem(),
		})
		defer ctx.CancelGracefully()

		// we create an event source
		evs, err := NewEventSource(ctx, core.Path(filepth+"/"))
		assert.ErrorIs(t, err, core.ErrFilePathShouldNotEndInSlash)
		assert.Nil(t, evs)
	})
}
