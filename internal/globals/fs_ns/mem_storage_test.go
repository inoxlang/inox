package fs_ns

import (
	"errors"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestMemoryStorage(t *testing.T) {

	//TODO: add tests

	t.Run("creation time & modification time should be set at creation", func(t *testing.T) {
		maxStorage := core.ByteCount(100)
		storage := newInMemoryStorage(maxStorage)

		now := time.Now().Truncate(time.Millisecond)

		file := utils.Must(storage.New("file", 0600, os.O_WRONLY))

		assert.Equal(t, now, file.content.creationTime.Truncate(time.Millisecond))
		assert.Equal(t, now, file.content.modificationTime.Load().(time.Time).Truncate(time.Millisecond))

		assert.NoError(t, file.Close())
	})

	t.Run("writing too much to the same file should cause an error", func(t *testing.T) {
		maxStorage := core.ByteCount(100)
		storage := newInMemoryStorage(maxStorage)

		file := utils.Must(storage.New("file", 0600, os.O_WRONLY))

		for i := 0; i < int(maxStorage)+1; i++ {
			file, _ := storage.Get("file")

			_, err := file.Write([]byte{'a'})

			if i == int(maxStorage)+1 {
				assert.ErrorIs(t, err, ErrInMemoryStorageLimitExceededDuringWrite)
				break
			} else if !assert.NoError(t, err) {
				return
			}

		}
		assert.NoError(t, file.Close())
	})

	t.Run("truncating a file should update the modification time", func(t *testing.T) {
		maxStorage := core.ByteCount(100)
		storage := newInMemoryStorage(maxStorage)

		now := time.Now()
		file := utils.Must(storage.New("file", 0600, os.O_WRONLY))
		err := file.Truncate(10)

		info, _ := file.Stat()
		assert.True(t, info.ModTime().After(now))

		assert.NoError(t, err)
		assert.NoError(t, file.Close())
	})

	t.Run("truncating a file to a large size should cause an error", func(t *testing.T) {
		maxStorage := core.ByteCount(100)
		storage := newInMemoryStorage(maxStorage)

		file := utils.Must(storage.New("file", 0600, os.O_WRONLY))

		err := file.Truncate(int64(maxStorage) + 1)
		assert.ErrorIs(t, err, ErrInMemoryStorageLimitExceededDuringWrite)
		assert.NoError(t, file.Close())
	})

	t.Run("creating small regular files in parallel should be thread safe", func(t *testing.T) {
		goroutineCount := 10
		singleWriteData := []byte{'a'}
		singleGoroutineFiles := 10

		storage := newInMemoryStorage(core.ByteCount(goroutineCount * len(singleWriteData) * singleGoroutineFiles))

		wg := new(sync.WaitGroup)

		wg.Add(goroutineCount)

		for i := 0; i < goroutineCount; i++ {
			go func(i string) {
				defer wg.Done()
				time.Sleep(time.Microsecond)

				for index := 0; index < singleGoroutineFiles; index++ {
					file := utils.Must(storage.New("file"+i+strconv.Itoa(rand.Int()), 0600, os.O_WRONLY))
					utils.Must(file.Write(singleWriteData))
					assert.NoError(t, file.Close())
				}
			}(strconv.Itoa(i))
		}

		wg.Wait()
	})

	t.Run("creating too many small regular files in parallel should cause an error", func(t *testing.T) {
		goroutineCount := 10
		singleWriteData := []byte{'a'}
		singleGoroutineFiles := 10

		//an additional byte of storage is needed, so we should get a single error.
		storage := newInMemoryStorage(core.ByteCount(goroutineCount*len(singleWriteData)*singleGoroutineFiles) - 1)

		wg := new(sync.WaitGroup)

		wg.Add(goroutineCount)

		var actualError error
		var actualErrLock sync.Mutex //prevent data race for access to actualError
		var errCount atomic.Int32

		for i := 0; i < goroutineCount; i++ {
			go func(i string) {
				defer wg.Done()
				time.Sleep(time.Microsecond)

				for index := 0; index < singleGoroutineFiles; index++ {
					file := utils.Must(storage.New("file"+i+strconv.Itoa(rand.Int()), 0600, os.O_WRONLY))

					_, err := file.Write(singleWriteData)
					if err != nil {
						if errCount.Add(1) == 1 {
							actualErrLock.Lock()
							actualError = err
							actualErrLock.Unlock()
						}
					}

					assert.NoError(t, file.Close())
				}
			}(strconv.Itoa(i))
		}

		wg.Wait()

		if !assert.ErrorIs(t, actualError, ErrInMemoryStorageLimitExceededDuringWrite) {
			return
		}

		assert.Equal(t, int32(1), errCount.Load())
	})

	t.Run("writing to the same file should be thread safe", func(t *testing.T) {
		storage := newInMemoryStorage(1000)

		file := utils.Must(storage.New("file", 0400, os.O_WRONLY))
		file.Close()

		wg := new(sync.WaitGroup)
		goroutineCount := 1000

		wg.Add(goroutineCount)

		var panicErr error

		for i := 0; i < goroutineCount; i++ {
			go func() {
				defer wg.Done()
				defer func() {
					e := recover()
					if e != nil {
						panicErr = e.(error)
					}
				}()
				time.Sleep(time.Microsecond)

				for i := 0; i < 10; i++ {
					file, _ := storage.Get("file")
					_, err := file.Write([]byte{'a'})

					for errors.Is(err, os.ErrClosed) {
						file, _ = storage.Get("file")
						_, err = file.Write([]byte{'a'})
					}

					file.Close()
				}
			}()
		}

		//TODO: add assertions
		wg.Wait()

		if !assert.NoError(t, panicErr) {
			return
		}

		file, _ = storage.Get("file")
		assert.Equal(t, 10*goroutineCount, file.content.Len())
	})

	t.Run("Stat() should be thread safe", func(t *testing.T) {
		storage := newInMemoryStorage(1000)

		file := utils.Must(storage.New("file", 0400, os.O_WRONLY))
		file.Close()

		wg := new(sync.WaitGroup)
		goroutineCount := 1000

		wg.Add(goroutineCount)

		var panicErr atomic.Value

		for i := 0; i < goroutineCount; i++ {
			go func() {
				defer wg.Done()
				defer func() {
					e := recover()
					if e != nil {
						panicErr.Store(e)
					}
				}()
				time.Sleep(time.Microsecond)

				for i := 0; i < 10; i++ {
					file, _ := storage.Get("file")

					info, _ := file.Stat()
					size := info.Size()

					_, err := file.Write([]byte{'a'})

					for errors.Is(err, os.ErrClosed) {
						file, _ = storage.Get("file")
						_, err = file.Write([]byte{'a'})
					}

					newInfo, _ := file.Stat()
					newSize := newInfo.Size()

					if newSize <= size {
						panicErr.Store(errors.New("size should have increased"))
					}

					file.Close()
				}
			}()
		}

		//TODO: add assertions
		wg.Wait()

		if v := panicErr.Load(); v != nil && v != error(nil) {
			assert.FailNow(t, v.(error).Error())
		}

		file, _ = storage.Get("file")
		assert.Equal(t, 10*goroutineCount, file.content.Len())
	})

	t.Run("truncating the same file should be thread safe", func(t *testing.T) {
		storage := newInMemoryStorage(1000)

		file := utils.Must(storage.New("file", 0400, os.O_WRONLY))
		file.Close()

		wg := new(sync.WaitGroup)
		goroutineCount := 100

		wg.Add(goroutineCount)

		for i := 0; i < goroutineCount; i++ {
			go func() {
				defer wg.Done()
				time.Sleep(time.Microsecond)

				for i := 0; i < 10; i++ {
					file, _ := storage.Get("file")
					file.Truncate(int64(i))
					file.Close()
				}
			}()
		}

		//TODO: add assertions
		wg.Wait()
	})

	t.Run("Persist should not call the callback function if file is not dirty", func(t *testing.T) {
		storage := newInMemoryStorage(1000)

		file := utils.Must(storage.New("file", 0400, os.O_WRONLY))
		defer file.Close()

		called := false
		err := file.content.Persist(func(p []byte) error {
			called = true
			return nil
		})

		assert.NoError(t, err)
		assert.False(t, called)
		assert.False(t, file.content.IsDirty())
	})

	t.Run("Persist should call the callback function if file is dirty", func(t *testing.T) {
		storage := newInMemoryStorage(1000)

		file := utils.Must(storage.New("file", 0400, os.O_WRONLY))
		defer file.Close()

		file.Write([]byte{'a'})
		assert.True(t, file.content.IsDirty())

		called := false
		err := file.content.Persist(func(p []byte) error {
			assert.True(t, file.content.IsDirty())

			called = true
			assert.Equal(t, []byte{'a'}, p)
			return nil
		})

		assert.NoError(t, err)
		assert.True(t, called)
		assert.False(t, file.content.IsDirty())
	})

	t.Run("Persist should recover if the persist callback function panics", func(t *testing.T) {
		storage := newInMemoryStorage(1000)

		file := utils.Must(storage.New("file", 0400, os.O_WRONLY))
		defer file.Close()

		file.Write([]byte{'a'})
		assert.True(t, file.content.IsDirty())

		panicErr := errors.New("!")
		err := file.content.Persist(func(p []byte) error {
			panic(panicErr)
		})

		assert.ErrorIs(t, err, panicErr)
		assert.True(t, file.content.IsDirty())
	})
}
