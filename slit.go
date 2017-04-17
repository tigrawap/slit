package main

import (
	"flag"
	"os"
	"github.com/tigrawap/slit/logging"
	"io/ioutil"
	"io"
	"fmt"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func init() {
	logging.Config.LogPath = "/tmp/slit.log"
}

var config struct{
	outPath string
}

func main() {
	flag.StringVar(&config.outPath, "O", "", "Sets stdin cache location, if not set tmp file used, if set file preserved")
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
		var cacheFile *os.File
		if config.outPath == ""{
			cacheFile, err = ioutil.TempFile("/tmp", "slit_")
			check(err)
			defer os.Remove(cacheFile.Name())
		}else{
			openFile := func() error{
				cacheFile, err = os.OpenFile(config.outPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
				return err
			}
			if err = openFile(); os.IsExist(err){
				os.Remove(config.outPath)
				err = openFile()
			}
			check(err)
		}
		f, err = os.Open(cacheFile.Name())
		check(err)
		defer cacheFile.Close()
		defer f.Close()
		go io.Copy(cacheFile, os.Stdin)
	} else {
		if flag.NArg() != 1 {
			fmt.Fprintln(os.Stderr,"Only viewing of one file or from STDIN is supported")
			os.Exit(1)
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
