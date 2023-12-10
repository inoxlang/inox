package core

import (
	"runtime"
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestResourceGraph(t *testing.T) {
	{
		runtime.GC()
		startMemStats := new(runtime.MemStats)
		runtime.ReadMemStats(startMemStats)

		defer utils.AssertNoMemoryLeak(t, startMemStats, 10, utils.AssertNoMemoryLeakOptions{
			CheckGoroutines: true,
			GoroutineCount:  runtime.NumGoroutine(),
		})
	}

	t.Run("", func(t *testing.T) {
		g := NewResourceGraph()

		g.AddResource(Path("/main.ix"), "module")
		g.AddResource(Path("/lib.ix"), "module")
		g.AddEdge(Path("/main.ix"), Path("/lib.ix"), CHUNK_IMPORT_MOD_REL)
	})
}

func TestURL(t *testing.T) {
	t.Run("AppendRelativePath", func(t *testing.T) {
		url := URL("https://example.com/")

		t.Run("relative path argument starting with .", func(t *testing.T) {
			assert.Equal(t, URL(url+"a"), url.AppendRelativePath("./a"))
		})

		t.Run("path duplicated in query param", func(t *testing.T) {
			url := URL("https://example.com/path/?x=/path/")
			expectedURL := URL("https://example.com/path/a?x=/path/")
			assert.Equal(t, expectedURL, url.AppendRelativePath("./a"))
		})

		t.Run("non-suffixed path duplicated in query param", func(t *testing.T) {
			url := URL("https://example.com/path/?x=/path")
			expectedURL := URL("https://example.com/path/a?x=/path")
			assert.Equal(t, expectedURL, url.AppendRelativePath("./a"))
		})

		t.Run("URL not ending with '/'", func(t *testing.T) {
			assert.Panics(t, func() {
				URL("https://example.com/x").AppendRelativePath("./a")
			})
		})

		t.Run("relative path argument starting with ..", func(t *testing.T) {
			assert.Panics(t, func() {
				url.AppendRelativePath("../a")
			})
		})

		t.Run("absolute path argument", func(t *testing.T) {
			assert.Panics(t, func() {
				url.AppendRelativePath("/a")
			})
		})
	})

	t.Run("AppendAbsolutePath", func(t *testing.T) {
		url := URL("https://example.com/")

		t.Run("relative path argument starting with .", func(t *testing.T) {
			assert.Equal(t, URL(url+"a"), url.AppendAbsolutePath("/a"))
		})

		t.Run("path duplicated in query param", func(t *testing.T) {
			url := URL("https://example.com/path/?x=/path/")
			expectedURL := URL("https://example.com/path/a?x=/path/")
			assert.Equal(t, expectedURL, url.AppendAbsolutePath("/a"))
		})

		t.Run("non-suffixed path duplicated in query param", func(t *testing.T) {
			url := URL("https://example.com/path/?x=/path")
			expectedURL := URL("https://example.com/path/a?x=/path")
			assert.Equal(t, expectedURL, url.AppendAbsolutePath("/a"))
		})

		t.Run("URL not ending with '/'", func(t *testing.T) {
			assert.Panics(t, func() {
				URL("https://example.com/x").AppendAbsolutePath("./a")
			})
		})

		t.Run("relative path", func(t *testing.T) {
			assert.Panics(t, func() {
				url.AppendAbsolutePath("./a")
			})
		})
	})
}
