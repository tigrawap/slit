package main

import (
	"flag"
	"os"
	"errors"
	"github.com/tigrawap/slit/logging"
	"io/ioutil"
	"io"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func init() {
	logging.Config.LogPath = "/tmp/slit.log"
}

func main() {
	flag.BoolVar(&logging.Config.Enabled, "debug", false, "Enables debug messages, written to /tmp/slit.log")
	flag.Parse()
	stdinStat, _ := os.Stdin.Stat()
	stdoutStat, _ := os.Stdout.Stat()
	var f *os.File
	var err error
	if isPipe(stdinStat) {
		if isPipe(stdoutStat) {
			outputToStdout(os.Stdin)
			return
		}
		tmpFile, err := ioutil.TempFile("/tmp", "slit_")
		check(err)
		f, err = os.Open(tmpFile.Name())
		check(err)
		defer os.Remove(tmpFile.Name())
		defer tmpFile.Close()
		defer f.Close()
		go io.Copy(tmpFile, os.Stdin)
	} else {
		if flag.NArg() != 1 {
			panic(errors.New("Viewing of single file only supported for now"))
		}
		filename := flag.Arg(0)
		f, err = os.Open(filename)
		check(err)
		defer f.Close()
		if isPipe(stdoutStat) {
			outputToStdout(f)
			return
		}
	}

	v := &viewer{
		fetcher: newFetcher(f),
	}
	v.termGui()
}
func outputToStdout(file *os.File) {
	io.Copy(os.Stdout, file)
}
func isPipe(info os.FileInfo) bool {
	return (info.Mode() & os.ModeCharDevice) == 0
}
