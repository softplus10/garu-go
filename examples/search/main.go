package main

import (
	"fmt"
	garu "github.com/softplus10/garu-go"
	"log"
)

func main() {
	analyzer, err := garu.New()
	if err != nil {
		log.Fatal(err)
	}
	defer analyzer.Close()
	nouns, err := analyzer.Nouns("도서관의 책과 AI 문서", garu.NounsOptions{IncludeSL: true})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(nouns)
}
