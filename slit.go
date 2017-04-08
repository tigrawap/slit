package main

import (
	"flag"
	"os"
	"errors"
	"github.com/tigrawap/slit/logging"
)

var config struct {
	filename string
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	defer logging.Timeit("iterating")()
	flag.Parse()
	if flag.NArg() != 1 {
		panic(errors.New("Viewing on single file only supported now"))
	}
	//data := readFile(flag.Arg(0))
	//for i := 0; i < 20; i++ {
	//	readFile(flag.Arg(0))
	//}
	//return
	f, err := os.Open(flag.Arg(0))
	check(err)
	defer f.Close()
	v := &viewer{
		fetcher: newFetcher(f),
	}
	v.termGui()
}
