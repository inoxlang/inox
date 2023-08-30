package fs_ns

import "os"

func IsCreate(flag int) bool {
	return flag&os.O_CREATE != 0
}

func IsExclusive(flag int) bool {
	return flag&os.O_EXCL != 0
}

func IsAppend(flag int) bool {
	return flag&os.O_APPEND != 0
}

func IsTruncate(flag int) bool {
	return flag&os.O_TRUNC != 0
}

func IsReadAndWrite(flag int) bool {
	return flag&os.O_RDWR != 0
}

func isReadOnly(flag int) bool {
	return flag == os.O_RDONLY
}

func isWriteOnly(flag int) bool {
	return flag&os.O_WRONLY != 0
}

func isSymlink(m os.FileMode) bool {
	return m&os.ModeSymlink != 0
}
