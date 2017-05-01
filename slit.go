package main

import (
	"context"
	"fmt"
	flag "github.com/ogier/pflag"
	"github.com/tigrawap/slit/logging"
	"io"
	"io/ioutil"
	//"log"
	//"net/http"
	//_ "net/http/pprof"
	"os"
	"os/user"
	"path/filepath"
	"sync"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func init() {
	logging.Config.LogPath = filepath.Join(os.TempDir(), "slit.log")
	slitdir := os.Getenv("SLIT_DIR")
	if slitdir == "" {
		currentUser, err := user.Current()
		var homedir string
		if err != nil {
			homedir = os.Getenv("HOME")
			if homedir == "" {
				homedir = os.TempDir()
			}
		} else {
			homedir = currentUser.HomeDir
		}
		slitdir = filepath.Join(homedir, ".slit")
	}
	config.historyPath = filepath.Join(slitdir, "history")
	config.stdinFinished = make(chan struct{})
	//go func() {
	//	log.Println(http.ListenAndServe("localhost:6060", nil))
	//}()
}

type Config struct {
	outPath       string
	historyPath   string
	stdin         bool
	stdinFinished chan struct{}
	follow        bool
}

var config Config

func (c *Config) isStdinRead() bool {
	select {
	case <-c.stdinFinished:
		return true
	default:
		return false
	}

}

const VERSION = "1.1.2"

func main() {
	flag.StringVarP(&config.outPath, "output", "O", "", "Sets stdin cache location, if not set tmp file used, if set file preserved")
	flag.BoolVar(&logging.Config.Enabled, "debug", false, "Enables debug messages, written to /tmp/slit.log")
	flag.BoolVarP(&config.follow, "follow", "f", false, "Will follow file/stdin")
	showVersion := false
	flag.BoolVar(&showVersion, "version", false, "Print version")
	flag.Parse()
	if showVersion {
		fmt.Println("Slit Version: ", VERSION)
		os.Exit(0)
	}
	stdinStat, _ := os.Stdin.Stat()
	stdoutStat, _ := os.Stdout.Stat()
	var f *os.File
	var err error
	var wg sync.WaitGroup
	defer wg.Wait()
	ctx, cancel := context.WithCancel(context.Background())
	if isPipe(stdinStat) {
		config.stdin = true
		if isPipe(stdoutStat) {
			outputToStdout(os.Stdin)
			cancel()
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
		copyDone := sync.WaitGroup{}
		defer cacheFile.Close()
		defer copyDone.Wait()
		defer f.Close()
		defer wg.Done()
		copyDone.Add(1)
		go func() {
			var err error
		copyLoop:
			for {
				select {
				case <-ctx.Done():
					break copyLoop
				default:
					_, err = io.CopyN(cacheFile, os.Stdin, 64*1024)
					if err != nil {
						break copyLoop
					}
				}
			}
			close(config.stdinFinished)
			copyDone.Done()
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
		defer wg.Done()
		if isPipe(stdoutStat) {
			outputToStdout(f)
			cancel()
			return
		}
	}

	wg.Add(1)
	v := &viewer{
		fetcher: newFetcher(f),
		ctx:     ctx,
	}
	v.termGui()
	cancel()
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
