package in_mem_ds

import "errors"

var (
	ErrOutOfBoundsBit32Index = errors.New("out of bounds Bit32Index")
)

// A BitSet32 is a bit set that performs no allocations and can store up to 32 bits.
type BitSet32 uint32

type Bit32Index uint8

func (i Bit32Index) checkInBounds() {
	if i > 31 {
		panic(ErrOutOfBoundsBit32Index)
	}
}

func (s *BitSet32) IsSet(index Bit32Index) bool {
	index.checkInBounds()
	return (*s & (1 << index)) != 0
}

func (s *BitSet32) Set(index Bit32Index) {
	index.checkInBounds()
	*s |= (1 << index)
}

func (s *BitSet32) SetAll() {
	*s = 0xff_ff_ff_ff
}

func (s *BitSet32) Unset(index Bit32Index) {
	index.checkInBounds()
	*s &= (0xff_ff_ff_ff ^ (1 << index))
}

func (s *BitSet32) CountSet() int {
	count := 0
	for i := Bit32Index(0); i < 32; i++ {
		if s.IsSet(i) {
			count++
		}
	}
	return count
}

func (s *BitSet32) ForEachSet(fn func(index Bit32Index) error) error {
	for i := Bit32Index(0); i < 32; i++ {
		if s.IsSet(i) {
			err := fn(i)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
