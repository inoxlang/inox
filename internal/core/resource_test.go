package core

import (
	"runtime"
	"testing"

	utils "github.com/inoxlang/inox/internal/utils/common"
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

func TestPath(t *testing.T) {
	t.Run("ToPrefixPattern()", func(t *testing.T) {
		assert.EqualValues(t, "/...", Path("/").ToPrefixPattern())
		assert.EqualValues(t, "/\\*/...", Path("/*/").ToPrefixPattern())
		assert.EqualValues(t, "/\\?/...", Path("/?/").ToPrefixPattern())
		assert.EqualValues(t, "/\\[x]/...", Path("/[x]/").ToPrefixPattern())

		assert.EqualValues(t, "/\\*/\\*/...", Path("/*/*/").ToPrefixPattern())

		assert.Panics(t, func() {
			Path("/a").ToPrefixPattern()
		})

		assert.Panics(t, func() {
			Path("/*/a").ToPrefixPattern()
		})
	})
}

func TestHost(t *testing.T) {
	httpHost := Host("http://example.com")
	assert.True(t, httpHost.HasScheme())
	assert.True(t, httpHost.HasHttpScheme())
	assert.Equal(t, Scheme("http"), httpHost.Scheme())
	assert.Equal(t, httpHost, httpHost.HostWithoutPort())
	assert.Equal(t, "example.com", httpHost.WithoutScheme())

	httpHostWithPort := Host("http://example.com:80")
	assert.True(t, httpHostWithPort.HasScheme())
	assert.True(t, httpHostWithPort.HasHttpScheme())
	assert.Equal(t, Scheme("http"), httpHostWithPort.Scheme())
	assert.Equal(t, Host("http://example.com"), httpHostWithPort.HostWithoutPort())
	assert.Equal(t, "example.com:80", httpHostWithPort.WithoutScheme())

	ldbHost := Host("ldb://main")
	assert.True(t, ldbHost.HasScheme())
	assert.False(t, ldbHost.HasHttpScheme())
	assert.Equal(t, Scheme("ldb"), ldbHost.Scheme())
	assert.Equal(t, Host("ldb://main"), ldbHost.HostWithoutPort())
	assert.Equal(t, "main", ldbHost.WithoutScheme())

	schemelessHost := Host("://example.com")
	assert.False(t, schemelessHost.HasScheme())
	assert.False(t, schemelessHost.HasHttpScheme())
	assert.Equal(t, NO_SCHEME_SCHEME_NAME, schemelessHost.Scheme())
	assert.Equal(t, Host("://example.com"), schemelessHost.HostWithoutPort())
	assert.Equal(t, "example.com", schemelessHost.WithoutScheme())

	schemelessHostWithPort := Host("://example.com:80")
	assert.False(t, schemelessHostWithPort.HasScheme())
	assert.False(t, schemelessHostWithPort.HasHttpScheme())
	assert.Equal(t, NO_SCHEME_SCHEME_NAME, schemelessHostWithPort.Scheme())
	assert.Equal(t, Host("://example.com:80"), schemelessHostWithPort.HostWithoutPort())
	assert.Equal(t, "example.com:80", schemelessHostWithPort.WithoutScheme())
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

	t.Run("IsDir", func(t *testing.T) {
		assert.True(t, URL("https://example.com/").IsDir())
		assert.True(t, URL("https://example.com/a/").IsDir())

		assert.False(t, URL("https://example.com/a?").IsDir())
		assert.False(t, URL("https://example.com/a/b?").IsDir())

		assert.False(t, URL("https://example.com/a#").IsDir())
		assert.False(t, URL("https://example.com/a/b#").IsDir())

		assert.False(t, URL("https://example.com/a").IsDir())
		assert.False(t, URL("https://example.com/a/b").IsDir())
	})

	t.Run("IsDirOf", func(t *testing.T) {
		t.Run("'root' dir", func(t *testing.T) {
			dir1 := URL("https://example.com/")

			yes, err := dir1.IsDirOf("https://example.com/a")
			if assert.NoError(t, err) {
				assert.True(t, yes)
			}

			yes, err = dir1.IsDirOf("https://example.com//a")
			if assert.NoError(t, err) {
				assert.True(t, yes)
			}

			yes, err = dir1.IsDirOf("https://example.com/a/")
			if assert.NoError(t, err) {
				assert.True(t, yes)
			}

			yes, err = dir1.IsDirOf("https://example.com/a//")
			if assert.NoError(t, err) {
				assert.True(t, yes)
			}

			yes, err = dir1.IsDirOf("https://example.com/a/b")
			if assert.NoError(t, err) {
				assert.False(t, yes)
			}

			yes, err = dir1.IsDirOf("https://example.com/a?")
			if assert.Error(t, err) {
				assert.False(t, yes)
			}
		})

		t.Run("'root' dir with query", func(t *testing.T) {
			dir1 := URL("https://example.com/?")

			yes, err := dir1.IsDirOf("https://example.com/a")
			if assert.Error(t, err) {
				assert.False(t, yes)
			}

			yes, err = dir1.IsDirOf("https://example.com/a/")
			if assert.Error(t, err) {
				assert.False(t, yes)
			}
		})

		t.Run("'root' dir with fragment", func(t *testing.T) {
			dir1 := URL("https://example.com/#")

			yes, err := dir1.IsDirOf("https://example.com/a")
			if assert.Error(t, err) {
				assert.False(t, yes)
			}

			yes, err = dir1.IsDirOf("https://example.com/a/")
			if assert.Error(t, err) {
				assert.False(t, yes)
			}
		})

		t.Run("non 'root' dir", func(t *testing.T) {
			dir1 := URL("https://example.com/a/")

			yes, err := dir1.IsDirOf("https://example.com/a/b")
			if assert.NoError(t, err) {
				assert.True(t, yes)
			}

			yes, err = dir1.IsDirOf("https://example.com/a/b/")
			if assert.NoError(t, err) {
				assert.True(t, yes)
			}

			yes, err = dir1.IsDirOf("https://example.com/a/b//")
			if assert.NoError(t, err) {
				assert.True(t, yes)
			}

			yes, err = dir1.IsDirOf("https://example.com/a/b/c")
			if assert.NoError(t, err) {
				assert.False(t, yes)
			}

			yes, err = dir1.IsDirOf("https://example.com/a/b#")
			if assert.Error(t, err) {
				assert.False(t, yes)
			}
		})

		t.Run("non dir", func(t *testing.T) {
			nonDir := URL("https://example.com/a")

			yes, err := nonDir.IsDirOf("https://example.com/a/b")
			if assert.Error(t, err) {
				assert.False(t, yes)
			}
		})
	})

	t.Run("TruncatedBeforeQuery", func(t *testing.T) {
		u := URL("https://example.com/")
		assert.Equal(t, u, u.TruncatedBeforeQuery())

		u = URL("https://example.com/a")
		assert.Equal(t, u, u.TruncatedBeforeQuery())

		u = URL("https://example.com/?")
		assert.Equal(t, URL("https://example.com/"), u.TruncatedBeforeQuery())

		//A path should be added if there is no remaining URL-specific feature.
		u = URL("https://example.com?")
		assert.Equal(t, URL("https://example.com/"), u.TruncatedBeforeQuery())

		u = URL("https://example.com/?#")
		assert.Equal(t, URL("https://example.com/"), u.TruncatedBeforeQuery())

		u = URL("https://example.com?#")
		assert.Equal(t, URL("https://example.com/"), u.TruncatedBeforeQuery())

		u = URL("https://example.com/#")
		assert.Equal(t, URL("https://example.com/#"), u.TruncatedBeforeQuery())

		u = URL("https://example.com#")
		assert.Equal(t, URL("https://example.com#"), u.TruncatedBeforeQuery())
	})

	t.Run("WithoutQueryNorFragment", func(t *testing.T) {
		u := URL("https://example.com/")
		assert.Equal(t, u, u.WithoutQueryNorFragment())

		u = URL("https://example.com/a")
		assert.Equal(t, u, u.WithoutQueryNorFragment())

		u = URL("https://example.com/?")
		assert.Equal(t, URL("https://example.com/"), u.WithoutQueryNorFragment())

		//A path should be added if there is no remaining URL-specific feature.
		u = URL("https://example.com?")
		assert.Equal(t, URL("https://example.com/"), u.WithoutQueryNorFragment())

		u = URL("https://example.com/?#")
		assert.Equal(t, URL("https://example.com/"), u.WithoutQueryNorFragment())

		//A path should be added if there is no remaining URL-specific feature.
		u = URL("https://example.com?#")
		assert.Equal(t, URL("https://example.com/"), u.WithoutQueryNorFragment())

		u = URL("https://example.com/#")
		assert.Equal(t, URL("https://example.com/"), u.WithoutQueryNorFragment())

		//A path should be added if there is no remaining URL-specific feature.
		u = URL("https://example.com#")
		assert.Equal(t, URL("https://example.com/"), u.WithoutQueryNorFragment())
	})
}

func TestAppendPathSegmentToURLPattern(t *testing.T) {
	assert.Equal(t, "ldb://main/users/1", appendPathSegmentToURLPattern("ldb://main/users", "1"))
	assert.Equal(t, "ldb://main/users/1?", appendPathSegmentToURLPattern("ldb://main/users?", "1"))
	assert.Equal(t, "ldb://main/users/1#", appendPathSegmentToURLPattern("ldb://main/users#", "1"))

	assert.Equal(t, "ldb://main/users/1", appendPathSegmentToURLPattern("ldb://main/users/", "1"))
	assert.Equal(t, "ldb://main/users/1?", appendPathSegmentToURLPattern("ldb://main/users/?", "1"))
	assert.Equal(t, "ldb://main/users/1#", appendPathSegmentToURLPattern("ldb://main/users/#", "1"))

	assert.Equal(t, "ldb://main/users/%int/1", appendPathSegmentToURLPattern("ldb://main/users/%int", "1"))
	assert.Equal(t, "ldb://main/users/%int/1?", appendPathSegmentToURLPattern("ldb://main/users/%int?", "1"))
	assert.Equal(t, "ldb://main/users/%int/1#", appendPathSegmentToURLPattern("ldb://main/users/%int#", "1"))

	assert.Equal(t, "ldb://main/1", appendPathSegmentToURLPattern("ldb://main/", "1"))
	assert.Equal(t, "ldb://main/1?", appendPathSegmentToURLPattern("ldb://main/?", "1"))
	assert.Equal(t, "ldb://main/1#", appendPathSegmentToURLPattern("ldb://main/#", "1"))

	//assert.Equal(t, "ldb://main/1", appendPathSegmentToURLPattern("ldb://main", "1"))
	assert.Equal(t, "ldb://main/1?", appendPathSegmentToURLPattern("ldb://main?", "1"))
	assert.Equal(t, "ldb://main/1#", appendPathSegmentToURLPattern("ldb://main#", "1"))

	assert.Panics(t, func() {
		appendPathSegmentToURLPattern("ldb://main/users", "1/")
	})
	assert.Panics(t, func() {
		appendPathSegmentToURLPattern("ldb://main/users", "1/2")
	})
}
