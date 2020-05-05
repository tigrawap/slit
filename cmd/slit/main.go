package main

import (
	"fmt"
	"io"
	"os"

	"context"
	"time"

	flag "github.com/ogier/pflag"
	"github.com/tigrawap/slit"
	"github.com/tigrawap/slit/filters"
	"github.com/tigrawap/slit/logging"
)

const VERSION = "1.3.0"

var (
	outPath    string
	follow     bool
	keepChars  int
	filtersOpt string
)

func main() {

	ctx := context.Background()

	flag.StringVarP(&outPath, "output", "O", "", "Sets stdin cache location, if not set tmp file used, if set file preserved")
	flag.BoolVar(&logging.Config.Enabled, "debug", false, "Enables debug messages, written to /tmp/slit.log")
	flag.BoolVarP(&follow, "follow", "f", false, "Will follow file/stdin")
	showVersion := false
	alwaysTermMode := false
	waitForShortStdin := 10000
	flag.BoolVar(&showVersion, "version", false, "Print version")
	flag.BoolVar(&alwaysTermMode, "always-term", false, "Always opens in term mode, even if output is short")
	flag.IntVarP(&keepChars, "keep-chars", "K", 0, "Initial num of chars kept during horizontal scrolling")
	flag.IntVar(&waitForShortStdin, "short-stdin-timeout", 10000, "Maximum duration(ms) to wait for delayed short stdin(won't delay long stdin)")
	flag.StringVarP(&filtersOpt, "filters", "", "", "Filters file names or inline filters separated by semicolon")
	flag.Parse()

	if showVersion {
		fmt.Println("Slit Version: ", VERSION)
		os.Exit(0)
	}

	stdinStat, _ := os.Stdin.Stat()
	stdoutStat, _ := os.Stdout.Stat()

	var s *slit.Slit
	var err error

	if isPipe(stdinStat) && flag.NArg() == 0 {
		if isPipe(stdoutStat) {
			outputToStdout(os.Stdin)
			return
		}
		s, err = slit.NewFromStdin()
		exitOnErr(err)
	} else {
		if flag.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "Only viewing of one file or from STDIN is supported")
			os.Exit(1)
		}
		path := flag.Arg(0)

		if isPipe(stdoutStat) {
			f, err := os.Open(path)
			exitOnErr(err)
			outputToStdout(f)
			return
		}

		s, err = slit.NewFromFilepath(path)
		exitOnErr(err)
	}

	defer s.Shutdown()

	if !alwaysTermMode && tryDirectOutputIfShort(s, ctx, waitForShortStdin) {
		return
	}

	if filtersOpt != "" {
		initFilters, err := filters.ParseFiltersOpt(filtersOpt)
		exitOnErr(err)
		s.SetFilters(initFilters)
	}

	s.SetOutPath(outPath) // TODO: This is not really used right now, NewFromStdin uses config before it is set here
	// Probably should pass config to all slit constructors, with sane defaults
	s.SetFollow(follow)
	s.SetKeepChars(keepChars)

	s.Display()
}

func tryDirectOutputIfShort(s *slit.Slit, ctx context.Context, durationMs int) bool {
	localCtx, cancel := context.WithTimeout(ctx, time.Duration(durationMs)*time.Millisecond)
	defer cancel()
	if s.CanFitDisplay(localCtx) {
		file := s.GetFile()
		file.Seek(0, io.SeekStart)
		outputToStdout(file)
		return true
	}
	return false
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
