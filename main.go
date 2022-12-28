package main

import (
	"fmt"

	"github.com/bent101/go-wordle-solving/hint"
)

func main() {
	h1 := hint.New("trees", "roate")
	fmt.Println(h1.sequence)
}
