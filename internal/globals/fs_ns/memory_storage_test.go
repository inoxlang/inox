package fs_ns

import (
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/utils"
)

func TestMemoryStorage(t *testing.T) {

	//TODO: add tests

	t.Run("creating different regular files in parallel should be thread safe", func(t *testing.T) {
		storage := newInMemoryStorage()

		file := utils.Must(storage.New("file", 0400, 0))
		file.Close()

		wg := new(sync.WaitGroup)
		goroutineCount := 10

		wg.Add(goroutineCount)

		for i := 0; i < goroutineCount; i++ {
			go func(i string) {
				defer wg.Done()
				time.Sleep(time.Microsecond)

				for index := 0; index < 10; index++ {
					file := utils.Must(storage.New("file"+i+strconv.Itoa(rand.Int()), 0400, 0))
					file.Write([]byte{'a'})
					file.Close()
				}
			}(strconv.Itoa(i))
		}

		wg.Wait()
	})

	t.Run("writing to the same file should be thread safe", func(t *testing.T) {
		storage := newInMemoryStorage()

		file := utils.Must(storage.New("file", 0400, 0))
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
					file.Write([]byte{'a'})
					file.Close()
				}
			}()
		}

		wg.Wait()
	})

}
