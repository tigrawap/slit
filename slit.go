package slit

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"time"
	//"log"
	//"net/http"
	//_ "net/http/pprof"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/nsf/termbox-go"
	"github.com/tigrawap/slit/filters"
	"github.com/tigrawap/slit/logging"
	"github.com/tigrawap/slit/utils"
)

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

// Slit is a configured instance of the pager, ready to be displayed
type Slit struct {
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	file        *os.File
	isCacheFile bool // if true, file will be removed on shutdown
	fetcher     *Fetcher
	initialised bool
}

// Returns input file, original or cache file when reading from stdin
func (s *Slit) GetFile() *os.File { return s.file }

// Set explicit stdin cache location
func (s *Slit) SetOutPath(path string) { config.outPath = path }

// Set whether to follow file/stdin
func (s *Slit) SetFollow(b bool) { config.follow = b }

// Set initial num of chars kept during horizontal scrolling
func (s *Slit) SetKeepChars(i int) { config.keepChars = i }

// Set initial filters
func (s *Slit) SetFilters(f []*filters.Filter) { config.initFilters = f }

// Invoke the Slit UI
func (s *Slit) Init() {
	s.fetcher = newFetcher(s.file, s.ctx)
	s.fetcher.filters = config.initFilters
	s.initialised = true
}

// Invoke the Slit UI
func (s *Slit) Display() {
	if !s.initialised {
		s.Init()
	}

	s.file.Seek(0, io.SeekStart)
	//s.fetcher = newFetcher(s.file, s.ctx)
	s.fetcher.filters = config.initFilters
	s.fetcher.seek(0)
	s.initialised = true
	v := &viewer{
		fetcher:   s.fetcher,
		ctx:       s.ctx,
		keepChars: config.keepChars,
	}
	v.termGui()
}

// Shutdown and cleanup this pager instance. After instance shutdown,
// it cannot be displayed again
func (s *Slit) Shutdown() {
	s.cancel()
	s.wg.Wait()
	s.file.Close()
	if s.isCacheFile {
		os.Remove(s.file.Name())
	}
}

func New(f *os.File) *Slit {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Slit{
		wg:     sync.WaitGroup{},
		ctx:    ctx,
		cancel: cancel,
		file:   f,
	}
	return s
}

func NewFromStream(ch chan string) (*Slit, error) {
	cacheFile, err := mkCacheFile()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(cacheFile.Name())
	if err != nil {
		return nil, err
	}

	s := New(f)
	s.isCacheFile = true

	w := bufio.NewWriter(cacheFile)
	lock := sync.Mutex{}
	done := false

	s.wg.Add(1)
	go func() {
		for _ = range time.Tick(100 * time.Millisecond) {
			lock.Lock()
			w.Flush()
			lock.Unlock()
			if done {
				break
			}
		}
		s.wg.Done()
	}()

	s.wg.Add(1)
	go func() {
		defer func() {
			done = true
			s.wg.Done()
		}()

		for {
			select {
			case line, ok := <-ch:
				if !ok { // channel closed
					return
				}
				lock.Lock()
				_, err := w.WriteString(line + "\n")
				if err != nil {
					panic(err)
					//return
				}
				lock.Unlock()
			case <-s.ctx.Done():
				return
			}
		}
	}()

	return s, nil
}

func NewFromStdin() (*Slit, error) {
	config.stdin = true

	cacheFile, err := mkCacheFile()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(cacheFile.Name())
	if err != nil {
		return nil, err
	}

	s := New(f)
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
	return New(f), nil
}

func mkCacheFile() (f *os.File, err error) {
	if config.outPath == "" {
		f, err = ioutil.TempFile(os.TempDir(), "slit_")
	} else {
		f = utils.OpenRewrite(config.outPath)
	}
	return f, err
}

func (s *Slit) CanFitDisplay(ctx context.Context) bool {
	s.Init()
	termbox.Init()
	w, h := termbox.Size()
	termbox.Close()
	localCtx, cancel := context.WithCancel(ctx)
	parsedLineCount := 0
	lines := s.fetcher.Get(localCtx, Pos{})
	defer func(){
		for range lines {} // Draining channel to avoid races on fetcher
	}()
	defer cancel()
FORLOOP:
	for {
		select {
		case <-ctx.Done():
			return false
		case line, isOpen := <-lines:
			if !isOpen {
				if config.stdin && !config.isStdinRead() {
					select {
					case <-ctx.Done():
						return false
					case <-time.After(10 * time.Millisecond):
						lines = s.fetcher.Get(localCtx, line.Pos)
						continue FORLOOP
					}
				}
				break FORLOOP
			}
			lineLen := len(line.Str.Runes)
			if lineLen > 0 {
				parsedLineCount += lineLen / w
			} else {
				parsedLineCount++
			}
			mod := lineLen % w
			if mod != 0 {
				parsedLineCount += 1
			}
			if parsedLineCount > h {
				return false
			}
		}
	}
	if parsedLineCount < h {
		return true
	}
	return false
}
