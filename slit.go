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
	keepChars     int
	initFilters   []*filters.Filter
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

type Slit struct {
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	file        *os.File
	isCacheFile bool // if true, file will be removed on shutdown
}

func (s *Slit) Display() {
	defer s.Shutdown()
	v := &viewer{
		fetcher:   newFetcher(s.file),
		ctx:       s.ctx,
		keepChars: config.keepChars,
		filters:   config.initFilters,
	}
	v.termGui()
	s.cancel()
}

func (s *Slit) Shutdown() {
	s.wg.Wait()
	s.file.Close()
	if s.isCacheFile {
		os.Remove(s.file.Name())
	}
}

func New(f *os.File) (*Slit, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Slit{
		wg:     sync.WaitGroup{},
		ctx:    ctx,
		cancel: cancel,
		file:   f,
	}
	return s, nil
}

func NewFromStdin() (*Slit, error) {
	var err error
	var cacheFile *os.File

	if config.outPath == "" {
		cacheFile, err = ioutil.TempFile(os.TempDir(), "slit_")
		if err != nil {
			return nil, err
		}
	} else {
		cacheFile = utils.OpenRewrite(config.outPath)
	}

	f, err := os.Open(cacheFile.Name())
	if err != nil {
		return nil, err
	}

	s, err := New(f)
	if err != nil {
		return nil, err
	}
	s.isCacheFile = true
	s.wg.Add(1)
	go func() {
		var err error
	copyLoop:
		for {
			select {
			case <-s.ctx.Done():
				break copyLoop
			default:
				_, err = io.CopyN(cacheFile, os.Stdin, 64*1024)
				if err != nil {
					break copyLoop
				}
			}
		}
		close(config.stdinFinished)
		s.wg.Done()
	}()

	return s, nil
}

func NewFromFilepath(path string) (*Slit, error) {
	if err := utils.ValidateRegularFile(path); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if e, ok := err.(*os.PathError); ok && e.Err == syscall.EACCES {
		return nil, fmt.Errorf("%s: Permission denied\n", path)
	}
	if err != nil {
		return nil, err
	}
	return New(f)
}

func main() {
	var filtersOpt string

	flag.StringVarP(&config.outPath, "output", "O", "", "Sets stdin cache location, if not set tmp file used, if set file preserved")
	flag.BoolVar(&logging.Config.Enabled, "debug", false, "Enables debug messages, written to /tmp/slit.log")
	flag.BoolVarP(&config.follow, "follow", "f", false, "Will follow file/stdin")
	showVersion := false
	flag.BoolVar(&showVersion, "version", false, "Print version")
	flag.IntVarP(&config.keepChars, "keep-chars", "K", 0, "Initial num of chars kept during horizontal scrolling")
	flag.StringVarP(&filtersOpt, "filters", "", "", "Filters file names or inline filters separated by semicolon")
	flag.Parse()

	if showVersion {
		fmt.Println("Slit Version: ", VERSION)
		os.Exit(0)
	}

	stdinStat, _ := os.Stdin.Stat()
	stdoutStat, _ := os.Stdout.Stat()

	var s *Slit
	var err error

	if isPipe(stdinStat) && flag.NArg() == 0 {
		config.stdin = true
		if isPipe(stdoutStat) {
			outputToStdout(os.Stdin)
			return
		}
		s, err = NewFromStdin()
		exitOnErr(err)
	} else {
		if flag.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "Only viewing of one file or from STDIN is supported")
			os.Exit(1)
		}
		path := flag.Arg(0)
		s, err = NewFromFilepath(path)
		exitOnErr(err)
		if isPipe(stdoutStat) {
			outputToStdout(s.file)
			return
		}
	}

	if filtersOpt != "" {
		initFilters, err := filters.ParseFiltersOpt(filtersOpt)
		exitOnErr(err)
		config.initFilters = initFilters
	}

	s.Display()
}

func exitOnErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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
