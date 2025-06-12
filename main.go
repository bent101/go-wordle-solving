package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
)

// WordleAnalysis provides comprehensive information about the guess
type WordleAnalysis struct {
	Target      string
	Guess       string
	LetterInfos map[byte]LetterInfo
}

// FilterWords returns words from the candidates list that satisfy all the constraints
func (w WordleAnalysis) FilterWords(candidates []string) []string {
	var validWords []string
	for _, word := range candidates {
		if w.WordSatisfiesConstraints(word) {
			validWords = append(validWords, word)
		}
	}
	return validWords
}

// WordSatisfiesConstraints checks if a word satisfies all the letter constraints
func (w WordleAnalysis) WordSatisfiesConstraints(word string) bool {
	// Count letter frequencies in the candidate word
	wordFreq := make(map[byte]int)
	for i := range 5 {
		wordFreq[word[i]]++
	}

	// Check each letter constraint
	for letter, info := range w.LetterInfos {
		actualFreq := wordFreq[letter]

		if info.FrequencyIsExact {
			if actualFreq != info.Frequency {
				return false
			}
		} else {
			if actualFreq < info.Frequency {
				return false
			}
		}

		for _, pos := range info.MustBeInPositions {
			if word[pos] != letter {
				return false
			}
		}

		for _, pos := range info.CantBeInPositions {
			if word[pos] == letter {
				return false
			}
		}
	}

	return true
}

func (w WordleAnalysis) String() string {
	var result strings.Builder

	result.WriteString(fmt.Sprintf("Target: %s, Guess: %s\n", w.Target, w.Guess))

	// Letter constraint analysis
	result.WriteString("Letter Constraints:\n")
	for letter, info := range w.LetterInfos {
		if info.InTarget() {
			freqStr := fmt.Sprintf("%d", info.Frequency)
			if !info.FrequencyIsExact {
				freqStr = "at least " + freqStr
			}
			result.WriteString(fmt.Sprintf("  %c: appears %s times", letter, freqStr))

			if len(info.MustBeInPositions) > 0 {
				result.WriteString(fmt.Sprintf(" | must be in positions: %v", info.MustBeInPositions))
			}
			if len(info.CantBeInPositions) > 0 {
				result.WriteString(fmt.Sprintf(" | can't be in positions: %v", info.CantBeInPositions))
			}

			possible := info.PossiblePositions()
			if len(possible) < 5 && len(info.MustBeInPositions) < info.Frequency {
				result.WriteString(fmt.Sprintf(" | could be in positions: %v", possible))
			}
			result.WriteString("\n")
		} else {
			result.WriteString(fmt.Sprintf("  %c: not in target word\n", letter))
		}
	}

	return result.String()
}

// AnalyzeWordle generates comprehensive Wordle letter information
func AnalyzeWordle(target, guess string) WordleAnalysis {
	// Count letter frequencies in target
	targetFreq := make(map[byte]int)
	for i := range 5 {
		targetFreq[target[i]]++
	}

	// Initialize letter info for all letters in guess
	letterInfos := make(map[byte]LetterInfo)
	for i := range 5 {
		char := guess[i]
		if _, exists := letterInfos[char]; !exists {
			letterInfos[char] = LetterInfo{
				MustBeInPositions: []int{},
				CantBeInPositions: []int{},
				Frequency:         0,
				FrequencyIsExact:  false,
			}
		}
	}

	// Track what we learn from each position
	usedInstances := make(map[byte]int)

	// First pass: identify correct positions
	for i := range 5 {
		guessChar := guess[i]
		targetChar := target[i]
		if guessChar == targetChar {
			usedInstances[guessChar]++

			// Update letter info - we know it must be in this position
			info := letterInfos[guessChar]
			info.MustBeInPositions = append(info.MustBeInPositions, i)
			info.Frequency = max(info.Frequency, usedInstances[guessChar])
			letterInfos[guessChar] = info
		}
	}

	// Second pass: identify wrong positions and frequency constraints
	for i := range 5 {
		guessChar := guess[i]
		targetChar := target[i]

		// Skip if already correct position
		if guessChar == targetChar {
			continue
		}

		targetCount := targetFreq[guessChar]
		usedCount := usedInstances[guessChar]

		info := letterInfos[guessChar]

		if targetCount == 0 {
			// Letter not in target at all
			info.Frequency = 0
			info.FrequencyIsExact = true
		} else if usedCount < targetCount {
			// Letter is in target, wrong position, and we haven't exceeded frequency
			info.CantBeInPositions = append(info.CantBeInPositions, i)
			usedInstances[guessChar]++
			info.Frequency = max(info.Frequency, usedInstances[guessChar])
		} else {
			// Letter is in target but we've exceeded frequency - now we know exact count
			info.CantBeInPositions = append(info.CantBeInPositions, i)
			info.Frequency = targetCount
			info.FrequencyIsExact = true
		}

		letterInfos[guessChar] = info
	}

	// Mark exact frequency for letters we fully mapped
	for letter, info := range letterInfos {
		if !info.FrequencyIsExact && info.Frequency > 0 {
			if len(info.MustBeInPositions) == info.Frequency {
				info.FrequencyIsExact = true
				letterInfos[letter] = info
			}
		}
	}

	return WordleAnalysis{
		Target:      target,
		Guess:       guess,
		LetterInfos: letterInfos,
	}
}

// LetterConstraintKey represents a unique constraint for caching
type LetterConstraintKey struct {
	Letter byte
	Info   LetterInfo
}

func (k LetterConstraintKey) Canonical() string {
	return fmt.Sprintf("%c:%s", k.Letter, k.Info.Canonical())
}

func (k LetterConstraintKey) Hash() string {
	canonical := k.Canonical()
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:8])
}

// PrecomputedConstraints stores bitvectors for all possible letter constraints
type PrecomputedConstraints struct {
	Words       []string                       // All candidate words
	Constraints map[string]*Bitvector          // Key hash -> bitvector
	KeyMap      map[string]LetterConstraintKey // Hash -> original key for debugging
}

func LoadWords(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var words []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		word := strings.TrimSpace(scanner.Text())
		words = append(words, word)
	}

	return words, scanner.Err()
}

func TestLetterConstraint(word string, letter byte, info LetterInfo) bool {
	// Count letter frequency in word
	freq := 0
	for i := range 5 {
		if word[i] == letter {
			freq++
		}
	}

	// Check frequency constraints
	if info.Frequency == 0 {
		if freq > 0 {
			return false
		}
	} else {
		if info.FrequencyIsExact {
			if freq != info.Frequency {
				return false
			}
		} else {
			if freq < info.Frequency {
				return false
			}
		}
	}

	// Check position constraints
	for _, pos := range info.MustBeInPositions {
		if word[pos] != letter {
			return false
		}
	}

	for _, pos := range info.CantBeInPositions {
		if word[pos] == letter {
			return false
		}
	}

	return true
}

func BuildPrecomputedConstraints(words []string) *PrecomputedConstraints {
	allInfos := GenerateAllLetterInfos()
	fmt.Printf("Generated %d valid LetterInfo objects\n", len(allInfos))

	constraints := make(map[string]*Bitvector)
	keyMap := make(map[string]LetterConstraintKey)

	// Calculate total number of constraints (26 letters × N infos)
	totalConstraints := len(allInfos) * 26 // All letters
	processed := 0

	for letter := byte('a'); letter <= byte('z'); letter++ {
		for _, info := range allInfos {
			key := LetterConstraintKey{Letter: letter, Info: info}
			hash := key.Hash()

			// Skip if already processed (duplicate hash)
			if _, exists := constraints[hash]; exists {
				continue
			}

			bv := NewBitvector(len(words))
			for i, word := range words {
				if TestLetterConstraint(word, letter, info) {
					bv.Set(i)
				}
			}

			constraints[hash] = bv
			keyMap[hash] = key
			processed++

			if processed%100 == 0 {
				fmt.Printf("Progress: %d/%d constraints (%.1f%%)\n",
					processed, totalConstraints, float64(processed)*100/float64(totalConstraints))
			}
		}
	}

	fmt.Printf("Completed building %d constraint bitvectors\n", len(constraints))

	return &PrecomputedConstraints{
		Words:       words,
		Constraints: constraints,
		KeyMap:      keyMap,
	}
}

func DemoMain() {
	fmt.Println("=== WORDLE SOLVING DEMO ===")

	// Load word lists
	guesses, err := LoadWords("io/guesses.txt")
	if err != nil {
		fmt.Printf("Error loading guesses: %v\n", err)
		return
	}

	answers, err := LoadWords("io/answers.txt")
	if err != nil {
		fmt.Printf("Error loading answers: %v\n", err)
		return
	}

	fmt.Printf("Loaded %d guesses and %d answers\n", len(guesses), len(answers))

	// Build constraints once
	fmt.Println("\n=== BUILDING CONSTRAINTS ===")
	precomputed := BuildPrecomputedConstraints(answers)

	// Performance test using bitvectors
	fmt.Println("\n=== PERFORMANCE TEST (BITVECTORS) ===")

	validWordsFunc := func(candidates []string, analysis WordleAnalysis) []string {
		if len(candidates) == 0 {
			return []string{}
		}

		// Start with all candidates
		result := NewBitvector(len(candidates))
		for i := 0; i < len(candidates); i++ {
			result.Set(i)
		}

		// Apply each constraint
		for letter, info := range analysis.LetterInfos {
			key := LetterConstraintKey{Letter: letter, Info: info}
			hash := key.Hash()

			if bv, exists := precomputed.Constraints[hash]; exists {
				result = result.And(bv)
			}
		}

		// Convert back to word list
		validWords := make([]string, 0)
		for _, idx := range result.GetSetIndices() {
			if idx < len(candidates) {
				validWords = append(validWords, candidates[idx])
			}
		}

		return validWords
	}

	// Test equivalence
	candidates := answers
	example := AnalyzeWordle("crane", "audio")
	traditionalValid := example.FilterWords(candidates)
	bitvectorValid := validWordsFunc(candidates, example)

	sort.Strings(traditionalValid)
	sort.Strings(bitvectorValid)

	match := len(bitvectorValid) == len(traditionalValid)
	if match {
		for i, word := range bitvectorValid {
			if i >= len(traditionalValid) || word != traditionalValid[i] {
				match = false
				break
			}
		}
	}

	if match {
		fmt.Printf("✓ Bitvector method matches traditional method\n")
	} else {
		fmt.Printf("✗ Bitvector method differs from traditional method\n")
	}

	// Performance summary
	fmt.Printf("\n=== PERFORMANCE SUMMARY ===\n")
	fmt.Printf("- Total constraints precomputed: %d\n", len(precomputed.Constraints))
	fmt.Printf("- Total candidate words: %d\n", len(candidates))
	fmt.Printf("- Words remaining after filtering: %d\n", len(bitvectorValid))
	fmt.Printf("- Reduction: %.1f%%\n", 100.0*(1.0-float64(len(bitvectorValid))/float64(len(candidates))))
}

func FindBestStartingWord(guesses, answers []string) (string, float64) {
	fmt.Printf("Testing %d guesses against %d answers...\n", len(guesses), len(answers))

	bestGuess := ""
	bestAverage := float64(len(answers)) // Start with worst case

	for guessIdx, guess := range guesses {
		totalRemaining := 0

		if (guessIdx+1)%100 == 0 {
			fmt.Printf("Progress: %d/%d (%.1f%%)\n",
				guessIdx, len(guesses), float64(guessIdx)*100/float64(len(guesses)))
		}

		for _, answer := range answers {
			analysis := AnalyzeWordle(answer, guess)
			validWords := analysis.FilterWords(answers)
			totalRemaining += len(validWords)
		}

		average := float64(totalRemaining) / float64(len(answers))

		if average < bestAverage {
			bestAverage = average
			bestGuess = guess
		}
	}

	return bestGuess, bestAverage
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "demo":
			DemoMain()
		case "best":
			BestWordMain()
		case "precompute":
			PrecomputeMain()
		default:
			fmt.Println("Usage: go run . [demo|best|precompute]")
		}
	} else {
		DemoMain()
	}
}

func BestWordMain() {
	fmt.Println("=== FINDING BEST STARTING WORD ===")

	// Load word lists
	guesses, err := LoadWords("io/guesses.txt")
	if err != nil {
		fmt.Printf("Error loading guesses: %v\n", err)
		return
	}

	answers, err := LoadWords("io/answers.txt")
	if err != nil {
		fmt.Printf("Error loading answers: %v\n", err)
		return
	}

	fmt.Printf("Loaded %d guesses and %d answers\n", len(guesses), len(answers))

	bestGuess, bestAverage := FindBestStartingWord(guesses, answers)

	fmt.Printf("\n=== RESULTS ===\n")
	fmt.Printf("Best starting word: %s\n", bestGuess)
	fmt.Printf("Average remaining words: %.2f\n", bestAverage)
	fmt.Printf("Average reduction: %.1f%%\n", 100.0*(1.0-bestAverage/float64(len(answers))))

	fmt.Printf("\nTesting with example answers:\n")
	examples := []string{answers[0], answers[len(answers)/4], answers[len(answers)/2], answers[3*len(answers)/4], answers[len(answers)-1]}
	for _, answer := range examples {
		analysis := AnalyzeWordle(answer, bestGuess)
		validWords := analysis.FilterWords(answers)
		fmt.Printf("Answer: %s → %d remaining (%.1f%% reduction)\n",
			answer, len(validWords), 100.0*(1.0-float64(len(validWords))/float64(len(answers))))
	}
}
