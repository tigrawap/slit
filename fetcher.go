package slit

import (
	"bufio"
	"context"
	"fmt"
	"github.com/tigrawap/slit/ansi"
	"github.com/tigrawap/slit/filters"
	"github.com/tigrawap/slit/logging"
	"io"
	"os"
	"sort"
	"sync"
	"time"
)

type Fetcher struct {
	mLock            sync.RWMutex
	lineMap          map[Offset]LineNo // caches Offset of some lines, meanwhile only last one, when available
	reader           *os.File
	lock             sync.RWMutex
	lineReader       *bufio.Reader
	lineReaderOffset Offset
	lineReaderPos    int
	filters          []*filters.Filter
	highlightedLines []LineNo
	filtersEnabled   bool
}

const (
	POS_UNKNOWN      = -2
	POS_FILTERED_OUT = -1
)

type Offset int64
type LineNo int64

type Pos struct {
	Line   LineNo
	Offset Offset
}

func (l Pos) String() string {
	if l.Line == POS_UNKNOWN {
		return fmt.Sprintf("b%d", l.Offset)
	}
	return fmt.Sprintf("%d", l.Line+1)
}

var POS_NOT_FOUND = Pos{-1, -1}

type PosLine struct {
	b []byte
	Pos
}

type Line struct {
	Str ansi.Astring
	Pos
	Highlighted bool
}

type offsetArr []Offset

func (a offsetArr) Len() int           { return len(a) }
func (a offsetArr) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a offsetArr) Less(i, j int) bool { return a[i] < a[j] }

// Line == -1 if Line is excluded
func (f *Fetcher) filteredLine(l PosLine) Line {
	str := ansi.NewAstring(l.b)
	if len(f.filters) == 0 && len(f.highlightedLines) == 0 {
		return Line{str, l.Pos, false}
	}
	var filterResult filters.FilterResult
	for _, highlighted := range f.highlightedLines {
		logging.Debug(f.highlightedLines, l.Pos.Line)
		if highlighted == l.Pos.Line {
			filterResult = filters.FilterHighlighted
			break
		}
	}

	for _, filter := range f.filters {
		if f.filtersEnabled || filter.Action == filters.FilterHighlight {
			filterResult = filter.TakeAction(str.Runes, filterResult)
		}
	}
	switch filterResult {
	case filters.FilterExcluded:
		return Line{Pos: Pos{Line: POS_FILTERED_OUT, Offset: l.Pos.Offset}}
	case filters.FilterHighlighted:
		return Line{str, l.Pos, true}
	default:
		return Line{str, l.Pos, false}
	}

}

func newFetcher(reader *os.File, ctx context.Context) *Fetcher {
	f := &Fetcher{
		reader:         reader,
		lineMap:        map[Offset]LineNo{0: 0},
		lineReader:     bufio.NewReaderSize(reader, 64*1024),
		filtersEnabled: true,
	}
	go f.gcMap(ctx)
	return f
}

// returns position of next line starting from offset
// If line starts on offset - will return same value as in function argument
// Initially will seek to offset -1 byte, to see if given offset actually is position where line starts
func (f *Fetcher) findLine(offset Offset) (Offset, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	if offset <= 0 {
		return 0, nil
	}
	seekTo := offset - 1
	f.seek(seekTo)
	_, _, err := f.readline()
	if err != nil {
		if err == io.EOF {
			return POS_UNKNOWN, err
		}
		panic(fmt.Sprintf("Unhandled error during findLine: %s", err))
	}

	return f.lineReaderOffset, nil
}

func (f *Fetcher) seek(offset Offset) {
	if offset < 0 {
		panic("Seeking out of bounds")
	}
	if f.lineReaderOffset == offset && f.lineReader != nil {
		return // We are already there
	}
	f.reader.Seek(int64(offset), io.SeekStart)
	f.lineReader = bufio.NewReaderSize(f.reader, 64*1024)
	f.lineReaderOffset = offset
}

//reads and returns one Line, position and error, which can only be io.EOF, otherwise panics
func (f *Fetcher) readline() ([]byte, Offset, error) {
	str, err := f.lineReader.ReadBytes('\n')
	startingOffset := f.lineReaderOffset
	if len(str) > 0 {
		if err == io.EOF && config.stdin && !config.isStdinRead() {
			// Bad idea to remember position when we are not done reading current line
			f.lineReader = nil
			f.lineReaderPos = 0
		} else if err == nil {
			f.lineReaderOffset += Offset(len(str))
		}
	}
	if err == nil {
		return str[:len(str)-1], startingOffset, err //TODO: Handle \r for windows logs?
	} else if err == io.EOF {
		return str, startingOffset, err
	} else {
		panic(err)
	}
}

// Returns 2 channels: for consuming posLines and returning of built Line struct
// lines guranteed to return in received order with filters applied
func (f *Fetcher) lineBuilder(ctx context.Context) (chan<- PosLine, <-chan Line) {
	bufSize := 256
	feeder := make(chan PosLine, bufSize)
	lines := make(chan Line, bufSize)
	buffer := make([]Line, bufSize)
	var ok bool
	var l PosLine
	go func() {
		bLen := 0
		wg := sync.WaitGroup{}
		flush := func() {
			wg.Wait()
			for i := 0; i < bLen; i++ {
				if buffer[i].Pos.Line == POS_FILTERED_OUT {
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
				go func(i int, l PosLine) {
					buffer[i] = f.filteredLine(l)
					wg.Done()
				}(bLen, l)
				bLen++
			}
			if bLen == bufSize {
				flush()
			}
		}
	}()
	return feeder, lines
}

// Get returns channel for yielding lines. Channel will be closed when no more lines to send
// Client should close context when no more lines needed
func (f *Fetcher) Get(ctx context.Context, from Pos) <-chan Line {
	ret := make(chan Line, 500)
	startFrom, err := f.findLine(from.Offset)
	if err == io.EOF {
		close(ret)
		return ret
	}
	if from.Line == POS_UNKNOWN {
		from.Line = f.resolveLine(from.Offset)
	}
	f.lock.Lock()
	f.seek(startFrom)
	var wg sync.WaitGroup
	feeder, lines := f.lineBuilder(ctx)
	wg.Add(1)
	go func(lineNum LineNo) {
		defer wg.Done()
		defer close(feeder)
		for {
			str, pos, err := f.readline()
			if len(str) == 0 && err == io.EOF {
				return
			}
			select {
			case feeder <- PosLine{b: str, Pos: Pos{lineNum, pos}}:
			case <-ctx.Done():
				return
			}
			if err == io.EOF {
				return
			}
			if lineNum != POS_UNKNOWN {
				lineNum++
			}
		}
	}(from.Line)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(ret)
		var l Line
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
	go func() {
		wg.Wait()
		for range lines { //draining lines, will act as lock for linebuilder
		}
		f.lock.Unlock()
	}()
	return ret
}

// Search returns position of next matching search
func (f *Fetcher) Search(ctx context.Context, from Pos, searchFunc filters.SearchFunc) (pos Pos) {
	defer logging.Timeit("Searching")()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	reader := f.Get(ctx, from)
	for l := range reader {
		if searchFunc(l.Str.Runes) != nil {
			return l.Pos
		}
	}
	return POS_NOT_FOUND
}

// Search returns position of next matching search
func (f *Fetcher) SearchHighlighted(ctx context.Context, from Pos) (pos Pos) {
	defer logging.Timeit("Searching")()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	reader := f.Get(ctx, from)
	for l := range reader {
		if l.Highlighted {
			return l.Pos
		}
	}
	return POS_NOT_FOUND
}

// SearchBack returns position of next matching back-search
func (f *Fetcher) SearchBack(ctx context.Context, from Pos, searchFunc filters.SearchFunc) (pos Pos) {
	defer logging.Timeit("Back-Searching")()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	reader := f.GetBack(ctx, from)
	for l := range reader {
		if searchFunc(l.Str.Runes) != nil {
			return l.Pos
		}
	}
	return POS_NOT_FOUND
}

// SearchBack returns position of next matching back-search
func (f *Fetcher) SearchBackHighlighted(ctx context.Context, from Pos) (pos Pos) {
	defer logging.Timeit("Back-Searching")()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	reader := f.GetBack(ctx, from)
	for l := range reader {
		if l.Highlighted {
			return l.Pos
		}
	}
	return POS_NOT_FOUND
}

func (f *Fetcher) advanceLines(from Pos) PosLine {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.seek(from.Offset)
	i := from.Line
	ret := PosLine{Pos: from}
	for {
		str, offset, err := f.readline()
		if err == io.EOF && len(str) == 0 {
			return ret
		}
		ret.b, ret.Offset, ret.Line = str, offset, i
		if err == io.EOF {
			ret.b = ret.b[:len(ret.b)-1] // got no \n stripped, hacky, used only for saving map and it lacks error information
			return ret
		}
		if i >= 3500+from.Line {
			return ret
		}
		i++
	}
}

func (f *Fetcher) lastOffset() Offset {
	stat, err := f.reader.Stat()
	if err != nil {
		logging.Debug(fmt.Sprintf("Error retrieving stat from file: %s", err))
		return Offset(0)
	}
	if stat.Size() == 0 {
		return Offset(0)
	}
	return Offset(stat.Size() - 1)
}

const fetchBackStep = 64 * 1024

func (f *Fetcher) GetBack(ctx context.Context, fromPos Pos) <-chan Line {
	//f.lock.Lock()
	ret := make(chan Line, 500)
	tmpLines := make([]PosLine, fetchBackStep/20) // Presuming, that average line > 20 cols. Otherwise - append will increase underlying array
	var l Line
	var lineOffset Offset
	var err error
	// Determine if seeking from the end
	if fromPos.Line == POS_UNKNOWN {
		fromPos.Line = f.resolveLine(fromPos.Offset)
	}
	lineAssign := fromPos.Line
	from := fromPos.Offset
	go func(lineAssign LineNo) {
		//defer f.lock.Unlock()
		defer close(ret)
		for {
			if from < 0 {
				return
			}
			tmpLines = tmpLines[:0]
			lineOffset, err = f.findLine(from - fetchBackStep)
			if err == io.EOF || lineOffset > fromPos.Offset {
				// line longer then 64k
				from = from - fetchBackStep
				continue
			}
			f.lock.Lock()
			f.seek(lineOffset)
			for {
				str, pos, err := f.readline()
				if len(str) == 0 && err == io.EOF {
					break
				}
				if pos > from {
					break
				}
				tmpLines = append(tmpLines, PosLine{str, Pos{POS_UNKNOWN, pos}})
				// Ignoring context cancel here, reading is fast enough to exclude expensive channel checks
				if err == io.EOF {
					break
				}
			}
			for i := len(tmpLines) - 1; i >= 0; i-- {
				if fromPos.Line > 0 {
					tmpLines[i].Line = lineAssign
					lineAssign--
					//logging.Debug("assigned line", tmpLines[i].Line)
				}
				l = f.filteredLine(tmpLines[i])
				if l.Pos.Line == POS_FILTERED_OUT { //filtered out
					continue
				}
				select {
				case ret <- l: //TODO: paralellize
				case <-ctx.Done():
					f.lock.Unlock()
					return
				}
			}
			from = from - fetchBackStep
			f.lock.Unlock()
		}
	}(lineAssign)
	return ret
}

func (f *Fetcher) gcMap(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
			f.mLock.RLock()
			size := len(f.lineMap)
			f.mLock.RUnlock()
			if size < 1000 {
				continue
			}
			f.mLock.Lock()
			keys := make(offsetArr, len(f.lineMap))

			i := 0
			for k := range f.lineMap {
				keys[i] = k
				i++
			}
			sort.Sort(keys)
			toDelete := keys[:len(keys)-1000]
			for _, k := range toDelete {
				delete(f.lineMap, k)
			}
			f.mLock.Unlock()
		}
	}
}

func (f *Fetcher) updateMap(pos PosLine) {
	f.mLock.Lock()
	f.lineMap[pos.Offset+Offset(len(pos.b))] = pos.Line
	f.lineMap[pos.Offset] = pos.Line
	f.mLock.Unlock()
}

func (f *Fetcher) resolveLine(o Offset) LineNo {
	f.mLock.Lock()
	defer f.mLock.Unlock()
	if lineNum, ok := f.lineMap[o]; ok {
		return lineNum
	}
	return POS_UNKNOWN
}

func (f *Fetcher) removeLastFilter() bool {
	if len(f.filters) > 0 {
		f.filters = f.filters[:len(f.filters)-1]
		return true
	}
	return false
}
func (f *Fetcher) toggleHighlight(line LineNo) {
	for i, highlighted := range f.highlightedLines {
		if highlighted == line {
			copy(f.highlightedLines[i:], f.highlightedLines[i+1:])
			f.highlightedLines = f.highlightedLines[:len(f.highlightedLines)-1]
			return
		}
	}
	f.highlightedLines = append(f.highlightedLines, line)
	sort.Slice(f.highlightedLines, func(i, j int) bool {
		return f.highlightedLines[i] < f.highlightedLines[j]
	})
}
