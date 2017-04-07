package main

import (
	"flag"
	"os"
	"bufio"
	"errors"
	"github.com/tigrawap/slit/ansi"
	"io"
	"github.com/tigrawap/slit/logging"
	"sync"
)

var config struct {
	filename string
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

type RuneFile []ansi.Arunes

func readLines(reader *bufio.Reader, ret chan<- []byte) {
	defer close(ret)
	for {
		str, err := reader.ReadBytes('\n')
		if err == nil {
			ret <- str[:len(str)-1]
			continue
		}
		if err == io.EOF {
			ret <- str
			return
		} else {
			panic(err)
		}
	}
}

type runeReady struct{
	i int
	r ansi.Arunes
}

func readFile(filename string) RuneFile {
	defer logging.Timeit("full file read")()
	f, err := os.Open(filename)
	check(err)
	defer f.Close()
	data := RuneFile{}
	reader := bufio.NewReader(f)
	lines := make(chan []byte, 100)
	go readLines(reader, lines)
	w := sync.WaitGroup{}
	i := 0
	semaphore := make(chan struct{}, 200)
	for line := range lines {
		w.Add(1)
		line:=line
		data = append(data, ansi.Arunes{})
		semaphore <- struct{}{}
		go func(i int) {
			data[i] = ansi.NewArune(line)
			//appendChan <- runeReady{i, ansi.NewArune(line)}
			w.Done()
			<- semaphore
		}(i)
		i+=1
	}
	w.Wait()
	return data
}

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		panic(errors.New("Viewing on single file only supported now"))
	}
	data := readFile(flag.Arg(0))
	//return
	viewer := &viewer{
		rf: data,
	}
	viewer.termGui()
}
