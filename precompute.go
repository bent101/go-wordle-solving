package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
)

// LetterInfo represents what we know about a letter's constraints in the target word
type LetterInfo struct {
	MustBeInPositions []int `json:"must_be_in_positions"`
	CantBeInPositions []int `json:"cant_be_in_positions"`
	Frequency         int   `json:"frequency"`
	FrequencyIsExact  bool  `json:"frequency_is_exact"`
}

func (l LetterInfo) Canonical() string {
	mustBe := make([]int, len(l.MustBeInPositions))
	copy(mustBe, l.MustBeInPositions)
	sort.Ints(mustBe)

	cantBe := make([]int, len(l.CantBeInPositions))
	copy(cantBe, l.CantBeInPositions)
	sort.Ints(cantBe)

	return fmt.Sprintf("must:%v,cant:%v,freq:%d,exact:%t",
		mustBe, cantBe, l.Frequency, l.FrequencyIsExact)
}

func (l LetterInfo) Hash() string {
	canonical := l.Canonical()
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:8])
}

func (l LetterInfo) IsValid() bool {
	if l.Frequency == 0 {
		return len(l.MustBeInPositions) == 0 && l.FrequencyIsExact
	}

	if len(l.MustBeInPositions) > l.Frequency {
		return false
	}

	for _, mustPos := range l.MustBeInPositions {
		if slices.Contains(l.CantBeInPositions, mustPos) {
			return false
		}
	}

	// All positions are guaranteed to be 1-5 for 5-letter words
	return true
}

func (l LetterInfo) InTarget() bool {
	return l.Frequency > 0
}

func (l LetterInfo) CouldBeInPosition(pos int) bool {
	// Check if this position is ruled out
	return !slices.Contains(l.CantBeInPositions, pos)
}

func (l LetterInfo) PossiblePositions() []int {
	var possible []int
	for pos := 0; pos < 5; pos++ {
		if l.CouldBeInPosition(pos) {
			possible = append(possible, pos)
		}
	}
	return possible
}

func GenerateAllLetterInfos() []LetterInfo {
	var allInfos []LetterInfo

	for freq := 0; freq <= 3; freq++ {
		for exactness := range 2 {
			isExact := exactness == 1

			for mustBits := range 1 << 5 {
				var mustPositions []int
				for pos := range 5 {
					if (mustBits & (1 << pos)) != 0 {
						mustPositions = append(mustPositions, pos)
					}
				}

				if len(mustPositions) > freq {
					continue
				}

				for cantBits := range 1 << 5 {
					var cantPositions []int
					hasOverlap := false

					for pos := range 5 {
						if (cantBits & (1 << pos)) != 0 {
							if slices.Contains(mustPositions, pos) {
								hasOverlap = true
								break
							}
							cantPositions = append(cantPositions, pos)
						}
					}

					if hasOverlap {
						continue
					}

					info := LetterInfo{
						MustBeInPositions: mustPositions,
						CantBeInPositions: cantPositions,
						Frequency:         freq,
						FrequencyIsExact:  isExact,
					}

					if info.IsValid() {
						allInfos = append(allInfos, info)
					}
				}
			}
		}
	}

	return allInfos
}

type HintDatabase struct {
	LetterInfos []LetterInfo   `json:"letter_infos"`
	HashToIndex map[string]int `json:"hash_to_index"`
	Metadata    map[string]any `json:"metadata"`
}

func PrecomputeMain() {
	fmt.Println("=== WORDLE HINT PRECOMPUTATION ===")

	allInfos := GenerateAllLetterInfos()
	fmt.Printf("Generated %d valid LetterInfo objects\n", len(allInfos))

	hashToIndex := make(map[string]int)
	for i, info := range allInfos {
		hash := info.Hash()
		hashToIndex[hash] = i
	}

	if len(hashToIndex) != len(allInfos) {
		fmt.Printf("Warning: Hash collisions detected!\n")
	} else {
		fmt.Printf("✓ No hash collisions - %d unique hashes\n", len(hashToIndex))
	}

	db := HintDatabase{
		LetterInfos: allInfos,
		HashToIndex: hashToIndex,
		Metadata: map[string]any{
			"version":       "1.0",
			"description":   "Precomputed Wordle letter constraints",
			"total_hints":   len(allInfos),
			"word_length":   5,
			"max_frequency": 3,
		},
	}

	jsonData, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		return
	}

	filename := "wordle_hints.json"
	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		fmt.Printf("Error writing file: %v\n", err)
		return
	}

	fmt.Printf("✓ Successfully wrote %d hints to %s\n", len(allInfos), filename)
	fmt.Printf("File size: %.2f KB\n", float64(len(jsonData))/1024)

	fmt.Println("\n=== STATISTICS ===")
	freqCounts := make(map[int]int)
	exactCounts := make(map[bool]int)

	for _, info := range allInfos {
		freqCounts[info.Frequency]++
		exactCounts[info.FrequencyIsExact]++
	}

	fmt.Println("Frequency distribution:")
	for freq := 0; freq <= 3; freq++ {
		fmt.Printf("  Frequency %d: %d hints\n", freq, freqCounts[freq])
	}

	fmt.Println("Exactness distribution:")
	fmt.Printf("  Exact frequency: %d hints\n", exactCounts[true])
	fmt.Printf("  Minimum frequency: %d hints\n", exactCounts[false])

	fmt.Printf("\nThis generates ALL theoretically possible Wordle hints!\n")
	fmt.Printf("Use this data to build bitvectors for specific word lists.\n")
}
