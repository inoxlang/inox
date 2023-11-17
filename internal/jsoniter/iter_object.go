package jsoniter

import (
	"fmt"
)

// ReadObject read one field from object.
// If object ended, returns empty string.
// Otherwise, returns the field name.
func (iter *Iterator) ReadObject() (ret string) {
	c := iter.nextToken()
	switch c {
	case 'n':
		iter.skipThreeBytes('u', 'l', 'l')
		return "" // null
	case '{':
		c = iter.nextToken()
		if c == '"' {
			iter.unreadByte()
			field := iter.ReadString()
			c = iter.nextToken()
			if c != ':' {
				iter.ReportError("ReadObject", "expect : after object field, but found "+string([]byte{c}))
			}
			return field
		}
		if c == '}' {
			return "" // end of object
		}
		iter.ReportError("ReadObject", `expect " after {, but found `+string([]byte{c}))
		return
	case ',':
		field := iter.ReadString()
		c = iter.nextToken()
		if c != ':' {
			iter.ReportError("ReadObject", "expect : after object field, but found "+string([]byte{c}))
		}
		return field
	case '}':
		return "" // end of object
	default:
		iter.ReportError("ReadObject", fmt.Sprintf(`expect { or , or } or n, but found %s`, string([]byte{c})))
		return
	}
}

// ReadObjectCB reads an object and calls callback each time it reads a key.
func (iter *Iterator) ReadObjectCB(callback func(*Iterator, string) bool) bool {
	c := iter.nextToken()
	var field string
	if c == '{' {
		if !iter.incrementDepth() {
			return false
		}
		c = iter.nextToken()
		if c == '"' {
			iter.unreadByte()
			field = iter.ReadString()
			c = iter.nextToken()
			if c != ':' {
				iter.ReportError("ReadObjectCB", "expect : after object field, but found "+string([]byte{c}))
			}
			if !callback(iter, field) {
				iter.decrementDepth()
				return false
			}
			c = iter.nextToken()
			for c == ',' {
				field = iter.ReadString()
				c = iter.nextToken()
				if c != ':' {
					iter.ReportError("ReadObjectCB", "expect : after object field, but found "+string([]byte{c}))
				}
				if !callback(iter, field) {
					iter.decrementDepth()
					return false
				}
				c = iter.nextToken()
			}
			if c != '}' {
				iter.ReportError("ReadObjectCB", `object not ended with }`)
				iter.decrementDepth()
				return false
			}
			return iter.decrementDepth()
		}
		if c == '}' {
			return iter.decrementDepth()
		}
		iter.ReportError("ReadObjectCB", `expect " after {, but found `+string([]byte{c}))
		iter.decrementDepth()
		return false
	}
	if c == 'n' {
		iter.skipThreeBytes('u', 'l', 'l')
		return true // null
	}
	iter.ReportError("ReadObjectCB", `expect { or n, but found `+string([]byte{c}))
	return false
}

// ReadObjectAvoidAllocationsCB reads an object and calls callbackFn each time it reads a key,
// keys that don't require decoding are passed to the callbackFn without allocation.
// If the callback function is called with the allocated argument to true that means the key has been decoded,
// the key argument can be stored and modified. Otherwise the key should only be accessed during the duration
// of the current callback.
func (iter *Iterator) ReadObjectMinimizeAllocationsCB(callbackFn func(it *Iterator, key []byte, allocated bool) bool) bool {
	c := iter.nextToken()
	var field []byte
	var fieldAllocated bool

	if c == '{' {
		if !iter.incrementDepth() {
			return false
		}
		c = iter.nextToken()
		if c == '"' {
			iter.unreadByte()
			field, fieldAllocated = iter.ReadStringAsBytes()
			c = iter.nextToken()
			if c != ':' {
				iter.ReportError("ReadObjectMinimizeAllocationsCB", "expect : after object field, but found "+string([]byte{c}))
			}
			if !callbackFn(iter, field, fieldAllocated) {
				iter.decrementDepth()
				return false
			}
			c = iter.nextToken()
			for c == ',' {
				field, fieldAllocated = iter.ReadStringAsBytes()
				c = iter.nextToken()
				if c != ':' {
					iter.ReportError("ReadObjectMinimizeAllocationsCB", "expect : after object field, but found "+string([]byte{c}))
				}
				if !callbackFn(iter, field, fieldAllocated) {
					iter.decrementDepth()
					return false
				}
				c = iter.nextToken()
			}
			if c != '}' {
				iter.ReportError("ReadObjectMinimizeAllocationsCB", `object not ended with }`)
				iter.decrementDepth()
				return false
			}
			return iter.decrementDepth()
		}
		if c == '}' {
			return iter.decrementDepth()
		}
		iter.ReportError("ReadObjectMinimizeAllocationsCB", `expect " after {, but found `+string([]byte{c}))
		iter.decrementDepth()
		return false
	}
	if c == 'n' {
		iter.skipThreeBytes('u', 'l', 'l')
		return true // null
	}
	iter.ReportError("ReadObjectMinimizeAllocationsCB", `expect { or n, but found `+string([]byte{c}))
	return false
}

// ReadMapCB read map with callback, the key can be any string
func (iter *Iterator) ReadMapCB(callback func(*Iterator, string) bool) bool {
	c := iter.nextToken()
	if c == '{' {
		if !iter.incrementDepth() {
			return false
		}
		c = iter.nextToken()
		if c == '"' {
			iter.unreadByte()
			field := iter.ReadString()
			if iter.nextToken() != ':' {
				iter.ReportError("ReadMapCB", "expect : after object field, but found "+string([]byte{c}))
				iter.decrementDepth()
				return false
			}
			if !callback(iter, field) {
				iter.decrementDepth()
				return false
			}
			c = iter.nextToken()
			for c == ',' {
				field = iter.ReadString()
				if iter.nextToken() != ':' {
					iter.ReportError("ReadMapCB", "expect : after object field, but found "+string([]byte{c}))
					iter.decrementDepth()
					return false
				}
				if !callback(iter, field) {
					iter.decrementDepth()
					return false
				}
				c = iter.nextToken()
			}
			if c != '}' {
				iter.ReportError("ReadMapCB", `object not ended with }`)
				iter.decrementDepth()
				return false
			}
			return iter.decrementDepth()
		}
		if c == '}' {
			return iter.decrementDepth()
		}
		iter.ReportError("ReadMapCB", `expect " after {, but found `+string([]byte{c}))
		iter.decrementDepth()
		return false
	}
	if c == 'n' {
		iter.skipThreeBytes('u', 'l', 'l')
		return true // null
	}
	iter.ReportError("ReadMapCB", `expect { or n, but found `+string([]byte{c}))
	return false
}

func (iter *Iterator) readObjectStart() bool {
	c := iter.nextToken()
	if c == '{' {
		c = iter.nextToken()
		if c == '}' {
			return false
		}
		iter.unreadByte()
		return true
	} else if c == 'n' {
		iter.skipThreeBytes('u', 'l', 'l')
		return false
	}
	iter.ReportError("readObjectStart", "expect { or n, but found "+string([]byte{c}))
	return false
}

func (iter *Iterator) readObjectFieldAsBytes() (ret []byte) {
	str := iter.ReadStringAsSlice()
	if iter.skipWhitespacesWithoutLoadMore() {
		if ret == nil {
			ret = make([]byte, len(str))
			copy(ret, str)
		}
		if !iter.loadMore() {
			return
		}
	}
	if iter.buf[iter.head] != ':' {
		iter.ReportError("readObjectFieldAsBytes", "expect : after object field, but found "+string([]byte{iter.buf[iter.head]}))
		return
	}
	iter.head++
	if iter.skipWhitespacesWithoutLoadMore() {
		if ret == nil {
			ret = make([]byte, len(str))
			copy(ret, str)
		}
		if !iter.loadMore() {
			return
		}
	}
	if ret == nil {
		return str
	}
	return ret
}
