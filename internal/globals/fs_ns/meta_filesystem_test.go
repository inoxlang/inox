package fs_ns

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"gopkg.in/check.v1"
)

func TestMetaFilesystemWithUnderlyingFs(t *testing.T) {
	check.Run(&MetaFsWithUnderlyingFsTestSuite{}, &check.RunConf{
		Verbose: true,
	})
}

func TestMetaFilesystemWithBasic(t *testing.T) {
	check.Run(&MetaFsTestSuite{}, &check.RunConf{
		Verbose: true,
	})
}

type MetaFsWithUnderlyingFsTestSuite struct {
	BasicTestSuite
	DirTestSuite
}

func (s *MetaFsWithUnderlyingFsTestSuite) SetUpTest(c *check.C) {

	createMetaFS := func() *MetaFilesystem {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		underlyingFS := NewMemFilesystem(100_000_000)

		fls, err := OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemOptions{
			Dir: "/metafs/",
		})
		if err != nil {
			panic(err)
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
	//
}

type MetaFsTestSuite struct {
	BasicTestSuite
	DirTestSuite
}

func (s *MetaFsTestSuite) SetUpTest(c *check.C) {

	createMetaFS := func() *MetaFilesystem {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		underlyingFS := NewMemFilesystem(100_000_000)

		//no dir provided
		fls, err := OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemOptions{})
		if err != nil {
			panic(err)
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
	//
}
