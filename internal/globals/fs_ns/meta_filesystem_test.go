package fs_ns

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"gopkg.in/check.v1"
)

type MetaFsTestSuite struct {
	BasicTestSuite
	//DirTestSuite
}

func (s *MetaFsTestSuite) SetUpTest(c *check.C) {

	createMetaFS := func() *MetaFilesystem {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		underlyingFS := NewMemFilesystem(100_000_000)

		fls, err := OpenMetaFilesystem(ctx, underlyingFS, "/metafs/")
		if err != nil {
			panic(err)
		}
		return fls
	}

	s.BasicTestSuite = BasicTestSuite{
		FS: createMetaFS(),
	}
	// s.DirTestSuite = DirTestSuite{
	// 	FS: createMetaFS(),
	// }
}

func TestMetaFilesystem(t *testing.T) {
	check.Run(&MetaFsTestSuite{}, &check.RunConf{
		Verbose: true,
	})
}
