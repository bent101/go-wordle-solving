package main

import (
	"golang.org/x/exp/slices"
)

type Duplicate struct {
	char byte

	greenIdxs []int

	// 0-4 other than greens, yellows, and grays, and other greens
	possibleIdxs []int

	// yellows and grays
	impossibleIdxs []int

	// count is exact if there are grays; otherwise, it's a minimum
	count   int
	hasGray bool
}

type Green struct {
	char byte
	idx  int
}

type Yellow struct {
	impossibleIdx int

	// 0-4 other than greens and impossibleIdx
	possibleIdxs []int
}

type Hint struct {
	sequence [5]int // 0, 1, 2 = gray, yellow, green
	rank     int    // sequence as a base 3 number (for hashing)

	duplicates []Duplicate
	greens     []Green
	yellows    []Yellow
	grays      [26]bool
}

func New(guess, answer string) *Hint {
	hint := Hint{}

	// greens
	for i := 0; i < 5; i++ {
		if guess[i] == answer[i] {
			hint.greens = append(hint.greens, Green{char: guess[i], idx: i})
		}
		hint.sequence[i] = 2
	}

	// get list of non-green indeces and un-hinted-at chars
	unHintedAtChars := make([]byte, 0, 5)
	for i := 0; i < 5; i++ {
		if hint.sequence[i] == 0 {
			unHintedAtChars = append(unHintedAtChars, answer[i])
		}
	}

	for i := 0; i < 5; i++ {
		if hint.sequence[i] == 2 {
			continue
		}
		c := guess[i]
		if slices.Contains(unHintedAtChars, c) {
			hint.sequence[i] = 1
			j := slices.Index(unHintedAtChars, c)
			unHintedAtChars = slices.Remove(unHintedAtChars, j, j+1)
		}
	}

	return &hint
}
