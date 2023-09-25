package fs_ns

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
	"gopkg.in/check.v1"
)

func TestMetaFilesystemWithUnderlyingFs(t *testing.T) {
	result := check.Run(&MetaFsWithUnderlyingFsTestSuite{}, &check.RunConf{
		Verbose: true,
	})

	if result.Failed > 0 || result.Panicked > 0 {
		assert.Fail(t, result.String())
		return
	}

	testCount := result.Succeeded
	resultWhenClosed := check.Run(&MetaFsWithUnderlyingFsTestSuite{closed: true}, &check.RunConf{
		Verbose: true,
	})

	if resultWhenClosed.Failed+resultWhenClosed.Panicked != testCount-1 {
		assert.Fail(t, "all tests expected one should have failed: \n"+resultWhenClosed.String())
		return
	}
}

func TestMetaFilesystemWithBasic(t *testing.T) {
	result := check.Run(&MetaFsTestSuite{}, &check.RunConf{
		Verbose: true,
	})

	if result.Failed > 0 || result.Panicked > 0 {
		assert.Fail(t, result.String())
	}

	testCount := result.Succeeded
	resultWhenClosed := check.Run(&MetaFsWithUnderlyingFsTestSuite{closed: true}, &check.RunConf{
		Verbose: true,
	})

	if resultWhenClosed.Failed+resultWhenClosed.Panicked != testCount-1 {
		assert.Fail(t, "all tests expected one should have failed: \n"+resultWhenClosed.String())
		return
	}
}

type MetaFsWithUnderlyingFsTestSuite struct {
	closed   bool
	contexts []*core.Context

	BasicTestSuite
	DirTestSuite
}

func (s *MetaFsWithUnderlyingFsTestSuite) SetUpTest(c *check.C) {

	createMetaFS := func() *MetaFilesystem {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		s.contexts = append(s.contexts, ctx)
		underlyingFS := NewMemFilesystem(100_000_000)

		fls, err := OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemOptions{
			Dir: "/metafs/",
		})
		if err != nil {
			panic(err)
		}
		if s.closed {
			fls.Close(ctx)
		}
		return fls
	}

	s.BasicTestSuite = BasicTestSuite{
		FS: createMetaFS(),
	}
	s.DirTestSuite = DirTestSuite{
		FS: createMetaFS(),
	}
}

func (s *MetaFsWithUnderlyingFsTestSuite) TearDownTest(c *check.C) {
	for _, ctx := range s.contexts {
		ctx.CancelGracefully()
	}
}

type MetaFsTestSuite struct {
	closed   bool
	contexts []*core.Context

	BasicTestSuite
	DirTestSuite
}

func (s *MetaFsTestSuite) SetUpTest(c *check.C) {

	createMetaFS := func() *MetaFilesystem {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		s.contexts = append(s.contexts, ctx)
		underlyingFS := NewMemFilesystem(100_000_000)

		//no dir provided
		fls, err := OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemOptions{})
		if err != nil {
			panic(err)
		}
		if s.closed {
			fls.Close(ctx)
		}
		return fls
	}

	s.BasicTestSuite = BasicTestSuite{
		FS: createMetaFS(),
	}
	s.DirTestSuite = DirTestSuite{
		FS: createMetaFS(),
	}
}

func (s *MetaFsTestSuite) TearDownTest(c *check.C) {
	for _, ctx := range s.contexts {
		ctx.CancelGracefully()
	}
}
