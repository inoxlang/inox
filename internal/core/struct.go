package core

import (
	"encoding/binary"
	"math"
	"unsafe"
)

type Struct byte //converting *Struct to a Value interface does not require allocations.

type fieldRetrievalInfo struct {
	name   string
	offset int
	typ    FieldRetrievalType
}

type FieldRetrievalType int

const (
	GetBoolField FieldRetrievalType = iota
	GetIntField
	GetFloatField
	GetStringField
	GetStructPointerField
)

type structHelper struct {
	ptr  *byte
	size int
}

func structHelperFromPtr(s *Struct, size int) structHelper {
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

func (s structHelper) GetStructPointer(pos int) *Struct {
	u64 := binary.LittleEndian.Uint64(s.data()[pos : pos+8])
	return (*Struct)(unsafe.Pointer(uintptr(u64)))
}

func (s structHelper) SetStructPointer(pos int, ptr *Struct) {
	u64 := uint64(uintptr(unsafe.Pointer(ptr)))

	binary.LittleEndian.PutUint64(s.data()[pos:pos+8], u64)
}

func (s structHelper) GetValue(retrievalInfo fieldRetrievalInfo) Value {
	switch retrievalInfo.typ {
	case GetBoolField:
		return s.GetBool(retrievalInfo.offset)
	case GetIntField:
		return s.GetInt(retrievalInfo.offset)
	case GetFloatField:
		return s.GetFloat(retrievalInfo.offset)
	case GetStructPointerField:
		return s.GetStructPointer(retrievalInfo.offset)
	default:
		panic(ErrUnreachable)
	}
}

func (s structHelper) SetValue(retrievalInfo fieldRetrievalInfo, v Value) {
	switch retrievalInfo.typ {
	case GetBoolField:
		s.SetBool(retrievalInfo.offset, v.(Bool))
	case GetIntField:
		s.SetInt(retrievalInfo.offset, v.(Int))
	case GetFloatField:
		s.SetFloat(retrievalInfo.offset, v.(Float))
	case GetStructPointerField:
		s.SetStructPointer(retrievalInfo.offset, v.(*Struct))
	default:
		panic(ErrUnreachable)
	}
}
