package fs_ns

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

const SLEEP_DURATION = 100 * time.Millisecond

func TestEvents(t *testing.T) {

	t.Run("OS filesystem", func(t *testing.T) {
		testEvents(t, func(t *testing.T) (fls afs.Filesystem, tempDir string) {
			return GetOsFilesystem(), t.TempDir() + "/"
		})
	})

	t.Run("Memory filesystem", func(t *testing.T) {
		testEvents(t, func(t *testing.T) (fls afs.Filesystem, tempDir string) {
			return NewMemFilesystem(1_000_000), "/"
		})
	})

	t.Run("Meta filesystem", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		testEvents(t, func(t *testing.T) (fls afs.Filesystem, tempDir string) {
			underlyingFS := NewMemFilesystem(1_000_000)
			metaFS, err := OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemParams{})

			if !assert.NoError(t, err) {
				t.SkipNow()
				return
			}
			return metaFS, "/"
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
			Filesystem: fls,
		})
		defer ctx.CancelGracefully()

		// create the event source & add a callback function
		evs, err := NewEventSource(ctx, dirPatt)
		if !assert.NoError(t, err) {
			return
		}
		defer evs.Close()

		var callCount atomic.Int32
		filepth := filepath.Join(string(tempDir), "file_in_dir.txt")
		subdirFilepth := filepath.Join(string(subdir), "file_in_subdir.txt")

		err = evs.OnEvent(func(event *core.Event) {
			count := callCount.Add(1)

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

		assert.EqualValues(t, 3, callCount.Load())

		// create a file in the subdirectory
		f, err = fls.Create(subdirFilepth)
		if assert.NoError(t, err) {
			f.Close()
		}

		time.Sleep(SLEEP_DURATION)
		assert.EqualValues(t, 4, callCount.Load())
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
			Filesystem: fls,
		})
		defer ctx.CancelGracefully()

		// create the event source & add a callback function
		evs, err := NewEventSource(ctx, filepth)
		if !assert.NoError(t, err) {
			return
		}
		defer evs.Close()

		var callCount atomic.Int32

		err = evs.OnEvent(func(event *core.Event) {
			count := callCount.Add(1)

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

		assert.EqualValues(t, 1, callCount.Load())
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
			Filesystem: fls,
		})
		defer ctx.CancelGracefully()

		// create the event source & add a callback function
		evs, err := NewEventSource(ctx, dirPatt)
		if !assert.NoError(t, err) {
			return
		}
		defer evs.Close()

		var callCount atomic.Int32
		filepth := filepath.Join(string(tempDir), "file_in_dir.txt")
		subdirFilepth := filepath.Join(string(subdir), "file_in_subdir.txt")

		err = evs.OnEvent(func(event *core.Event) {
			count := callCount.Add(1)

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

		assert.EqualValues(t, 3, callCount.Load())

		// create a file in the subdirectory
		f, err = fls.Create(subdirFilepth)
		if assert.NoError(t, err) {
			f.Close()
		}

		time.Sleep(SLEEP_DURATION)
		assert.EqualValues(t, int32(4), callCount.Load())
	})

	t.Run("dir path should end in '/'", func(t *testing.T) {
		// we create a temporary dir
		fls, tempDir := setup(t)
		subdir := filepath.Join(tempDir, "subdir")
		if !assert.NoError(t, fls.MkdirAll(subdir, DEFAULT_DIR_FMODE)) {
			return
		}
		subdirPath := core.Path(subdir)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: subdirPath},
			},
			Filesystem: fls,
		})
		defer ctx.CancelGracefully()

		// we create an event source
		evs, err := NewEventSource(ctx, subdirPath)
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
			Filesystem: fls,
		})
		defer ctx.CancelGracefully()

		// we create an event source
		evs, err := NewEventSource(ctx, core.Path(filepth+"/"))
		assert.ErrorIs(t, err, core.ErrFilePathShouldNotEndInSlash)
		assert.Nil(t, evs)
	})

	t.Run("if virtual FS, no event or a read event should be emitted when opening an file immediately after having created it", func(t *testing.T) {
		// create a temporary directory & a file in it
		fls, tempDir := setup(t)

		if utils.Implements[*OsFilesystem](fls) {
			t.SkipNow()
		}

		filepth := core.Path(filepath.Join(tempDir, "file.txt"))

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
			},
			Filesystem: fls,
		})
		defer ctx.CancelGracefully()

		// create the event source & add a callback function
		evs, err := NewEventSource(ctx, core.PathPattern("/..."))
		if !assert.NoError(t, err) {
			return
		}
		defer evs.Close()

		var callCount atomic.Int32

		err = evs.OnEvent(func(event *core.Event) {
			count := callCount.Add(1)
			switch count {
			case 1:
				assert.Equal(t, core.NewRecordFromMap(core.ValMap{
					"path":      filepth,
					"write_op":  core.False,
					"create_op": core.True,
					"remove_op": core.False,
					"chmod_op":  core.False,
					"rename_op": core.False,
				}), event.Value())
			case 2:
				assert.Equal(t, core.NewRecordFromMap(core.ValMap{
					"path":      filepth,
					"write_op":  core.False,
					"create_op": core.False,
					"remove_op": core.False,
					"chmod_op":  core.False,
					"rename_op": core.False,
				}), event.Value())
			}

		})
		assert.NoError(t, err)

		f, err := fls.Create(string(filepth))
		if assert.NoError(t, err) {
			if capable, ok := f.(afs.SyncCapable); ok {
				capable.Sync()
			}
		}
		f.Close()

		f, err = fls.OpenFile(string(filepth), os.O_RDONLY, 0)
		if !assert.NoError(t, err) {
			return
		}
		f.Close()

		time.Sleep(SLEEP_DURATION)

		assert.LessOrEqual(t, int(callCount.Load()), 2)
	})

	t.Run("if virtual FS, a single event should be emitted for a few same-file writes that are very close in time", func(t *testing.T) {
		// create a temporary directory & a file in it
		fls, tempDir := setup(t)

		if utils.Implements[*OsFilesystem](fls) {
			t.SkipNow()
		}

		filepth := core.Path(filepath.Join(tempDir, "file.txt"))

		f, err := fls.Create(string(filepth))
		if assert.NoError(t, err) {
			if capable, ok := f.(afs.SyncCapable); ok {
				capable.Sync()
			}
		}

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: filepth},
			},
			Filesystem: fls,
		})
		defer ctx.CancelGracefully()

		// create the event source & add a callback function
		evs, err := NewEventSource(ctx, filepth)
		if !assert.NoError(t, err) {
			return
		}
		defer evs.Close()

		var callCount atomic.Int32

		err = evs.OnEvent(func(event *core.Event) {
			count := callCount.Add(1)

			switch count {
			case 1:
				//a single event with write_op true is expected.
				assert.Equal(t, core.NewRecordFromMap(core.ValMap{
					"path":      filepth,
					"write_op":  core.True,
					"create_op": core.False,
					"remove_op": core.False,
					"chmod_op":  core.False,
					"rename_op": core.False,
				}), event.Value())
			}
		})
		assert.NoError(t, err)

		for i := 0; i < 10; i++ {
			_, err := f.Write([]byte{'a'})
			if !assert.NoError(t, err) {
				return
			}
		}

		if capable, ok := f.(afs.SyncCapable); ok {
			capable.Sync()
		}

		if !assert.NoError(t, f.Close()) {
			return
		}

		time.Sleep(SLEEP_DURATION)

		assert.EqualValues(t, 1, callCount.Load())
	})

	t.Run("if virtual FS, a single event should be emitted for a few same-file writes that are very close in time (several files)", func(t *testing.T) {
		// create a temporary directory & a file in it
		fls, tempDir := setup(t)

		if utils.Implements[*OsFilesystem](fls) {
			t.SkipNow()
		}

		dirPatt := core.PathPattern(tempDir + "...")
		filepth1 := core.Path(filepath.Join(tempDir, "file1.txt"))
		filepth2 := core.Path(filepath.Join(tempDir, "file2.txt"))

		f1, err := fls.Create(string(filepth1))
		if assert.NoError(t, err) {
			if capable, ok := f1.(afs.SyncCapable); ok {
				capable.Sync()
			}
		}
		f2, err := fls.Create(string(filepth2))
		if assert.NoError(t, err) {
			if capable, ok := f2.(afs.SyncCapable); ok {
				capable.Sync()
			}
		}

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: dirPatt},
			},
			Filesystem: fls,
		})
		defer ctx.CancelGracefully()

		// create the event source & add a callback function
		evs, err := NewEventSource(ctx, dirPatt)
		if !assert.NoError(t, err) {
			return
		}
		defer evs.Close()

		var callCount atomic.Int32

		err = evs.OnEvent(func(event *core.Event) {
			count := callCount.Add(1)

			switch count {
			case 1:
				//a single event with write_op true is expected for file1.txt.
				assert.Equal(t, core.NewRecordFromMap(core.ValMap{
					"path":      filepth1,
					"write_op":  core.True,
					"create_op": core.False,
					"remove_op": core.False,
					"chmod_op":  core.False,
					"rename_op": core.False,
				}), event.Value())
			case 2:
				//a single event with write_op true is expected for file2.txt.
				assert.Equal(t, core.NewRecordFromMap(core.ValMap{
					"path":      filepth2,
					"write_op":  core.True,
					"create_op": core.False,
					"remove_op": core.False,
					"chmod_op":  core.False,
					"rename_op": core.False,
				}), event.Value())
			}
		})
		assert.NoError(t, err)

		//write to file 1
		for i := 0; i < 10; i++ {
			_, err := f1.Write([]byte{'a'})
			if !assert.NoError(t, err) {
				return
			}
		}

		//write to file 2
		for i := 0; i < 10; i++ {
			_, err := f2.Write([]byte{'a'})
			if !assert.NoError(t, err) {
				return
			}
		}

		//sync and close both files
		if capable, ok := f1.(afs.SyncCapable); ok {
			capable.Sync()
		}

		if !assert.NoError(t, f1.Close()) {
			return
		}

		if capable, ok := f2.(afs.SyncCapable); ok {
			capable.Sync()
		}

		if !assert.NoError(t, f2.Close()) {
			return
		}

		time.Sleep(SLEEP_DURATION)

		assert.EqualValues(t, 2, callCount.Load())
	})

	t.Run("if virtual FS, no events should be emitted if the FS is closed immediately after a few writes", func(t *testing.T) {
		// create a temporary directory & a file in it
		fls, tempDir := setup(t)

		if utils.Implements[*OsFilesystem](fls) {
			t.SkipNow()
		}

		if !utils.Implements[ClosableFilesystem](fls) {
			assert.FailNow(t, "filesystem should be closable")
		}

		closable := fls.(ClosableFilesystem)

		filepth := core.Path(filepath.Join(tempDir, "file.txt"))

		f, err := fls.Create(string(filepth))
		if assert.NoError(t, err) {
			if capable, ok := f.(afs.SyncCapable); ok {
				capable.Sync()
			}
		}

		ctx := core.NewContexWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: filepth},
			},
			Filesystem: fls,
		}, nil)
		defer ctx.CancelGracefully()

		// create the event source & add a callback function
		evs, err := NewEventSource(ctx, filepth)
		if !assert.NoError(t, err) {
			return
		}
		defer evs.Close()

		var callCount atomic.Int32

		err = evs.OnEvent(func(event *core.Event) {
			callCount.Add(1)
		})
		assert.NoError(t, err)

		for i := 0; i < 10; i++ {
			_, err := f.Write([]byte{'a'})
			if !assert.NoError(t, err) {
				return
			}
		}

		if capable, ok := f.(afs.SyncCapable); ok {
			capable.Sync()
		}

		if !assert.NoError(t, f.Close()) {
			return
		}
		if !assert.NoError(t, closable.Close(ctx)) {
			return
		}
		time.Sleep(SLEEP_DURATION)

		assert.EqualValues(t, 0, callCount.Load())
	})

	t.Run("several spaced out writes in the same file", func(t *testing.T) {
		// create a temporary directory & a file in it
		fls, tempDir := setup(t)

		if utils.Implements[*OsFilesystem](fls) {
			t.SkipNow()
		}

		const INTERVAL = 50 * time.Millisecond

		if !assert.Greater(t, INTERVAL, WATCHER_MANAGEMENT_TICK_INTERVAL) {
			return
		}

		filepth := core.Path(filepath.Join(tempDir, "file.txt"))

		f, err := fls.Create(string(filepth))
		if assert.NoError(t, err) {
			if capable, ok := f.(afs.SyncCapable); ok {
				capable.Sync()
			}
		}

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: filepth},
			},
			Filesystem: fls,
		})
		defer ctx.CancelGracefully()

		// create the event source & add a callback function
		evs, err := NewEventSource(ctx, filepth)
		if !assert.NoError(t, err) {
			return
		}
		defer evs.Close()

		var callCount atomic.Int32

		writeCount := 10

		err = evs.OnEvent(func(event *core.Event) {
			callCount.Add(1)
		})
		assert.NoError(t, err)

		for i := 0; i < writeCount; i++ {
			_, err := f.Write([]byte{'a'})
			if !assert.NoError(t, err) {
				return
			}
			time.Sleep(INTERVAL)
		}

		if capable, ok := f.(afs.SyncCapable); ok {
			capable.Sync()
		}

		time.Sleep(SLEEP_DURATION)

		assert.EqualValues(t, writeCount, callCount.Load())
	})

	t.Run("high number of file creation in parallel", func(t *testing.T) {
		// create a temporary directory & a file in it
		fls, tempDir := setup(t)

		if utils.Implements[*OsFilesystem](fls) {
			t.SkipNow()
		}

		dirPatt := core.PathPattern(tempDir + "...")

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: dirPatt},
			},
			Filesystem: fls,
		})
		defer ctx.CancelGracefully()

		// create the event source & add a callback function
		evs, err := NewEventSource(ctx, dirPatt)
		if !assert.NoError(t, err) {
			return
		}
		defer evs.Close()

		var callCount atomic.Int32
		fileCount := 1000

		err = evs.OnEvent(func(event *core.Event) {
			callCount.Add(1)
		})
		assert.NoError(t, err)

		wg := new(sync.WaitGroup)
		wg.Add(fileCount)

		for i := 0; i < fileCount; i++ {
			go func(i int) {
				defer wg.Done()
				util.WriteFile(fls, "/file"+strconv.Itoa(i)+".txt", []byte("a"), DEFAULT_FILE_FMODE)
			}(i)
		}

		wg.Wait()
		time.Sleep(SLEEP_DURATION)

		assert.EqualValues(t, 2*fileCount, callCount.Load())
	})

	t.Run("even if no watcher is created, all old events should be removed after a recent event", func(t *testing.T) {
		// create a temporary directory & a file in it
		fls, tempDir := setup(t)

		if !utils.Implements[WatchableVirtualFilesystem](fls) {
			t.SkipNow()
		}

		watchableFilesystem := fls.(WatchableVirtualFilesystem)

		dirPatt := core.PathPattern(tempDir + "...")

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: dirPatt},
			},
			Filesystem: fls,
		})
		defer ctx.CancelGracefully()

		fileCount := 10

		//create fileCount-1 files
		wg := new(sync.WaitGroup)
		wg.Add(fileCount - 1)

		for i := 0; i < fileCount-1; i++ {
			go func(i int) {
				defer wg.Done()
				util.WriteFile(fls, "/file"+strconv.Itoa(i)+".txt", []byte("a"), DEFAULT_FILE_FMODE)
			}(i)
		}

		wg.Wait()

		assert.EqualValues(t, 2*(fileCount-1), watchableFilesystem.Events().Size())
		time.Sleep(OLD_EVENT_MIN_AGE)

		//create a last file
		util.WriteFile(fls, "/file"+strconv.Itoa(fileCount-1)+".txt", []byte("a"), DEFAULT_FILE_FMODE)

		//all old events should have been removed.
		assert.EqualValues(t, 2, watchableFilesystem.Events().Size(), fileCount/2)
	})

}
