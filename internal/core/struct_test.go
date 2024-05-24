package core

import (
	"bytes"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStructHelper(t *testing.T) {

	EightZeroBytes := bytes.Repeat([]byte{0}, 8)

	t.Run("set/get int", func(t *testing.T) {
		memory := make([]byte, 10)
		structPtr := (*Struct)(&memory[1])
		helper := structHelperFromPtr(structPtr, 8 /*size*/)

		integers := []Int{math.MinInt64, math.MinInt64 + 1, -1, 0, 1, math.MaxInt64 - 1, math.MaxInt64}

		for _, integer := range integers {
			helper.SetInt(0, integer)

			assert.Equal(t, integer, helper.GetInt(0))
			assert.Zero(t, memory[0])
			assert.Zero(t, memory[len(memory)-1])

			if integer == 0 {
				assert.Equal(t, EightZeroBytes, memory[1:1+8])
			} else {
				assert.NotEqual(t, EightZeroBytes, memory[1:1+8])
			}
		}
	})

	t.Run("set/get float", func(t *testing.T) {
		memory := make([]byte, 10)
		structPtr := (*Struct)(&memory[1])
		helper := structHelperFromPtr(structPtr, 8 /*size*/)

		floats := []Float{-math.MaxFloat64, -1, 0, 1, math.MaxFloat64}

		for _, float := range floats {
			helper.SetFloat(0, float)

			assert.Equal(t, float, helper.GetFloat(0))
			assert.Zero(t, memory[0])
			assert.Zero(t, memory[len(memory)-1])

			if float == 0 {
				assert.Equal(t, EightZeroBytes, memory[1:1+8])
			} else {
				assert.NotEqual(t, EightZeroBytes, memory[1:1+8])
			}
		}
	})

	t.Run("set/get bool", func(t *testing.T) {
		memory := make([]byte, 10)
		structPtr := (*Struct)(&memory[1])
		helper := structHelperFromPtr(structPtr, 8 /*size*/)

		helper.SetTrue(0)
		assert.True(t, bool(helper.GetBool(0)))
		assert.Zero(t, memory[0])
		assert.Zero(t, memory[2])

		helper.SetFalse(0)
		assert.False(t, bool(helper.GetBool(0)))
		assert.Zero(t, memory[0])
		assert.Zero(t, memory[2])

		helper.SetBool(0, true)
		assert.True(t, bool(helper.GetBool(0)))
		assert.Zero(t, memory[0])
		assert.Zero(t, memory[2])

		helper.SetBool(0, false)
		assert.False(t, bool(helper.GetBool(0)))
		assert.Zero(t, memory[0])
		assert.Zero(t, memory[2])
	})

	t.Run("set/get struct pointer", func(t *testing.T) {
		memory := make([]byte, 10)
		structPtr := (*Struct)(&memory[1])

		helper := structHelperFromPtr(structPtr, 8 /*size*/)

		helper.SetStructPointer(0, structPtr)

		assert.Equal(t, structPtr, helper.GetStructPointer(0))
		assert.Zero(t, memory[0])
		assert.Zero(t, memory[len(memory)-1])

		assert.NotEqual(t, EightZeroBytes, memory[1:1+8])
	})

}

func BenchmarkStructHelperSetInt(b *testing.B) {
	memory := make([]byte, 8)
	structPtr := (*Struct)(&memory[0])
	helper := structHelperFromPtr(structPtr, 8 /*size*/)

	pos := 1
	if b.N >= 0 { //prevent optimization
		//always executed
		pos = 0
	}

	for i := 0; i < b.N/8; i++ {
		helper.SetInt(pos, Int(i))
		helper.SetInt(pos, Int(i))
		helper.SetInt(pos, Int(i))
		helper.SetInt(pos, Int(i))

		helper.SetInt(pos, Int(i))
		helper.SetInt(pos, Int(i))
		helper.SetInt(pos, Int(i))
		helper.SetInt(pos, Int(i))
	}
}

func BenchmarkStructHelperSetInt2(b *testing.B) {
	memory := make([]byte, 8)
	structPtr := (*Struct)(&memory[0])
	helper := structHelperFromPtr(structPtr, 8 /*size*/)

	pos := 1
	if b.N >= 0 { //prevent optimization
		//always executed
		pos = 0
	}

	for i := 0; i < b.N/8; i++ {
		helper.SetInt(pos, Int(i))
		helper.SetInt(pos, Int(i))
		helper.SetInt(pos, Int(i))
		helper.SetInt(pos, Int(i))

		helper.SetInt(pos, Int(i))
		helper.SetInt(pos, Int(i))
		helper.SetInt(pos, Int(i))
		helper.SetInt(pos, Int(i))
	}
}

func BenchmarkStructHelperGetInt(b *testing.B) {
	memory := make([]byte, 8)
	structPtr := (*Struct)(&memory[0])
	helper := structHelperFromPtr(structPtr, 8 /*size*/)

	pos := 1
	if b.N >= 0 { //prevent optimization
		pos = 0
	}

	for i := 0; i < b.N/8; i++ {
		_ = helper.GetInt(pos)
		_ = helper.GetInt(pos)
		_ = helper.GetInt(pos)
		_ = helper.GetInt(pos)

		_ = helper.GetInt(pos)
		_ = helper.GetInt(pos)
		_ = helper.GetInt(pos)
		_ = helper.GetInt(pos)
	}
}

func BenchmarkStructHelperSetFloat(b *testing.B) {
	memory := make([]byte, 8)
	structPtr := (*Struct)(&memory[0])
	helper := structHelperFromPtr(structPtr, 8 /*size*/)

	pos := 1
	if b.N >= 0 { //prevent optimization
		//always executed
		pos = 0
	}

	for i := 0; i < b.N/8; i++ {
		float := Float(i)

		helper.SetFloat(pos, float)
		helper.SetFloat(pos, float)
		helper.SetFloat(pos, float)
		helper.SetFloat(pos, float)

		helper.SetFloat(pos, float)
		helper.SetFloat(pos, float)
		helper.SetFloat(pos, float)
		helper.SetFloat(pos, float)
	}
}

func BenchmarkStructHelperGetFloat(b *testing.B) {
	memory := make([]byte, 8)
	structPtr := (*Struct)(&memory[0])
	helper := structHelperFromPtr(structPtr, 8 /*size*/)

	pos := 1
	if b.N >= 0 { //prevent optimization
		pos = 0
	}

	for i := 0; i < b.N/8; i++ {
		_ = helper.GetFloat(pos)
		_ = helper.GetFloat(pos)
		_ = helper.GetFloat(pos)
		_ = helper.GetFloat(pos)

		_ = helper.GetFloat(pos)
		_ = helper.GetFloat(pos)
		_ = helper.GetFloat(pos)
		_ = helper.GetFloat(pos)
	}
}

func BenchmarkStructHelperSetBool(b *testing.B) {
	memory := make([]byte, 8)
	structPtr := (*Struct)(&memory[0])
	helper := structHelperFromPtr(structPtr, 8 /*size*/)

	pos := 1
	if b.N >= 0 { //prevent optimization
		//always executed
		pos = 0
	}

	for i := 0; i < b.N/8; i++ {
		boolean := Bool(i == 2)

		helper.SetBool(pos, boolean)
		helper.SetBool(pos, boolean)
		helper.SetBool(pos, boolean)
		helper.SetBool(pos, boolean)

		helper.SetBool(pos, boolean)
		helper.SetBool(pos, boolean)
		helper.SetBool(pos, boolean)
		helper.SetBool(pos, boolean)
	}
}

func BenchmarkStructHelperGetBool(b *testing.B) {
	memory := make([]byte, 8)
	structPtr := (*Struct)(&memory[0])
	helper := structHelperFromPtr(structPtr, 8 /*size*/)

	pos := 1
	if b.N >= 0 { //prevent optimization
		pos = 0
	}

	for i := 0; i < b.N/8; i++ {
		_ = helper.GetBool(pos)
		_ = helper.GetBool(pos)
		_ = helper.GetBool(pos)
		_ = helper.GetBool(pos)

		_ = helper.GetBool(pos)
		_ = helper.GetBool(pos)
		_ = helper.GetBool(pos)
		_ = helper.GetBool(pos)
	}
}
