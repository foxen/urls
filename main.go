package main

import (
	"bufio"
	"github.com/foxen/urls/counter"
	"os"
)

func main() {
	ctr := counter.New(counter.Options{MaxJobsN: 5})
	if err := ctr.Count(bufio.NewReader(os.Stdin), os.Stdout, "Go"); err != nil {
		panic(err)
	}
}
