//slight modification of https://github.com/go-git/go-billy/blob/master/test/basic.go

package fs_ns

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"gopkg.in/check.v1"
)

var (
	customMode            os.FileMode = 0755
	expectedSymlinkTarget             = "/dir/file"
)

// BasicTestSuite is a convenient test suite to validate any implementation of
// billy.Basic
type BasicTestSuite struct {
	FS billy.Basic
}

func (s *BasicTestSuite) TestCreate(c *check.C) {
	f, err := s.FS.Create("foo")
	c.Assert(err, check.IsNil)
	c.Assert(f.Name(), check.Equals, "foo")
	c.Assert(f.Close(), check.IsNil)
}

func (s *BasicTestSuite) TestCreateDepth(c *check.C) {
	f, err := s.FS.Create("bar/foo")
	c.Assert(err, check.IsNil)
	c.Assert(f.Name(), check.Equals, s.FS.Join("bar", "foo"))
	c.Assert(f.Close(), check.IsNil)
}

func (s *BasicTestSuite) TestCreateDepthAbsolute(c *check.C) {
	f, err := s.FS.Create("/bar/foo")
	c.Assert(err, check.IsNil)

	//original assertion:
	//c.Assert(f.Name(), check.Equals, s.FS.Join("bar", "foo"))
	c.Assert(f.Name(), check.Equals, "/bar/foo")
	c.Assert(f.Close(), check.IsNil)
}

func (s *BasicTestSuite) TestCreateOverwrite(c *check.C) {
	for i := 0; i < 3; i++ {
		f, err := s.FS.Create("foo")
		c.Assert(err, check.IsNil)

		l, err := f.Write([]byte(fmt.Sprintf("foo%d", i)))
		c.Assert(err, check.IsNil)
		c.Assert(l, check.Equals, 4)

		err = f.Close()
		c.Assert(err, check.IsNil)
	}

	f, err := s.FS.Open("foo")
	c.Assert(err, check.IsNil)

	wrote, err := ioutil.ReadAll(f)
	c.Assert(err, check.IsNil)
	c.Assert(string(wrote), check.DeepEquals, "foo2")
	c.Assert(f.Close(), check.IsNil)
}

func (s *BasicTestSuite) TestCreateAndClose(c *check.C) {
	f, err := s.FS.Create("foo")
	c.Assert(err, check.IsNil)

	_, err = f.Write([]byte("foo"))
	c.Assert(err, check.IsNil)
	c.Assert(f.Close(), check.IsNil)

	f, err = s.FS.Open(f.Name())
	c.Assert(err, check.IsNil)

	wrote, err := ioutil.ReadAll(f)
	c.Assert(err, check.IsNil)
	c.Assert(string(wrote), check.DeepEquals, "foo")
	c.Assert(f.Close(), check.IsNil)
}

func (s *BasicTestSuite) TestOpen(c *check.C) {
	f, err := s.FS.Create("foo")
	c.Assert(err, check.IsNil)
	c.Assert(f.Name(), check.Equals, "foo")
	c.Assert(f.Close(), check.IsNil)

	f, err = s.FS.Open("foo")
	c.Assert(err, check.IsNil)
	c.Assert(f.Name(), check.Equals, "foo")
	c.Assert(f.Close(), check.IsNil)
}

func (s *BasicTestSuite) TestOpenNotExists(c *check.C) {
	f, err := s.FS.Open("not-exists")
	c.Assert(err, check.NotNil)
	c.Assert(f, check.IsNil)

	if errors.Is(err, ErrClosedFilesystem) {
		c.Fail()
	}
}

func (s *BasicTestSuite) TestOpenFile(c *check.C) {
	defaultMode := os.FileMode(0666)

	f, err := s.FS.OpenFile("foo1", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, defaultMode)
	c.Assert(err, check.IsNil)
	s.testWriteClose(c, f, "foo1")

	// Truncate if it exists
	f, err = s.FS.OpenFile("foo1", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, defaultMode)
	c.Assert(err, check.IsNil)
	c.Assert(f.Name(), check.Equals, "foo1")
	s.testWriteClose(c, f, "foo1overwritten")

	// Read-only if it exists
	f, err = s.FS.OpenFile("foo1", os.O_RDONLY, defaultMode)
	c.Assert(err, check.IsNil)
	c.Assert(f.Name(), check.Equals, "foo1")
	s.testReadClose(c, f, "foo1overwritten")

	// Create when it does exist
	f, err = s.FS.OpenFile("foo1", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, defaultMode)
	c.Assert(err, check.IsNil)
	c.Assert(f.Name(), check.Equals, "foo1")
	s.testWriteClose(c, f, "bar")

	f, err = s.FS.OpenFile("foo1", os.O_RDONLY, defaultMode)
	c.Assert(err, check.IsNil)
	s.testReadClose(c, f, "bar")
}

func (s *BasicTestSuite) TestOpenFileNoTruncate(c *check.C) {
	defaultMode := os.FileMode(0666)

	// Create when it does not exist
	f, err := s.FS.OpenFile("foo1", os.O_CREATE|os.O_WRONLY, defaultMode)
	c.Assert(err, check.IsNil)
	c.Assert(f.Name(), check.Equals, "foo1")
	s.testWriteClose(c, f, "foo1")

	f, err = s.FS.OpenFile("foo1", os.O_RDONLY, defaultMode)
	c.Assert(err, check.IsNil)
	s.testReadClose(c, f, "foo1")

	// Create when it does exist
	f, err = s.FS.OpenFile("foo1", os.O_CREATE|os.O_WRONLY, defaultMode)
	c.Assert(err, check.IsNil)
	c.Assert(f.Name(), check.Equals, "foo1")
	s.testWriteClose(c, f, "bar")

	f, err = s.FS.OpenFile("foo1", os.O_RDONLY, defaultMode)
	c.Assert(err, check.IsNil)
	s.testReadClose(c, f, "bar1")
}

func (s *BasicTestSuite) TestOpenFileAppend(c *check.C) {
	defaultMode := os.FileMode(0666)

	f, err := s.FS.OpenFile("foo1", os.O_CREATE|os.O_WRONLY|os.O_APPEND, defaultMode)
	c.Assert(err, check.IsNil)
	c.Assert(f.Name(), check.Equals, "foo1")
	s.testWriteClose(c, f, "foo1")

	f, err = s.FS.OpenFile("foo1", os.O_WRONLY|os.O_APPEND, defaultMode)
	c.Assert(err, check.IsNil)
	c.Assert(f.Name(), check.Equals, "foo1")
	s.testWriteClose(c, f, "bar1")

	f, err = s.FS.OpenFile("foo1", os.O_RDONLY, defaultMode)
	c.Assert(err, check.IsNil)
	s.testReadClose(c, f, "foo1bar1")
}

func (s *BasicTestSuite) TestOpenFileReadWrite(c *check.C) {
	defaultMode := os.FileMode(0666)

	f, err := s.FS.OpenFile("foo1", os.O_CREATE|os.O_TRUNC|os.O_RDWR, defaultMode)
	c.Assert(err, check.IsNil)
	c.Assert(f.Name(), check.Equals, "foo1")

	written, err := f.Write([]byte("foobar"))
	c.Assert(written, check.Equals, 6)
	c.Assert(err, check.IsNil)

	_, err = f.Seek(0, os.SEEK_SET)
	c.Assert(err, check.IsNil)

	written, err = f.Write([]byte("qux"))
	c.Assert(written, check.Equals, 3)
	c.Assert(err, check.IsNil)

	_, err = f.Seek(0, os.SEEK_SET)
	c.Assert(err, check.IsNil)

	s.testReadClose(c, f, "quxbar")
}

func (s *BasicTestSuite) TestOpenFileWithModes(c *check.C) {
	f, err := s.FS.OpenFile("foo", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, customMode)
	c.Assert(err, check.IsNil)
	c.Assert(f.Close(), check.IsNil)

	fi, err := s.FS.Stat("foo")
	c.Assert(err, check.IsNil)
	c.Assert(fi.Mode(), check.Equals, os.FileMode(customMode))
}

func (s *BasicTestSuite) testWriteClose(c *check.C, f billy.File, content string) {
	written, err := f.Write([]byte(content))
	c.Assert(written, check.Equals, len(content))
	c.Assert(err, check.IsNil)
	c.Assert(f.Close(), check.IsNil)
}

func (s *BasicTestSuite) testReadClose(c *check.C, f billy.File, content string) {
	read, err := ioutil.ReadAll(f)
	c.Assert(err, check.IsNil)
	c.Assert(string(read), check.Equals, content)
	c.Assert(f.Close(), check.IsNil)
}

func (s *BasicTestSuite) TestFileWrite(c *check.C) {
	f, err := s.FS.Create("foo")
	c.Assert(err, check.IsNil)

	n, err := f.Write([]byte("foo"))
	c.Assert(err, check.IsNil)
	c.Assert(n, check.Equals, 3)

	f.Seek(0, io.SeekStart)
	all, err := ioutil.ReadAll(f)
	c.Assert(err, check.IsNil)
	c.Assert(string(all), check.Equals, "foo")
	c.Assert(f.Close(), check.IsNil)
}

func (s *BasicTestSuite) TestFileWriteClose(c *check.C) {
	f, err := s.FS.Create("foo")
	c.Assert(err, check.IsNil)

	c.Assert(f.Close(), check.IsNil)

	_, err = f.Write([]byte("foo"))
	c.Assert(err, check.NotNil)
}

func (s *BasicTestSuite) TestFileRead(c *check.C) {
	err := util.WriteFile(s.FS, "foo", []byte("foo"), 0644)
	c.Assert(err, check.IsNil)

	f, err := s.FS.Open("foo")
	c.Assert(err, check.IsNil)

	all, err := ioutil.ReadAll(f)
	c.Assert(err, check.IsNil)
	c.Assert(string(all), check.Equals, "foo")
	c.Assert(f.Close(), check.IsNil)
}

func (s *BasicTestSuite) TestFileClosed(c *check.C) {
	err := util.WriteFile(s.FS, "foo", []byte("foo"), 0644)
	c.Assert(err, check.IsNil)

	f, err := s.FS.Open("foo")
	c.Assert(err, check.IsNil)
	c.Assert(f.Close(), check.IsNil)

	_, err = ioutil.ReadAll(f)
	c.Assert(err, check.NotNil)
}

func (s *BasicTestSuite) TestFileNonRead(c *check.C) {
	err := util.WriteFile(s.FS, "foo", []byte("foo"), 0644)
	c.Assert(err, check.IsNil)

	f, err := s.FS.OpenFile("foo", os.O_WRONLY, 0)
	c.Assert(err, check.IsNil)

	_, err = ioutil.ReadAll(f)
	c.Assert(err, check.NotNil)

	c.Assert(f.Close(), check.IsNil)
}

func (s *BasicTestSuite) TestFileSeekstart(c *check.C) {
	s.testFileSeek(c, 10, io.SeekStart)
}

func (s *BasicTestSuite) TestFileSeekCurrent(c *check.C) {
	s.testFileSeek(c, 5, io.SeekCurrent)
}

func (s *BasicTestSuite) TestFileSeekEnd(c *check.C) {
	s.testFileSeek(c, -26, io.SeekEnd)
}

func (s *BasicTestSuite) testFileSeek(c *check.C, offset int64, whence int) {
	err := util.WriteFile(s.FS, "foo", []byte("0123456789abcdefghijklmnopqrstuvwxyz"), 0644)
	c.Assert(err, check.IsNil)

	f, err := s.FS.Open("foo")
	c.Assert(err, check.IsNil)

	some := make([]byte, 5)
	_, err = f.Read(some)
	c.Assert(err, check.IsNil)
	c.Assert(string(some), check.Equals, "01234")

	p, err := f.Seek(offset, whence)
	c.Assert(err, check.IsNil)
	c.Assert(int(p), check.Equals, 10)

	all, err := ioutil.ReadAll(f)
	c.Assert(err, check.IsNil)
	c.Assert(all, check.HasLen, 26)
	c.Assert(string(all), check.Equals, "abcdefghijklmnopqrstuvwxyz")
	c.Assert(f.Close(), check.IsNil)
}

func (s *BasicTestSuite) TestSeekToEndAndWrite(c *check.C) {
	defaultMode := os.FileMode(0666)

	f, err := s.FS.OpenFile("foo1", os.O_CREATE|os.O_TRUNC|os.O_RDWR, defaultMode)
	c.Assert(err, check.IsNil)
	c.Assert(f.Name(), check.Equals, "foo1")

	_, err = f.Seek(10, io.SeekEnd)
	c.Assert(err, check.IsNil)

	n, err := f.Write([]byte(`TEST`))
	c.Assert(err, check.IsNil)
	c.Assert(n, check.Equals, 4)

	_, err = f.Seek(0, io.SeekStart)
	c.Assert(err, check.IsNil)

	s.testReadClose(c, f, "\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00TEST")
}

func (s *BasicTestSuite) TestFileSeekClosed(c *check.C) {
	err := util.WriteFile(s.FS, "foo", []byte("foo"), 0644)
	c.Assert(err, check.IsNil)

	f, err := s.FS.Open("foo")
	c.Assert(err, check.IsNil)
	c.Assert(f.Close(), check.IsNil)

	_, err = f.Seek(0, 0)
	c.Assert(err, check.NotNil)
}

func (s *BasicTestSuite) TestFileCloseTwice(c *check.C) {
	f, err := s.FS.Create("foo")
	c.Assert(err, check.IsNil)

	c.Assert(f.Close(), check.IsNil)
	c.Assert(f.Close(), check.NotNil)
}

func (s *BasicTestSuite) TestStat(c *check.C) {
	util.WriteFile(s.FS, "foo/bar", []byte("foo"), customMode)

	fi, err := s.FS.Stat("foo/bar")
	c.Assert(err, check.IsNil)
	c.Assert(fi.Name(), check.Equals, "bar")
	c.Assert(fi.Size(), check.Equals, int64(3))
	c.Assert(fi.Mode(), check.Equals, customMode)
	c.Assert(fi.ModTime().IsZero(), check.Equals, false)
	c.Assert(fi.IsDir(), check.Equals, false)
}

func (s *BasicTestSuite) TestStatNonExistent(c *check.C) {
	fi, err := s.FS.Stat("non-existent")
	comment := check.Commentf("error: %s", err)
	c.Assert(os.IsNotExist(err), check.Equals, true, comment)
	c.Assert(fi, check.IsNil)
}

func (s *BasicTestSuite) TestRename(c *check.C) {
	err := util.WriteFile(s.FS, "foo", nil, 0644)
	c.Assert(err, check.IsNil)

	err = s.FS.Rename("foo", "bar")
	c.Assert(err, check.IsNil)

	foo, err := s.FS.Stat("foo")
	c.Assert(foo, check.IsNil)
	c.Assert(os.IsNotExist(err), check.Equals, true)

	bar, err := s.FS.Stat("bar")
	c.Assert(err, check.IsNil)
	c.Assert(bar, check.NotNil)
}

func (s *BasicTestSuite) TestOpenAndWrite(c *check.C) {
	err := util.WriteFile(s.FS, "foo", nil, 0644)
	c.Assert(err, check.IsNil)

	foo, err := s.FS.Open("foo")
	c.Assert(foo, check.NotNil)
	c.Assert(err, check.IsNil)

	n, err := foo.Write([]byte("foo"))
	c.Assert(err, check.NotNil)
	c.Assert(n, check.Equals, 0)

	c.Assert(foo.Close(), check.IsNil)
}

func (s *BasicTestSuite) TestOpenAndStat(c *check.C) {
	err := util.WriteFile(s.FS, "foo", []byte("foo"), 0644)
	c.Assert(err, check.IsNil)

	foo, err := s.FS.Open("foo")
	c.Assert(foo, check.NotNil)
	c.Assert(foo.Name(), check.Equals, "foo")
	c.Assert(err, check.IsNil)
	c.Assert(foo.Close(), check.IsNil)

	stat, err := s.FS.Stat("foo")
	c.Assert(stat, check.NotNil)
	c.Assert(err, check.IsNil)
	c.Assert(stat.Name(), check.Equals, "foo")
	c.Assert(stat.Size(), check.Equals, int64(3))
}

func (s *BasicTestSuite) TestRemove(c *check.C) {
	f, err := s.FS.Create("foo")
	c.Assert(err, check.IsNil)
	c.Assert(f.Close(), check.IsNil)

	err = s.FS.Remove("foo")
	c.Assert(err, check.IsNil)
}

func (s *BasicTestSuite) TestRemoveNonExisting(c *check.C) {
	err := s.FS.Remove("NON-EXISTING")
	c.Assert(err, check.NotNil)
	c.Assert(os.IsNotExist(err), check.Equals, true)
}

func (s *BasicTestSuite) TestRemoveNotEmptyDir(c *check.C) {
	err := util.WriteFile(s.FS, "foo", nil, 0644)
	c.Assert(err, check.IsNil)

	err = s.FS.Remove("no-exists")
	c.Assert(err, check.NotNil)
}

func (s *BasicTestSuite) TestJoin(c *check.C) {
	c.Assert(s.FS.Join("foo", "bar"), check.Equals, fmt.Sprintf("foo%cbar", filepath.Separator))
}

func (s *BasicTestSuite) TestReadAtOnReadWrite(c *check.C) {
	f, err := s.FS.Create("foo")
	c.Assert(err, check.IsNil)
	_, err = f.Write([]byte("abcdefg"))
	c.Assert(err, check.IsNil)

	rf, ok := f.(io.ReaderAt)
	c.Assert(ok, check.Equals, true)

	b := make([]byte, 3)
	n, err := rf.ReadAt(b, 2)
	c.Assert(err, check.IsNil)
	c.Assert(n, check.Equals, 3)
	c.Assert(string(b), check.Equals, "cde")
	c.Assert(f.Close(), check.IsNil)
}

func (s *BasicTestSuite) TestReadAtOnReadOnly(c *check.C) {
	err := util.WriteFile(s.FS, "foo", []byte("abcdefg"), 0644)
	c.Assert(err, check.IsNil)

	f, err := s.FS.Open("foo")
	c.Assert(err, check.IsNil)

	rf, ok := f.(io.ReaderAt)
	c.Assert(ok, check.Equals, true)

	b := make([]byte, 3)
	n, err := rf.ReadAt(b, 2)
	c.Assert(err, check.IsNil)
	c.Assert(n, check.Equals, 3)
	c.Assert(string(b), check.Equals, "cde")
	c.Assert(f.Close(), check.IsNil)
}

func (s *BasicTestSuite) TestReadAtEOF(c *check.C) {
	err := util.WriteFile(s.FS, "foo", []byte("TEST"), 0644)
	c.Assert(err, check.IsNil)

	f, err := s.FS.Open("foo")
	c.Assert(err, check.IsNil)

	b := make([]byte, 5)
	n, err := f.ReadAt(b, 0)
	c.Assert(err, check.Equals, io.EOF)
	c.Assert(n, check.Equals, 4)
	c.Assert(string(b), check.Equals, "TEST\x00")

	err = f.Close()
	c.Assert(err, check.IsNil)
}

func (s *BasicTestSuite) TestReadAtOffset(c *check.C) {
	err := util.WriteFile(s.FS, "foo", []byte("TEST"), 0644)
	c.Assert(err, check.IsNil)

	f, err := s.FS.Open("foo")
	c.Assert(err, check.IsNil)

	rf, ok := f.(io.ReaderAt)
	c.Assert(ok, check.Equals, true)

	o, err := f.Seek(0, io.SeekCurrent)
	c.Assert(err, check.IsNil)
	c.Assert(o, check.Equals, int64(0))

	b := make([]byte, 4)
	n, err := rf.ReadAt(b, 0)
	c.Assert(err, check.IsNil)
	c.Assert(n, check.Equals, 4)
	c.Assert(string(b), check.Equals, "TEST")

	o, err = f.Seek(0, io.SeekCurrent)
	c.Assert(err, check.IsNil)
	c.Assert(o, check.Equals, int64(0))

	err = f.Close()
	c.Assert(err, check.IsNil)
}

func (s *BasicTestSuite) TestReadWriteLargeFile(c *check.C) {
	f, err := s.FS.Create("foo")
	c.Assert(err, check.IsNil)

	size := 1 << 20

	n, err := f.Write(bytes.Repeat([]byte("F"), size))
	c.Assert(err, check.IsNil)
	c.Assert(n, check.Equals, size)

	c.Assert(f.Close(), check.IsNil)

	f, err = s.FS.Open("foo")
	c.Assert(err, check.IsNil)
	b, err := ioutil.ReadAll(f)
	c.Assert(err, check.IsNil)
	c.Assert(len(b), check.Equals, size)
	c.Assert(f.Close(), check.IsNil)
}

func (s *BasicTestSuite) TestWriteFile(c *check.C) {
	err := util.WriteFile(s.FS, "foo", []byte("bar"), 0777)
	c.Assert(err, check.IsNil)

	f, err := s.FS.Open("foo")
	c.Assert(err, check.IsNil)

	wrote, err := ioutil.ReadAll(f)
	c.Assert(err, check.IsNil)
	c.Assert(string(wrote), check.DeepEquals, "bar")

	c.Assert(f.Close(), check.IsNil)
}

func (s *BasicTestSuite) TestTruncate(c *check.C) {
	f, err := s.FS.Create("foo")
	c.Assert(err, check.IsNil)

	for _, sz := range []int64{4, 7, 2, 30, 0, 1} {
		err = f.Truncate(sz)
		c.Assert(err, check.IsNil)

		bs, err := ioutil.ReadAll(f)
		c.Assert(err, check.IsNil)
		c.Assert(len(bs), check.Equals, int(sz))

		_, err = f.Seek(0, io.SeekStart)
		c.Assert(err, check.IsNil)
	}

	c.Assert(f.Close(), check.IsNil)
}
