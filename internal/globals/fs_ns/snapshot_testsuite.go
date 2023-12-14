package fs_ns

import (
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func testSnapshoting(t *testing.T, createFS func(*testing.T) (*core.Context, core.SnapshotableFilesystem)) {

	t.Helper()

	snapshotConfig := core.FilesystemSnapshotConfig{
		GetContent: func(ChecksumSHA256 [32]byte) core.AddressableContent {
			return nil
		},
		InclusionFilters: []core.PathPattern{"/..."},
	}

	t.Run("empty", func(t *testing.T) {
		ctx, fls := createFS(t)
		defer ctx.CancelGracefully()

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}
		assert.Empty(t, snapshot.RootDirEntries())
	})

	t.Run("file in rootdir", func(t *testing.T) {
		ctx, fls := createFS(t)
		defer ctx.CancelGracefully()

		f, err := fls.Create("/a.txt")
		assert.NoError(t, err)
		f.Write([]byte("a"))

		info := utils.Must(fls.Stat("/a.txt"))
		creationTime, modTime := utils.Must2(GetCreationAndModifTime(info))
		mode := info.Mode()
		f.Close()

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, snapshot.RootDirEntries(), 1) {
			return
		}

		metadata, err := snapshot.Metadata("/a.txt")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, core.EntrySnapshotMetadata{
			AbsolutePath:     "/a.txt",
			Size:             1,
			CreationTime:     core.DateTime(creationTime),
			ModificationTime: core.DateTime(modTime),
			Mode:             core.FileMode(mode),
			ChecksumSHA256:   sha256.Sum256([]byte("a")),
		}, metadata)

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
		ctx, fls := createFS(t)
		defer ctx.CancelGracefully()

		fls.MkdirAll("/dir/", DEFAULT_DIR_FMODE)
		info := utils.Must(fls.Stat("/dir/"))
		creationTime, modTime := utils.Must2(GetCreationAndModifTime(info))

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, snapshot.RootDirEntries(), 1) {
			return
		}

		metadata, err := snapshot.Metadata("/dir")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, core.EntrySnapshotMetadata{
			AbsolutePath:     "/dir/",
			CreationTime:     core.DateTime(creationTime),
			ModificationTime: core.DateTime(modTime),
			Mode:             core.FileMode(info.Mode()),
		}, metadata)
	})

	t.Run("subdir with file in root dir", func(t *testing.T) {
		ctx, fls := createFS(t)
		defer ctx.CancelGracefully()

		fls.MkdirAll("/dir/", DEFAULT_DIR_FMODE)
		util.WriteFile(fls, "/dir/file.txt", []byte("a"), DEFAULT_FILE_FMODE)

		fileInfo := utils.Must(fls.Stat("/dir/file.txt"))
		fileCreationTime, fileModifTime := utils.Must2(GetCreationAndModifTime(fileInfo))

		dirInfo := utils.Must(fls.Stat("/dir/"))
		dirCreationTime, dirModTime := utils.Must2(GetCreationAndModifTime(dirInfo))

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, snapshot.RootDirEntries(), 1) {
			return
		}

		dirMetadata, err := snapshot.Metadata("/dir")
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, dirMetadata.IsDir()) {
			return
		}

		assert.Equal(t, core.EntrySnapshotMetadata{
			AbsolutePath:     "/dir/",
			CreationTime:     core.DateTime(dirCreationTime),
			ModificationTime: core.DateTime(dirModTime),
			Mode:             core.FileMode(dirInfo.Mode()),
			ChildNames:       []string{"file.txt"},
		}, dirMetadata)

		fileMetadata, err := snapshot.Metadata("/dir/file.txt")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, core.EntrySnapshotMetadata{
			AbsolutePath:     "/dir/file.txt",
			Size:             1,
			CreationTime:     core.DateTime(fileCreationTime),
			ModificationTime: core.DateTime(fileModifTime),
			Mode:             core.FileMode(fileInfo.Mode()),
			ChecksumSHA256:   sha256.Sum256([]byte("a")),
		}, fileMetadata)

		addressableContent, err := snapshot.Content("/dir/file.txt")
		if !assert.NoError(t, err) {
			return
		}
		content, err := io.ReadAll(addressableContent.Reader())
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, "a", string(content))
	})

	t.Run("empty subdir & file in root dir", func(t *testing.T) {
		ctx, fls := createFS(t)
		defer ctx.CancelGracefully()

		fls.MkdirAll("/dir/", DEFAULT_DIR_FMODE)
		util.WriteFile(fls, "/a.txt", []byte("a"), DEFAULT_FILE_FMODE)

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

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

	t.Run("empty subdir & file in root dir: file name > dir name", func(t *testing.T) {
		ctx, fls := createFS(t)
		defer ctx.CancelGracefully()

		fls.MkdirAll("/dir/", DEFAULT_DIR_FMODE)
		util.WriteFile(fls, "/e.txt", []byte("e"), DEFAULT_FILE_FMODE)

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

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

		fileMetadata, err := snapshot.Metadata("/e.txt")
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, fileMetadata.IsRegularFile()) {
			return
		}
		assert.Equal(t, core.Path("/e.txt"), fileMetadata.AbsolutePath)
		assert.Equal(t, core.ByteCount(1), fileMetadata.Size)
		assert.Empty(t, fileMetadata.ChildNames)

		addressableContent, err := snapshot.Content("/e.txt")
		if !assert.NoError(t, err) {
			return
		}
		content, err := io.ReadAll(addressableContent.Reader())
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, "e", string(content))
	})

	t.Run("subdir with empty subdir in root dir", func(t *testing.T) {
		ctx, fls := createFS(t)
		defer ctx.CancelGracefully()

		fls.MkdirAll("/dir/", DEFAULT_DIR_FMODE)
		fls.MkdirAll("/dir/subdir/", DEFAULT_DIR_FMODE)

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

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
		ctx, fls := createFS(t)
		defer ctx.CancelGracefully()

		fls.MkdirAll("/dir/", DEFAULT_DIR_FMODE)
		fls.MkdirAll("/dir/subdir/", DEFAULT_DIR_FMODE)
		util.WriteFile(fls, "/dir/subdir/a.txt", nil, DEFAULT_FILE_FMODE)

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

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

	t.Run("file open in readonly mode", func(t *testing.T) {
		ctx, fls := createFS(t)
		defer ctx.CancelGracefully()

		err := util.WriteFile(fls, "/a.txt", []byte("a"), 0600)
		if !assert.NoError(t, err) {
			return
		}

		f, err := fls.OpenFile("/a.txt", os.O_RDONLY, 0600)
		if !assert.NoError(t, err) {
			return
		}

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

		err = f.Close()
		if !assert.NoError(t, err) {
			return
		}

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

	t.Run("file open in read-write mode", func(t *testing.T) {
		ctx, fls := createFS(t)
		defer ctx.CancelGracefully()

		f, err := fls.OpenFile("/a.txt", os.O_RDWR|os.O_CREATE, 0600)
		if !assert.NoError(t, err) {
			return
		}
		_, err = f.Write([]byte("a"))
		if !assert.NoError(t, err) {
			return
		}

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

		//write after the snapshot has been taken
		_, err = f.Write([]byte("b"))
		if !assert.NoError(t, err) {
			return
		}

		err = f.Close()
		if !assert.NoError(t, err) {
			return
		}

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

	t.Run("file open in read-write mode: parallel writing should not be slow for small files", func(t *testing.T) {
		ctx, fls := createFS(t)
		defer ctx.CancelGracefully()

		f, err := fls.OpenFile("/a.txt", os.O_RDWR|os.O_CREATE, 0600)
		if !assert.NoError(t, err) {
			return
		}
		_, err = f.Write([]byte("a"))
		if !assert.NoError(t, err) {
			return
		}

		var firstParallelWriteError error
		var writeCount = 0
		var writeStart, writeEnd time.Time

		wg := new(sync.WaitGroup)
		wg.Add(1)
		var snapshotDone atomic.Bool

		go func() {
			defer wg.Done()
			writeStart = time.Now()
			defer func() {
				writeEnd = time.Now()
			}()

			remainingWriteCountAfterSnapshotDone := 100

			for {
				if snapshotDone.Load() {
					remainingWriteCountAfterSnapshotDone--
					if remainingWriteCountAfterSnapshotDone == 0 {
						break
					}
				}

				start := time.Now()
				_, err := f.Write([]byte("a"))

				if err != nil {
					firstParallelWriteError = err
					break
				}
				writeCount++

				if time.Since(start) > 10*time.Millisecond {
					firstParallelWriteError = errors.New("write is too slow")
					break
				}
			}
		}()

		time.Sleep(time.Millisecond)
		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

		snapshotDone.Store(true)
		wg.Wait()

		if !assert.NoError(t, firstParallelWriteError) {
			return
		}

		assert.WithinDuration(t, writeEnd, writeStart, 10*time.Millisecond)

		err = f.Close()
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, snapshot.RootDirEntries(), 1) {
			return
		}

		metadata, err := snapshot.Metadata("/a.txt")
		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, metadata.IsRegularFile())
		assert.Equal(t, core.Path("/a.txt"), metadata.AbsolutePath)
		assert.Less(t, metadata.Size, core.ByteCount(writeCount+1))
		assert.Greater(t, metadata.Size, core.ByteCount(writeCount/2-(writeCount/10 /*delta*/)))
		assert.Empty(t, metadata.ChildNames)

		addressableContent, err := snapshot.Content("/a.txt")
		if !assert.NoError(t, err) {
			return
		}
		content, err := io.ReadAll(addressableContent.Reader())
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, strings.Repeat("a", int(metadata.Size)), string(content))
	})

	t.Run("file open in append mode", func(t *testing.T) {
		ctx, fls := createFS(t)
		defer ctx.CancelGracefully()

		f, err := fls.OpenFile("/a.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if !assert.NoError(t, err) {
			return
		}
		_, err = f.Write([]byte("a"))
		if !assert.NoError(t, err) {
			return
		}

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

		//write after the snapshot has been taken
		_, err = f.Write([]byte("b"))
		if !assert.NoError(t, err) {
			return
		}

		err = f.Close()
		if !assert.NoError(t, err) {
			return
		}

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

	t.Run("no included files", func(t *testing.T) {
		ctx, fls := createFS(t)
		defer ctx.CancelGracefully()

		util.WriteFile(fls, "/a.txt", []byte("a"), DEFAULT_FILE_FMODE)

		snapshot, err := fls.TakeFilesystemSnapshot(core.FilesystemSnapshotConfig{
			GetContent: snapshotConfig.GetContent,
		})
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, snapshot.RootDirEntries(), 0) {
			return
		}

		metadata, err := snapshot.Metadata("/")
		if !assert.NoError(t, err) {
			return
		}
		assert.Empty(t, metadata.ChildNames)
	})

	t.Run("included file in folder", func(t *testing.T) {
		ctx, fls := createFS(t)
		defer ctx.CancelGracefully()

		utils.PanicIfErrAmong(
			fls.MkdirAll("/dir", DEFAULT_DIR_FMODE),
			util.WriteFile(fls, "/dir/a.ix", []byte("a"), DEFAULT_FILE_FMODE),
		)

		snapshot, err := fls.TakeFilesystemSnapshot(core.FilesystemSnapshotConfig{
			GetContent:       snapshotConfig.GetContent,
			InclusionFilters: []core.PathPattern{"/**/*.ix"},
		})
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, snapshot.RootDirEntries(), 1) {
			return
		}

		metadata, err := snapshot.Metadata("/")
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, []string{"dir"}, metadata.ChildNames)

		metadata, err = snapshot.Metadata("/dir")
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, []string{"a.ix"}, metadata.ChildNames)
	})

	t.Run("file in folder: file is being written", func(t *testing.T) {
		ctx, fls := createFS(t)
		defer ctx.CancelGracefully()

		utils.PanicIfErr(fls.MkdirAll("/dir", DEFAULT_DIR_FMODE))

		f, err := fls.OpenFile("/dir/a.ix", os.O_CREATE|os.O_WRONLY, DEFAULT_FILE_FMODE)
		if !assert.NoError(t, err) {
			return
		}

		wg := new(sync.WaitGroup)
		var snapshotDone atomic.Bool
		wg.Add(1)

		go func() {
			defer wg.Done()
			defer f.Close()
			for !snapshotDone.Load() {
				f.Write([]byte("a"))
			}
		}()

		time.Sleep(time.Millisecond)
		snapshot, err := fls.TakeFilesystemSnapshot(core.FilesystemSnapshotConfig{
			GetContent:       snapshotConfig.GetContent,
			InclusionFilters: []core.PathPattern{"/**/*.ix"},
		})
		if !assert.NoError(t, err) {
			return
		}

		snapshotDone.Store(true)
		wg.Wait()

		if !assert.Len(t, snapshot.RootDirEntries(), 1) {
			return
		}

		metadata, err := snapshot.Metadata("/")
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, []string{"dir"}, metadata.ChildNames)

		metadata, err = snapshot.Metadata("/dir")
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, []string{"a.ix"}, metadata.ChildNames)
	})
}

func testSnapshotWriteToFilesystem(t *testing.T, createFS func(*testing.T) (*core.Context, core.SnapshotableFilesystem)) {
	//TODO
}
