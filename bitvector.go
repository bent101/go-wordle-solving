package main

import "math/bits"

type Bitvector struct {
	Bytes []uint64
	Count int
}

func NewBitvector(size int) *Bitvector {
	numBytes := (size + 63) / 64
	return &Bitvector{Bytes: make([]uint64, numBytes), Count: 0}
}

func (bv *Bitvector) Set(index int) {
	byteIndex := index / 64
	bitIndex := index % 64
	if (bv.Bytes[byteIndex] & (1 << bitIndex)) == 0 {
		bv.Bytes[byteIndex] |= 1 << bitIndex
		bv.Count++
	}
}

func (bv *Bitvector) Get(index int) bool {
	byteIndex := index / 64
	bitIndex := index % 64
	return (bv.Bytes[byteIndex] & (1 << bitIndex)) != 0
}

func (bv *Bitvector) And(other *Bitvector) *Bitvector {
	minLen := min(len(other.Bytes), len(bv.Bytes))

	result := &Bitvector{Bytes: make([]uint64, minLen), Count: 0}
	for i := range minLen {
		result.Bytes[i] = bv.Bytes[i] & other.Bytes[i]
		result.Count += bits.OnesCount64(result.Bytes[i])
	}
	return result
}

func (bv *Bitvector) GetCount() int {
	return bv.Count
}
