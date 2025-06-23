package main

import (
	"encoding/gob"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
)

type Hint uint8

type HintInfo struct {
	Bitvec *Bitvec
}

type GuessInfo struct {
	AnswerHints map[string]Hint
	HintsMap    map[Hint]*HintInfo
}

var guessesFile, _ = os.ReadFile("io/guesses.txt")
var answersFile, _ = os.ReadFile("io/answers.txt")
var guesses = strings.Split(string(guessesFile), "\n")
var answers = strings.Split(string(answersFile), "\n")

// load guessesMap from disk if possible
var guessesMap = loadGuessesMap()

func loadGuessesMap() map[string]*GuessInfo {
	file, err := os.Open("guesses_cache.gob")
	if err != nil {
		fmt.Println("Cache file not found, will calculate from scratch")
		return map[string]*GuessInfo{}
	}
	defer file.Close()

	start := time.Now()

	var guessesMap map[string]*GuessInfo
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&guessesMap)
	if err != nil {
		fmt.Println("Error decoding cache, will recalculate:", err)
		return map[string]*GuessInfo{}
	}

	fmt.Printf("Loaded guesses cache with %d entries in %v\n", len(guessesMap), time.Since(start))
	return guessesMap
}

func saveGuessesMap() {
	file, err := os.Create("guesses_cache.gob")
	if err != nil {
		fmt.Println("Error creating cache file:", err)
		return
	}
	defer file.Close()

	start := time.Now()

	encoder := gob.NewEncoder(file)
	err = encoder.Encode(guessesMap)
	if err != nil {
		fmt.Println("Error encoding cache:", err)
		return
	}

	fmt.Printf("Saved guesses cache to disk in %v\n", time.Since(start))
}

func main() {
	// run these functions if guessesMap was not loaded from disk
	if len(guessesMap) == 0 {
		calculateHints()
		calculateBitvecs()
		// calculateHintGuesses()
		// save guessesMap to disk if needed
		saveGuessesMap()
	}

	printWordHints("roate")

	// findBestGuess()
}

func calculateHintGuesses() {
	panic("unimplemented")
}

func calculateHints() {
	fmt.Println("calculating hints for all guess-answer pairs")
	bar := progressbar.Default(int64(len(guesses)))

	var wg sync.WaitGroup

	for _, guess := range guesses {
		answerHints := make(map[string]Hint)
		hintsMap := make(map[Hint]*HintInfo)

		guessesMap[guess] = &GuessInfo{
			answerHints,
			hintsMap,
		}

		wg.Add(1)

		go func() {
			defer wg.Done()
			for _, answer := range answers {
				hint := getHint(guess, answer)
				answerHints[answer] = hint

				if hintsMap[hint] == nil {
					hintsMap[hint] = &HintInfo{
						Bitvec: NewBitvec(len(answers)),
					}
				}
			}
			bar.Add(1)
		}()
	}

	wg.Wait()
}

func calculateBitvecs() {
	numUniqueHints := 0
	for _, guessInfo := range guessesMap {
		numUniqueHints += len(guessInfo.HintsMap)
	}

	fmt.Println("calculating bitvecs for", numUniqueHints, "unique hints")
	bar := progressbar.Default(int64(numUniqueHints))

	var wg sync.WaitGroup

	for _, guessInfo := range guessesMap {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for hint, hintInfo := range guessInfo.HintsMap {
				bar.Add(1)
				for answerIdx, answer := range answers {
					hint2 := guessInfo.AnswerHints[answer]
					if hint2 == hint {
						hintInfo.Bitvec.Set(answerIdx)
					}
				}
			}

		}()
	}

	wg.Wait()
}

func findBestGuess() {
	fmt.Printf("Finding best guess pair\n")

	guessBitvecs := []*Bitvec{}
	filteredGuesses := []string{}

	for _, guess := range guesses {
		bitvec := NewBitvec(26)

		for i := range 5 {
			j := int(guess[i] - 'a')
			bitvec.Set(j)
		}

		if bitvec.Count == 5 {
			guessBitvecs = append(guessBitvecs, bitvec)
			filteredGuesses = append(filteredGuesses, guess)
		}
	}

	totalPairs := int64(len(filteredGuesses) * (len(filteredGuesses) - 1) / 2)
	fmt.Printf("filtered down to %v guesses with 5 unique letters (%v pairs)\n", len(filteredGuesses), totalPairs)

	bar := progressbar.Default(totalPairs)

	bestGuess1 := filteredGuesses[0]
	bestGuess2 := filteredGuesses[1]
	bestGuessVal := AvgNumCandidates(bestGuess1, bestGuess2)

	mu := sync.Mutex{}
	wg := sync.WaitGroup{}

	for i := range len(filteredGuesses) - 1 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := i + 1; j < len(filteredGuesses); j++ {
				guess1 := filteredGuesses[i]
				guess2 := filteredGuesses[j]

				if guessBitvecs[i].And(guessBitvecs[j]).Count != 0 {
					bar.Add(1)
					continue
				}

				guessVal := AvgNumCandidates(guess1, guess2)
				mu.Lock()
				if guessVal < bestGuessVal {
					bestGuess1 = guess1
					bestGuess2 = guess2
					bestGuessVal = guessVal
					bar.Describe(fmt.Sprintf("Best: %v, %v (%.2f)", bestGuess1, bestGuess2, bestGuessVal))
				}
				mu.Unlock()
				bar.Add(1)
			}
		}()
	}

	wg.Wait()

	fmt.Printf("Done, best guess pair: %v, %v (%.2f)\n", bestGuess1, bestGuess2, bestGuessVal)
}

func getHint(guess, answer string) Hint {
	var charHints [5]uint8

	for i, ch := range guess {
		if answer[i] == byte(ch) {
			charHints[i] = 2
		} else if strings.ContainsRune(answer, ch) {
			charHints[i] = 1
		}
	}

	var ret uint8
	for _, d := range charHints {
		ret = (ret * 3) + d
	}

	return Hint(ret)
}

func lookupBitvec(guess, answer string) *Bitvec {
	answerHints := guessesMap[guess].AnswerHints
	hintsMap := guessesMap[guess].HintsMap
	return hintsMap[answerHints[answer]].Bitvec
}

func (h Hint) String() string {
	hintReplacer := strings.NewReplacer("0", "⬜", "1", "🟨", "2", "🟩")
	base3Str := strconv.FormatUint(uint64(h), 3)
	paddedBase3Str := fmt.Sprintf("%05s", base3Str)

	return hintReplacer.Replace(paddedBase3Str)
}

// ColoredWord displays a word with colored backgrounds based on the hint
func (h Hint) ColoredWord(word string) string {
	if len(word) != 5 {
		return word // Return unchanged if not 5 characters
	}

	// ANSI color codes
	const (
		reset    = "\033[0m"
		grayBg   = "\033[48;5;236m\033[38;5;255m" // gray background, white text
		yellowBg = "\033[43m\033[30m"             // yellow background, black text
		greenBg  = "\033[42m\033[30m"             // green background, black text
	)

	// Convert hint back to individual digits
	hintValue := uint64(h)
	var digits [5]int
	for i := 4; i >= 0; i-- {
		digits[i] = int(hintValue % 3)
		hintValue /= 3
	}

	var result strings.Builder
	for i, char := range word {
		switch digits[i] {
		case 0: // No match
			result.WriteString(grayBg)
		case 1: // Wrong position
			result.WriteString(yellowBg)
		case 2: // Correct position
			result.WriteString(greenBg)
		}
		result.WriteRune(char)
		result.WriteString(" ") // Add space between letters
		result.WriteString(reset)
	}

	return result.String()
}

func AvgNumCandidates(firstGuess string, guesses ...string) float64 {
	var tot float64

	for _, answer := range answers {
		bitvec := lookupBitvec(firstGuess, answer)
		broke := false

		for _, guess := range guesses {
			if bitvec.Count <= 2 {
				broke = true
				tot += 1.0
				break
			}
			bitvec = bitvec.And(lookupBitvec(guess, answer))
		}

		if !broke {
			tot += float64(bitvec.Count)
		}
	}

	return tot / float64(len(answers))
}

func printWordHints(word string) {
	type HintCount struct {
		hint  Hint
		count int
	}

	var hintCounts []HintCount
	for hint, hintInfo := range guessesMap[word].HintsMap {
		hintCounts = append(hintCounts, HintCount{hint, hintInfo.Bitvec.Count})
	}

	// Sort by count in descending order (high to low)
	sort.Slice(hintCounts, func(i, j int) bool {
		return hintCounts[i].count > hintCounts[j].count
	})

	// Print sorted results
	for _, hc := range hintCounts {
		fmt.Println(hc.hint.ColoredWord(word), hc.count)
	}
}
