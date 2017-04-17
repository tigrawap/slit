package main

import (
	"github.com/tigrawap/slit/ansi"
	"context"
	"io"
	"os"
	"bufio"
	"sync"
	"github.com/tigrawap/slit/logging"
	"github.com/tigrawap/slit/runes"
)

type fetcher struct {
	lock           sync.Mutex
	lineMap        map[int]int // caches offset of every line
	reader         io.ReadSeeker
	lineReader     *bufio.Reader
	lineReaderPos  int
	totalLines     int //Total lines currently fetched
	filters        []filter
	filtersEnabled bool
}

// Pos == -1 if line is excluded
func (f *fetcher) filteredLine(l posLine) (line) {
	str := ansi.NewAstring(l.b)
	if len(f.filters) == 0 || !f.filtersEnabled {
		return line{str, l.pos}
	}
	var action filterAction
	for _, filter := range f.filters {
		action = filter.takeAction(str.Runes, action)
	}
	switch action {
	case filterExclude:
		return line{Pos: -1}
	default:
		return line{str, l.pos}
	}

}

func newFetcher(reader io.ReadSeeker) *fetcher {
	return &fetcher{
		reader:         reader,
		lineMap:        map[int]int{0: 0},
		lineReader:     bufio.NewReaderSize(reader, 64*1024),
		filtersEnabled: true,
	}
}

type line struct {
	Str ansi.Astring
	Pos int
}

//seeks to line
func (f *fetcher) seek(to int) {
	if to < 0 {
		to = 0
	}
	seekTo := to
	logging.Debug("Seeking to line ", seekTo)
	fpos, ok := f.lineMap[seekTo]
	if !ok {
		if f.totalLines-1 >= seekTo {
			if f.totalLines != 0 {
				fpos = f.lineMap[f.totalLines-1] // Will search from known end
				seekTo = f.totalLines - 1
			} else {
				fpos = 0
				seekTo = 0 // Will search from the beginning...it's a very first call
			}
		}
	}

	if f.lineReaderPos == seekTo && f.lineReader != nil {
		return // We are already there
	}

	f.reader.Seek(int64(fpos), os.SEEK_SET)
	f.lineReader = bufio.NewReaderSize(f.reader, 64*1024)
	f.lineReaderPos = seekTo
	if seekTo == to {
		return
	}
	for {
		_, pos, err := f.readline()
		if pos == seekTo {
			break
		}
		if err == io.EOF {
			panic("Seeking out of bounds")
		}
	}
	return
}

//reads and returns one line, position and error, which can only be io.EOF
func (f *fetcher) readline() ([]byte, int, error) {
	str, err := f.lineReader.ReadBytes('\n')
	pos := f.lineReaderPos
	if err == nil {
		f.lineReaderPos += 1
		if _, ok := f.lineMap[f.lineReaderPos]; !ok {
			f.lineMap[f.lineReaderPos] = f.lineMap[f.lineReaderPos-1] + len(str)
			f.totalLines += 1
		}
		return str[:len(str)-1], pos, err //TODO: Handle \r for windows logs?
	}
	if err == io.EOF {
		return str, pos, err
	} else {
		panic(err)
	}
}

// Returns 2 channels: for consuming posLines and returning of built line struct
// lines guranteed to return in received order with filters applied
func (f *fetcher) lineBuilder(ctx context.Context) (chan<- posLine, <-chan line) {
	bufSize := 256
	feeder := make(chan posLine, bufSize)
	lines := make(chan line, bufSize)
	buffer := make([]line, bufSize)
	var ok bool
	var l posLine
	go func() {
		bLen := 0
		wg := sync.WaitGroup{}
		flush := func() {
			wg.Wait()
			for i := 0; i < bLen; i++ {
				if buffer[i].Pos == -1 {
					continue //filtered out
				}
				select {
				case lines <- buffer[i]:
				case <-ctx.Done():
					break
				}
			}
			bLen = 0
			wg = sync.WaitGroup{}
		}
		defer close(lines)
		defer flush()
		for {
			select {
			case <-ctx.Done():
				return
			case l, ok = <-feeder:
				if !ok { //feeder closed
					return
				}
				wg.Add(1)
				go func(i int, l posLine) {
					buffer[i] = f.filteredLine(l)
					wg.Done()
				}(bLen, l)
				bLen += 1
			}
			if bLen == bufSize {
				flush()
			}
		}
	}()
	return feeder, lines
}

// Returns channel for yielding lines. Channel will be closed when no more lines to send
// Client should close context when no more lines needed
func (f *fetcher) Get(ctx context.Context, from int) <-chan line {
	f.lock.Lock()
	f.seek(from)
	ret := make(chan line, 500)
	feeder, lines := f.lineBuilder(ctx)
	go func() {
		defer close(feeder)
		for {
			str, pos, err := f.readline()
			if len(str) == 0 && err == io.EOF {
				return
			}
			select {
			case feeder <- posLine{str, pos}:
			case <-ctx.Done():
				return
			}
			if err == io.EOF {
				return
			}
		}
	}()
	go func() {
		defer close(ret)
		defer f.lock.Unlock()
		var l line
		var ok bool
		for {
			select {
			case l, ok = <-lines:
				if !ok {
					return
				}
				select {
				case ret <- l:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return ret
}

// Returns position of next matching search
func (f *fetcher) Search(ctx context.Context, from int, sub []rune) (index int) {
	defer logging.Timeit("Searching for ", string(sub))()
	index = -1
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	reader := f.Get(ctx, from)
	for l := range reader {
		if runes.Index(l.Str.Runes, sub) != -1 {
			return l.Pos
		}
	}
	return
}

// Returns position of next matching back-search
func (f *fetcher) SearchBack(ctx context.Context, from int, sub []rune) (index int) {
	defer logging.Timeit("Back-Searching for ", string(sub))()
	index = -1
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	reader := f.GetBack(ctx, from)
	for l := range reader {
		if runes.Index(l.Str.Runes, sub) != -1 {
			return l.Pos
		}
	}
	return

}

type posLine struct {
	b   []byte
	pos int
}

func (f *fetcher) lastLine() int {
	f.seek(f.totalLines - 1)
	for {
		_, _, err := f.readline()
		if err == io.EOF {
			return f.totalLines - 1
		}
	}
	//}
	return f.totalLines - 1
}

func (f *fetcher) GetBack(ctx context.Context, from int) <-chan line {
	f.lock.Lock()
	ret := make(chan line, 500)
	tmpLines := make([]posLine, 100)
	var l line
	go func() {
		defer f.lock.Unlock()
		defer close(ret)
		for {
			if from == 0 {
				return
			}
			tmpLines = tmpLines[:0]
			f.seek(from - 100)
			for {
				str, pos, err := f.readline()
				if len(str) == 0 && err == io.EOF {
					break
				}
				if pos >= from {
					break
				}
				tmpLines = append(tmpLines, posLine{str, pos})
				// Ignoring channel close here, reading 100 is fast enough to exclude expensive channel checks
				if err == io.EOF {
					break
				}
			}
			for i := len(tmpLines) - 1; i >= 0; i-- {
				l = f.filteredLine(tmpLines[i])
				if l.Pos == -1 { //filtered out
					continue
				}
				select {
				case ret <- l: //TODO: paralellize
				case <-ctx.Done():
					return
				}
			}
			from = from - 100
			if from < 0 {
				from = 0
			}
		}
	}()
	return ret
}
