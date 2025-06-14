package handler

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/schollz/progressbar/v3"
)

type Hint uint8

type HintInfo struct {
	freq   int
	bitvec *Bitvec
}

type GuessInfo struct {
	answerHints map[string]Hint
	hintsMap    map[Hint]*HintInfo
}

var guessesFile, _ = os.ReadFile("io/guesses.txt")
var answersFile, _ = os.ReadFile("io/answers.txt")
var guesses = strings.Split(string(guessesFile), "\n")
var answers = strings.Split(string(answersFile), "\n")
var guessesMap = map[string]*GuessInfo{}

func main() {
	calculateHints()

	calculateBitvecs()

	findBestGuess()
}

func calculateHints() {
	fmt.Println("calculating hints for all guess-answer pairs")
	bar := progressbar.Default(int64(len(guesses)))

	var wg sync.WaitGroup

	for _, guess := range guesses {
		wg.Add(1)

		answerHints := make(map[string]Hint)
		hintsMap := make(map[Hint]*HintInfo)

		guessesMap[guess] = &GuessInfo{
			answerHints,
			hintsMap,
		}

		go func() {
			defer wg.Done()
			for _, answer := range answers {
				hint := getHint(guess, answer)
				answerHints[answer] = hint

				if hintsMap[hint] != nil {
					hintsMap[hint].freq++
				} else {
					hintsMap[hint] = &HintInfo{
						freq:   1,
						bitvec: NewBitvec(len(answers)),
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
		numUniqueHints += len(guessInfo.hintsMap)
	}

	fmt.Println("calculating bitvecs for", numUniqueHints, "unique hints")
	bar := progressbar.Default(int64(numUniqueHints))

	var wg sync.WaitGroup

	for _, guessInfo := range guessesMap {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for hint, hintInfo := range guessInfo.hintsMap {
				bar.Add(1)
				for answerIdx, answer := range answers {
					hint2 := guessInfo.answerHints[answer]
					if hint2 == hint {
						hintInfo.bitvec.Set(answerIdx)
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
	bestGuessVal := MaxNumCandidates(bestGuess1, bestGuess2)

	mu := sync.Mutex{}
	wg := sync.WaitGroup{}

	for i := range len(filteredGuesses) - 1 {
		go func() {
			for j := i + 1; j < len(filteredGuesses); j++ {
				guess1 := filteredGuesses[i]
				guess2 := filteredGuesses[j]

				if guessBitvecs[i].And(guessBitvecs[j]).Count != 0 {
					bar.Add(1)
					continue
				}

				wg.Add(1)
				guessVal := MaxNumCandidates(guess1, guess2)
				mu.Lock()
				if guessVal < bestGuessVal {
					bestGuess1 = guess1
					bestGuess2 = guess2
					bestGuessVal = guessVal
					bar.Describe(fmt.Sprintf("Best: %v, %v (%v)", bestGuess1, bestGuess2, bestGuessVal))
				}
				mu.Unlock()
				wg.Done()
				bar.Add(1)
			}
		}()
	}

	wg.Wait()

	fmt.Printf("Done, best guess pair: %v, %v (%v)\n", bestGuess1, bestGuess2, bestGuessVal)
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
	answerHints := guessesMap[guess].answerHints
	hintsMap := guessesMap[guess].hintsMap
	return hintsMap[answerHints[answer]].bitvec
}

func (h Hint) String() string {
	hintReplacer := strings.NewReplacer("0", "⬜", "1", "🟨", "2", "🟩")
	base3Str := strconv.FormatUint(uint64(h), 3)
	paddedBase3Str := fmt.Sprintf("%05s", base3Str)

	return hintReplacer.Replace(paddedBase3Str)
}

func AvgNumCandidates(firstGuess string, guesses ...string) float64 {
	tot := 0.0

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

func MaxNumCandidates(firstGuess string, guesses ...string) int {
	ret := 1

	for _, answer := range answers {
		bitvec := lookupBitvec(firstGuess, answer)
		broke := false

		for _, guess := range guesses {
			if bitvec.Count <= 2 {
				broke = true
				ret = max(ret, 1)
				break
			}
			bitvec = bitvec.And(lookupBitvec(guess, answer))
		}

		if !broke {
			ret = max(ret, bitvec.Count)
		}
	}

	return ret
}
