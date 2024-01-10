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

func (s structHelper) GetInt(offset int) Int {
	u64 := binary.LittleEndian.Uint64(s.data()[offset : offset+8])
	return Int(u64)
}

func (s structHelper) SetInt(offset int, value Int) {
	binary.LittleEndian.PutUint64(s.data()[offset:offset+8], uint64(value))
}

func (s structHelper) GetFloat(offset int) Float {
	u64 := binary.LittleEndian.Uint64(s.data()[offset : offset+8])
	return Float(math.Float64frombits(u64))
}

func (s structHelper) SetFloat(offset int, value Float) {
	bits := math.Float64bits(float64(value))
	binary.LittleEndian.PutUint64(s.data()[offset:offset+8], bits)
}

func (s structHelper) GetBool(offset int) Bool {
	return s.data()[offset] != 0
}

func (s structHelper) SetBool(offset int, value Bool) {
	if value {
		s.SetTrue(offset)
	} else {
		s.SetFalse(offset)
	}
}

func (s structHelper) SetTrue(offset int) {
	s.data()[offset] = 1
}

func (s structHelper) SetFalse(pos int) {
	s.data()[pos] = 0
}

func (s structHelper) GetStructPointer(offset int) *Struct {
	u64 := binary.LittleEndian.Uint64(s.data()[offset : offset+8])
	return (*Struct)(unsafe.Pointer(uintptr(u64)))
}

func (s structHelper) SetStructPointer(offset int, ptr *Struct) {
	u64 := uint64(uintptr(unsafe.Pointer(ptr)))

	binary.LittleEndian.PutUint64(s.data()[offset:offset+8], u64)
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
