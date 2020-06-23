package main

import (
	"bufio"
	"github.com/foxen/urls/counter"
	"os"
)

var goCountFunc counter.CountFunc = func(bs []byte) (int, error) {
	cnt := 0
	isG := false
	for _, b := range bs {
		if b == 'G' {
			isG = true
			continue
		}
		if b == 'o' && isG {
			cnt++
		}
		isG = false
	}
	return cnt, nil
}

func main() {
	ctr := counter.New(counter.Options{MaxJobsN: 5})
	if err := ctr.CountWith(goCountFunc, bufio.NewReader(os.Stdin), os.Stdout); err != nil {
		panic(err)
	}
}
