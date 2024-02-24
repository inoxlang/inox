//go:build unix

package fs_ns

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
	"gopkg.in/check.v1"
)

var _ = check.Suite(&MemoryFsTestSuite{})

type OSFsWithContextTestSuite struct {
	BasicTestSuite
	DirTestSuite
}

func (s *OSFsWithContextTestSuite) SetUpTest(c *check.C) {
	ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)

	s.BasicTestSuite = BasicTestSuite{
		FS: GetOsFilesystem().WithSecondaryContext(ctx).(*OsFilesystem),
	}
	s.DirTestSuite = DirTestSuite{
		FS: GetOsFilesystem().WithSecondaryContext(ctx).(*OsFilesystem),
	}
}

func TestOSFsWithContextFilesystem(t *testing.T) {
	result := check.Run(&MemoryFsTestSuite{}, &check.RunConf{
		Verbose: true,
	})

	if result.Failed > 0 || result.Panicked > 0 {
		assert.Fail(t, result.String())
	}
}
