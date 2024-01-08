package core

import (
	"encoding/binary"
	"math"
	"unsafe"
)

type StructPtr *byte

type structHelper struct {
	ptr  *byte
	size int
}

func structHelperFromPtr(s StructPtr, size int) structHelper {
	return structHelper{
		ptr:  (*byte)(s),
		size: size,
	}
}

func (s structHelper) data() []byte {
	return unsafe.Slice(s.ptr, s.size)
}

func (s structHelper) GetInt(pos int) Int {
	u64 := binary.LittleEndian.Uint64(s.data()[pos : pos+8])
	return Int(u64)
}

func (s structHelper) SetInt(pos int, value Int) {
	binary.LittleEndian.PutUint64(s.data()[pos:pos+8], uint64(value))
}

func (s structHelper) GetFloat(pos int) Float {
	u64 := binary.LittleEndian.Uint64(s.data()[pos : pos+8])
	return Float(math.Float64frombits(u64))
}

func (s structHelper) SetFloat(pos int, value Float) {
	bits := math.Float64bits(float64(value))
	binary.LittleEndian.PutUint64(s.data()[pos:pos+8], bits)
}

func (s structHelper) GetBool(pos int) Bool {
	return s.data()[pos] != 0
}

func (s structHelper) SetBool(pos int, value Bool) {
	if value {
		s.SetTrue(pos)
	} else {
		s.SetFalse(pos)
	}
}

func (s structHelper) SetTrue(pos int) {
	s.data()[pos] = 1
}

func (s structHelper) SetFalse(pos int) {
	s.data()[pos] = 0
}
