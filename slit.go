package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"

	flag "github.com/ogier/pflag"
	"github.com/tigrawap/slit/filters"
	"github.com/tigrawap/slit/logging"
	"github.com/tigrawap/slit/utils"
	//"log"
	//"net/http"
	//_ "net/http/pprof"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

const VERSION = "1.1.6"

func init() {
	logging.Config.LogPath = filepath.Join(os.TempDir(), "slit.log")
	slitdir := os.Getenv("SLIT_DIR")
	if slitdir == "" {
		slitdir = filepath.Join(utils.GetHomeDir(), ".slit")
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

func main() {
	flag.StringVarP(&config.outPath, "output", "O", "", "Sets stdin cache location, if not set tmp file used, if set file preserved")
	flag.BoolVar(&logging.Config.Enabled, "debug", false, "Enables debug messages, written to /tmp/slit.log")
	flag.BoolVarP(&config.follow, "follow", "f", false, "Will follow file/stdin")
	showVersion := false
	flag.BoolVar(&showVersion, "version", false, "Print version")
	var keepChars int
	flag.IntVarP(&keepChars, "keep-chars", "K", 0, "Initial num of chars kept during horizontal scrolling")
	var filtersOpt string
	flag.StringVarP(&filtersOpt, "filters", "", "", "Filters file names or inline filters separated by semicolon")
	flag.Parse()
	if showVersion {
		fmt.Println("Slit Version: ", VERSION)
		os.Exit(0)
	}
	stdinStat, _ := os.Stdin.Stat()
	stdoutStat, _ := os.Stdout.Stat()
	var f *os.File
	var err error
	ctx, cancel := context.WithCancel(context.Background())
	if isPipe(stdinStat) && flag.NArg() == 0 {
		config.stdin = true
		if isPipe(stdoutStat) {
			outputToStdout(os.Stdin)
			cancel()
			return
		}
		var cacheFile *os.File
		if config.outPath == "" {
			cacheFile, err = ioutil.TempFile(os.TempDir(), "slit_")
			utils.Check(err)
			defer os.Remove(cacheFile.Name())
		} else {
			cacheFile = utils.OpenRewrite(config.outPath)
		}
		f, err = os.Open(cacheFile.Name())
		utils.Check(err)
		copyDone := sync.WaitGroup{}
		defer cacheFile.Close()
		defer copyDone.Wait()
		defer f.Close()
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
		if err := utils.ValidateRegularFile(filename); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		f, err = os.Open(filename)
		if e, ok := err.(*os.PathError); ok && e.Err == syscall.EACCES {
			fmt.Fprintf(os.Stderr, "%s: Permission denied\n", filename)
			os.Exit(1)
		}
		utils.Check(err)
		defer f.Close()
		if isPipe(stdoutStat) {
			outputToStdout(f)
			cancel()
			return
		}
	}

	var initFilters []*filters.Filter
	if filtersOpt != "" {
		initFilters, err = filters.ParseFiltersOpt(filtersOpt)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
	}

	v := &viewer{
		fetcher:   newFetcher(f),
		ctx:       ctx,
		keepChars: keepChars,
		filters:   initFilters,
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
