// Command parity emits wrapper results for comparison with the upstream npm package.
package main

import (
	"bufio"
	"encoding/json"
	garu "github.com/softplus10/garu-go"
	"log"
	"os"
)

func main() {
	a, err := garu.New()
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()
	scan := bufio.NewScanner(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	for scan.Scan() {
		text := scan.Text()
		r, err := a.Analyze(text)
		if err != nil {
			log.Fatal(err)
		}
		nouns, err := a.Nouns(text, garu.NounsOptions{IncludeSL: true})
		if err != nil {
			log.Fatal(err)
		}
		if err := enc.Encode(struct {
			Text   string      `json:"text"`
			Result garu.Result `json:"result"`
			Nouns  []string    `json:"nouns"`
		}{text, r, nouns}); err != nil {
			log.Fatal(err)
		}
	}
	if err := scan.Err(); err != nil {
		log.Fatal(err)
	}
}
