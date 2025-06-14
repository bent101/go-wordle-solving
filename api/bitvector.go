package handler

import "math/bits"

type Bitvec struct {
	Bytes []uint64
	Size  int
	Count int
}

func NewBitvec(size int) *Bitvec {
	numBytes := (size + 63) / 64
	return &Bitvec{
		Bytes: make([]uint64, numBytes),
		Size:  size,
		Count: 0,
	}
}

func (bv *Bitvec) Set(index int) {
	byteIndex := index / 64
	bitIndex := index % 64
	if (bv.Bytes[byteIndex] & (1 << bitIndex)) == 0 {
		bv.Bytes[byteIndex] |= 1 << bitIndex
		bv.Count++
	}
}

func (bv *Bitvec) Get(index int) bool {
	byteIndex := index / 64
	bitIndex := index % 64
	return (bv.Bytes[byteIndex] & (1 << bitIndex)) != 0
}

func (bv *Bitvec) And(other *Bitvec) *Bitvec {
	minLen := min(len(other.Bytes), len(bv.Bytes))

	result := &Bitvec{Bytes: make([]uint64, minLen), Count: 0}
	for i := range minLen {
		result.Bytes[i] = bv.Bytes[i] & other.Bytes[i]
		result.Count += bits.OnesCount64(result.Bytes[i])
	}
	return result
}
