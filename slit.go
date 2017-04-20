package main

import (
	"flag"
	"fmt"
	"github.com/tigrawap/slit/logging"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func init() {
	logging.Config.LogPath = filepath.FromSlash("/tmp/slit.log")
	currentUser, err := user.Current()
	if err != nil {
		logging.Debug("Could not obtain current user")
		config.historyPath = filepath.FromSlash("/tmp/.slit/history")
	} else {
		config.historyPath = filepath.Join(currentUser.HomeDir, ".slit", "history")
	}
}

var config struct {
	outPath       string
	historyPath   string
	stdin         bool
	stdinFinished bool
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
		config.stdin = true
		config.stdinFinished = false
		if isPipe(stdoutStat) {
			outputToStdout(os.Stdin)
			return
		}
		var cacheFile *os.File
		if config.outPath == "" {
			cacheFile, err = ioutil.TempFile(os.TempDir(), "slit_")
			check(err)
			defer os.Remove(cacheFile.Name())
		} else {
			cacheFile = openRewrite(config.outPath)
		}
		f, err = os.Open(cacheFile.Name())
		check(err)
		defer cacheFile.Close()
		defer f.Close()
		go func() {
			io.Copy(cacheFile, os.Stdin)
			config.stdinFinished = true
		}()
	} else {
		if flag.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "Only viewing of one file or from STDIN is supported")
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
	if info == nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) == 0
}
