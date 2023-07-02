//slight modification of https://github.com/go-git/go-billy/blob/master/test/dir.go

package fs_ns

import (
	"os"
	"strconv"

	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"gopkg.in/check.v1"
)

// DirTestSuite is a convenient test suite to validate any implementation of
// billy.Dir
type DirTestSuite struct {
	FS interface {
		billy.Basic
		billy.Dir
	}
}

func (s *DirTestSuite) TestMkdirAll(c *check.C) {
	err := s.FS.MkdirAll("empty", os.FileMode(0755))
	c.Assert(err, check.IsNil)

	fi, err := s.FS.Stat("empty")
	c.Assert(err, check.IsNil)
	c.Assert(fi.IsDir(), check.Equals, true)
}

func (s *DirTestSuite) TestMkdirAllNested(c *check.C) {
	err := s.FS.MkdirAll("foo/bar/baz", os.FileMode(0755))
	c.Assert(err, check.IsNil)

	fi, err := s.FS.Stat("foo/bar/baz")
	c.Assert(err, check.IsNil)
	c.Assert(fi.IsDir(), check.Equals, true)

	fi, err = s.FS.Stat("foo/bar")
	c.Assert(err, check.IsNil)
	c.Assert(fi.IsDir(), check.Equals, true)

	fi, err = s.FS.Stat("foo")
	c.Assert(err, check.IsNil)
	c.Assert(fi.IsDir(), check.Equals, true)
}

func (s *DirTestSuite) TestMkdirAllIdempotent(c *check.C) {
	err := s.FS.MkdirAll("empty", 0755)
	c.Assert(err, check.IsNil)
	fi, err := s.FS.Stat("empty")
	c.Assert(err, check.IsNil)
	c.Assert(fi.IsDir(), check.Equals, true)

	// idempotent
	err = s.FS.MkdirAll("empty", 0755)
	c.Assert(err, check.IsNil)
	fi, err = s.FS.Stat("empty")
	c.Assert(err, check.IsNil)
	c.Assert(fi.IsDir(), check.Equals, true)
}

func (s *DirTestSuite) TestMkdirAllAndCreate(c *check.C) {
	err := s.FS.MkdirAll("dir", os.FileMode(0755))
	c.Assert(err, check.IsNil)

	f, err := s.FS.Create("dir/bar/foo")
	c.Assert(err, check.IsNil)
	c.Assert(f.Close(), check.IsNil)

	fi, err := s.FS.Stat("dir/bar/foo")
	c.Assert(err, check.IsNil)
	c.Assert(fi.IsDir(), check.Equals, false)
}

func (s *DirTestSuite) TestMkdirAllWithExistingFile(c *check.C) {
	f, err := s.FS.Create("dir/foo")
	c.Assert(err, check.IsNil)
	c.Assert(f.Close(), check.IsNil)

	err = s.FS.MkdirAll("dir/foo", os.FileMode(0755))
	c.Assert(err, check.NotNil)

	fi, err := s.FS.Stat("dir/foo")
	c.Assert(err, check.IsNil)
	c.Assert(fi.IsDir(), check.Equals, false)
}

func (s *DirTestSuite) TestStatDir(c *check.C) {
	s.FS.MkdirAll("foo/bar", 0755)

	fi, err := s.FS.Stat("foo/bar")
	c.Assert(err, check.IsNil)
	c.Assert(fi.Name(), check.Equals, "bar")
	c.Assert(fi.Mode().IsDir(), check.Equals, true)
	c.Assert(fi.ModTime().IsZero(), check.Equals, false)
	c.Assert(fi.IsDir(), check.Equals, true)
}

func (s *DirTestSuite) TestStatRootDir(c *check.C) {
	fi, err := s.FS.Stat("/")
	c.Assert(err, check.IsNil)
	c.Assert(fi.Name(), check.Equals, "/")
	c.Assert(fi.Mode().IsDir(), check.Equals, true)
	c.Assert(fi.ModTime().IsZero(), check.Equals, false)
	c.Assert(fi.IsDir(), check.Equals, true)
}

func (s *BasicTestSuite) TestStatDeep(c *check.C) {
	files := []string{"foo", "bar", "qux/baz", "qux/qux"}
	for _, name := range files {
		err := util.WriteFile(s.FS, name, nil, 0644)
		c.Assert(err, check.IsNil)
	}

	// Some implementations detect directories based on a prefix
	// for all files; it's easy to miss path separator handling there.
	fi, err := s.FS.Stat("qu")
	c.Assert(os.IsNotExist(err), check.Equals, true, check.Commentf("error: %s", err))
	c.Assert(fi, check.IsNil)

	fi, err = s.FS.Stat("qux")
	c.Assert(err, check.IsNil)
	c.Assert(fi.Name(), check.Equals, "qux")
	c.Assert(fi.IsDir(), check.Equals, true)

	fi, err = s.FS.Stat("qux/baz")
	c.Assert(err, check.IsNil)
	c.Assert(fi.Name(), check.Equals, "baz")
	c.Assert(fi.IsDir(), check.Equals, false)
}

func (s *DirTestSuite) TestReadDir(c *check.C) {
	files := []string{"foo", "bar", "qux/baz", "qux/qux"}
	for _, name := range files {
		err := util.WriteFile(s.FS, name, nil, 0644)
		c.Assert(err, check.IsNil)
	}

	info, err := s.FS.ReadDir("/")
	c.Assert(err, check.IsNil)
	c.Assert(info, check.HasLen, 3)

	info, err = s.FS.ReadDir("/qux")
	c.Assert(err, check.IsNil)
	c.Assert(info, check.HasLen, 2)
}

func (s *DirTestSuite) TestReadDirNested(c *check.C) {
	max := 100
	path := "/"
	for i := 0; i <= max; i++ {
		path = s.FS.Join(path, strconv.Itoa(i))
	}

	files := []string{s.FS.Join(path, "f1"), s.FS.Join(path, "f2")}
	for _, name := range files {
		err := util.WriteFile(s.FS, name, nil, 0644)
		c.Assert(err, check.IsNil)
	}

	path = "/"
	for i := 0; i < max; i++ {
		path = s.FS.Join(path, strconv.Itoa(i))
		info, err := s.FS.ReadDir(path)
		c.Assert(err, check.IsNil)
		c.Assert(info, check.HasLen, 1)
	}

	path = s.FS.Join(path, strconv.Itoa(max))
	info, err := s.FS.ReadDir(path)
	c.Assert(err, check.IsNil)
	c.Assert(info, check.HasLen, 2)
}

func (s *DirTestSuite) TestReadDirWithMkDirAll(c *check.C) {
	err := s.FS.MkdirAll("qux", 0755)
	c.Assert(err, check.IsNil)

	files := []string{"qux/baz", "qux/qux"}
	for _, name := range files {
		err := util.WriteFile(s.FS, name, nil, 0644)
		c.Assert(err, check.IsNil)
	}

	info, err := s.FS.ReadDir("/")
	c.Assert(err, check.IsNil)
	c.Assert(info, check.HasLen, 1)
	c.Assert(info[0].IsDir(), check.Equals, true)

	info, err = s.FS.ReadDir("/qux")
	c.Assert(err, check.IsNil)
	c.Assert(info, check.HasLen, 2)
}

func (s *DirTestSuite) TestReadDirFileInfo(c *check.C) {
	err := util.WriteFile(s.FS, "foo", []byte{'F', 'O', 'O'}, 0644)
	c.Assert(err, check.IsNil)

	info, err := s.FS.ReadDir("/")
	c.Assert(err, check.IsNil)
	c.Assert(info, check.HasLen, 1)

	c.Assert(info[0].Size(), check.Equals, int64(3))
	c.Assert(info[0].IsDir(), check.Equals, false)
	c.Assert(info[0].Name(), check.Equals, "foo")
}

func (s *DirTestSuite) TestReadDirFileInfoDirs(c *check.C) {
	files := []string{"qux/baz/foo"}
	for _, name := range files {
		err := util.WriteFile(s.FS, name, []byte{'F', 'O', 'O'}, 0644)
		c.Assert(err, check.IsNil)
	}

	info, err := s.FS.ReadDir("qux")
	c.Assert(err, check.IsNil)
	c.Assert(info, check.HasLen, 1)
	c.Assert(info[0].IsDir(), check.Equals, true)
	c.Assert(info[0].Name(), check.Equals, "baz")

	info, err = s.FS.ReadDir("qux/baz")
	c.Assert(err, check.IsNil)
	c.Assert(info, check.HasLen, 1)
	c.Assert(info[0].Size(), check.Equals, int64(3))
	c.Assert(info[0].IsDir(), check.Equals, false)
	c.Assert(info[0].Name(), check.Equals, "foo")
	c.Assert(info[0].Mode(), check.Not(check.Equals), 0)
}

func (s *DirTestSuite) TestReadDirAfterFileRename(c *check.C) {
	err := util.WriteFile(s.FS, "foo", nil, 0644)
	c.Assert(err, check.IsNil)

	err = s.FS.Rename("foo", "bar")
	c.Assert(err, check.IsNil)

	info, err := s.FS.ReadDir("/")
	c.Assert(err, check.IsNil)
	c.Assert(info, check.HasLen, 1)
	c.Assert(info[0].Name(), check.Equals, "bar")
}

func (s *DirTestSuite) TestRenameToDir(c *check.C) {
	err := util.WriteFile(s.FS, "foo", nil, 0644)
	c.Assert(err, check.IsNil)

	err = s.FS.Rename("foo", "bar/qux")
	c.Assert(err, check.IsNil)

	old, err := s.FS.Stat("foo")
	c.Assert(old, check.IsNil)
	c.Assert(os.IsNotExist(err), check.Equals, true)

	dir, err := s.FS.Stat("bar")
	c.Assert(dir, check.NotNil)
	c.Assert(err, check.IsNil)

	file, err := s.FS.Stat("bar/qux")
	c.Assert(file.Name(), check.Equals, "qux")
	c.Assert(err, check.IsNil)
}

func (s *DirTestSuite) TestRenameDir(c *check.C) {
	err := s.FS.MkdirAll("foo", 0755)
	c.Assert(err, check.IsNil)

	err = util.WriteFile(s.FS, "foo/bar", nil, 0644)
	c.Assert(err, check.IsNil)

	err = s.FS.Rename("foo", "bar")
	c.Assert(err, check.IsNil)

	dirfoo, err := s.FS.Stat("foo")
	c.Assert(dirfoo, check.IsNil)
	c.Assert(os.IsNotExist(err), check.Equals, true)

	dirbar, err := s.FS.Stat("bar")
	c.Assert(err, check.IsNil)
	c.Assert(dirbar, check.NotNil)

	foo, err := s.FS.Stat("foo/bar")
	c.Assert(os.IsNotExist(err), check.Equals, true)
	c.Assert(foo, check.IsNil)

	bar, err := s.FS.Stat("bar/bar")
	c.Assert(err, check.IsNil)
	c.Assert(bar, check.NotNil)

	info, err := s.FS.ReadDir("/")
	c.Assert(err, check.IsNil)
	c.Assert(info, check.HasLen, 1)
	c.Assert(info[0].IsDir(), check.Equals, true)
	c.Assert(info[0].Name(), check.Equals, "bar")
}

func (s *DirTestSuite) TestRemoveNonEmptyDir(c *check.C) {
	err := s.FS.MkdirAll("foo", 0755)
	c.Assert(err, check.IsNil)

	err = util.WriteFile(s.FS, "foo/bar", nil, 0644)
	c.Assert(err, check.IsNil)

	err = s.FS.Remove("foo")
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, fmtDirContainFiles("/foo"))

	//check the dir still exists
	info, err := s.FS.ReadDir("/")
	c.Assert(err, check.IsNil)
	c.Assert(info, check.HasLen, 1)
	c.Assert(info[0].IsDir(), check.Equals, true)
	c.Assert(info[0].Name(), check.Equals, "foo")
}

func (s *DirTestSuite) TestOpenDir(c *check.C) {
	err := s.FS.MkdirAll("foo", 0755)
	c.Assert(err, check.IsNil)

	f, err := s.FS.Open("/foo")
	c.Assert(err, check.ErrorMatches, ".*"+ErrCannotOpenDir.Error()+".*")
	c.Assert(f, check.IsNil)
}
