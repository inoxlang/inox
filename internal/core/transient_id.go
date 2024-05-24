package core

import (
	"fmt"
	"reflect"
	"unsafe"
)

// A TransientID of an Inox value is an address-based identifier,
// it should not be used to identify the value once the value is no longer accessible (GCed).
// TransientIDs are only obtainable by calling TransientIdOf on Inox values.
type TransientID [2]uintptr

const (
	INT_ADDRESS_LESS_TYPE_ID uintptr = iota + 1
	FLOAT_ADDRESS_LESS_TYPE_ID
	BOOL_ADDRESS_LESS_TYPE_ID
	STR_ADDRESS_LESS_TYPE_ID
	URL_ADDRESS_LESS_TYPE_ID
	HOST_ADDRESS_LESS_TYPE_ID
)

var (
	upperCoreAddressLessTypeId = HOST_ADDRESS_LESS_TYPE_ID
	addressLessTypeRegistry    []reflect.Type
)

// RegisterAddressLessType registers an Inox value type in order to enable the computation of transient ids
// on its instances. Only address-less types such as types of kind reflect.Int, reflect.UInt should be registered.
func RegisterAddressLessType(typ reflect.Type) {
	for _, reflType := range addressLessTypeRegistry {
		if reflType == typ {
			panic(fmt.Errorf("address less type %s is already registred", typ.Name()))
		}
	}
	addressLessTypeRegistry = append(addressLessTypeRegistry, typ)
}

func TransientIdOf(v Value) (result TransientID, hastFastId bool) {
	switch val := v.(type) {
	case Int:
		hastFastId = true
		result = TransientID{INT_ADDRESS_LESS_TYPE_ID, uintptr(val)}
		return
	case Float:
		hastFastId = true
		result = TransientID{FLOAT_ADDRESS_LESS_TYPE_ID, uintptr(val)}
		return
	case Bool:
		hastFastId = true
		if val {
			result = TransientID{BOOL_ADDRESS_LESS_TYPE_ID, 1}
		} else {
			result = TransientID{BOOL_ADDRESS_LESS_TYPE_ID, 0}
		}
		return
	case String:
		hastFastId = true
		header := (*reflect.StringHeader)(unsafe.Pointer(&val))
		result = TransientID{header.Data, uintptr(header.Len)}
		return
	case URL:
		hastFastId = true
		header := (*reflect.StringHeader)(unsafe.Pointer(&val))
		result = TransientID{header.Data, uintptr(header.Len)}
		return
	case Host:
		hastFastId = true
		header := (*reflect.StringHeader)(unsafe.Pointer(&val))
		result = TransientID{header.Data, uintptr(header.Len)}
		return
	case CheckedString:
		hastFastId = true
		header := (*reflect.StringHeader)(unsafe.Pointer(&val.str))
		result = TransientID{header.Data, uintptr(header.Len)}
		return
	default:
		reflectVal := reflect.ValueOf(v)
		reflectType := reflectVal.Type()
		kind := reflectType.Kind()

		switch kind {
		case reflect.Func, reflect.UnsafePointer:
			return //no id
		case reflect.Pointer, reflect.Chan, reflect.Map:
			hastFastId = true
			result = TransientID{reflectVal.Pointer()}
			return
		case reflect.Slice:
			hastFastId = true
			result = TransientID{reflectVal.Pointer(), uintptr(reflectVal.Len())}
			return
		}

		typeId := uintptr(0)
		for i, registeredType := range addressLessTypeRegistry {
			if registeredType != reflectType {
				continue
			}

			typeId = upperCoreAddressLessTypeId + uintptr(i) + 1
		}

		if typeId == 0 {
			return //no id
		}

		if kind >= reflect.Int && kind <= reflect.Int64 {
			hastFastId = true
			result = TransientID{typeId, uintptr(reflectVal.Int())}
			return
		}

		if kind >= reflect.Uint && kind <= reflect.Uintptr {
			hastFastId = true
			result = TransientID{typeId, uintptr(reflectVal.Uint())}
			return
		}

		if kind >= reflect.Float32 && kind <= reflect.Float64 {
			hastFastId = true
			result = TransientID{typeId, uintptr(reflectVal.Float())}
			return
		}
	}
	return
}
