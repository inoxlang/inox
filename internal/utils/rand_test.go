// source:
// https://github.com/shogo82148/random-float/tree/b8df8274c0fd35cfd34e632acd62e465b3b25c77
// by Ichinose Shogo, MIT Licensed.

package utils

import (
	"math"
	"math/rand"
	"testing"
)

// zeroSource is a rand.Source that always returns 0.
type zeroSource struct{}

func (zeroSource) Seed(int64)   {}
func (zeroSource) Int63() int64 { return 0 }

// zeroSource64 is a rand.Source64 that always returns 0.
type zeroSource64 struct {
	zeroSource
}

func (zeroSource64) Uint64() uint64 { return 0 }

// oneSource is a rand.Source that always returns all 1s.
type oneSource struct{}

func (oneSource) Seed(int64)   {}
func (oneSource) Int63() int64 { return 1<<63 - 1 }

// oneSource64 is a rand.Source64 that always returns all 1s.
type oneSource64 struct {
	oneSource
}

func (oneSource64) Uint64() uint64 { return 1<<64 - 1 }

func TestFloat32(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	r := NewFloatRand(rand.NewSource(42))
	for i := 0; i < 1e8; i++ {
		f := r.Float32()
		if f < 0 || f >= 1 {
			t.Errorf("invalid range: %x", f)
		}
	}
}

func TestFloat32Zero(t *testing.T) {
	var r *FloatRand
	var f float32

	r = NewFloatRand(zeroSource{})
	f = r.Float32()
	if f != 0 {
		t.Errorf("invalid range: %x", f)
	}

	r = NewFloatRand(zeroSource64{})
	f = r.Float32()
	if f != 0 {
		t.Errorf("invalid range: %x", f)
	}
}

func TestFloat32One(t *testing.T) {
	var r *FloatRand
	var f float32

	r = NewFloatRand(oneSource{})
	f = r.Float32()
	if f != math.Nextafter32(1, 0) {
		t.Errorf("invalid range: %x", f)
	}

	r = NewFloatRand(oneSource64{})
	f = r.Float32()
	if f != math.Nextafter32(1, 0) {
		t.Errorf("invalid range: %x", f)
	}
}

func TestFloat64(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	r := NewFloatRand(rand.NewSource(42))
	for i := 0; i < 1e8; i++ {
		f := r.Float64()
		if f < 0 || f >= 1 {
			t.Errorf("invalid range: %x", f)
		}
	}
}

func TestFloat64Zero(t *testing.T) {
	var r *FloatRand
	var f float64

	r = NewFloatRand(zeroSource{})
	f = r.Float64()
	if f != 0 {
		t.Errorf("invalid range: %x", f)
	}

	r = NewFloatRand(zeroSource64{})
	f = r.Float64()
	if f != 0 {
		t.Errorf("invalid range: %x", f)
	}
}

func TestFloat64One(t *testing.T) {
	var r *FloatRand
	var f float64

	r = NewFloatRand(oneSource{})
	f = r.Float64()
	if f != math.Nextafter(1, 0) {
		t.Errorf("invalid range: %x", f)
	}

	r = NewFloatRand(oneSource64{})
	f = r.Float64()
	if f != math.Nextafter(1, 0) {
		t.Errorf("invalid range: %x", f)
	}
}

func BenchmarkFloat32(b *testing.B) {
	r := NewFloatRand(rand.NewSource(42))
	for i := 0; i < b.N; i++ {
		r.Float32()
	}
}

func BenchmarkFloat64(b *testing.B) {
	r := NewFloatRand(rand.NewSource(42))
	for i := 0; i < b.N; i++ {
		r.Float64()
	}
}

func BenchmarkRandFloat32(b *testing.B) {
	r := rand.New(rand.NewSource(42))
	for i := 0; i < b.N; i++ {
		r.Float32()
	}
}

func BenchmarkRandFloat64(b *testing.B) {
	r := rand.New(rand.NewSource(42))
	for i := 0; i < b.N; i++ {
		r.Float64()
	}
}

func BenchmarkSource(b *testing.B) {
	src := rand.NewSource(42)
	for i := 0; i < b.N; i++ {
		src.Int63()
	}
}

func BenchmarkSource64(b *testing.B) {
	src := rand.NewSource(42)
	s64 := src.(rand.Source64)
	for i := 0; i < b.N; i++ {
		s64.Uint64()
	}
}
