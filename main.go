package main

import (
	"fmt"
	"io"
	"os"

	flag "github.com/ogier/pflag"
	"github.com/tigrawap/slit/core"
	"github.com/tigrawap/slit/filters"
	"github.com/tigrawap/slit/logging"
)

const VERSION = "1.1.6"

var (
	outPath    string
	follow     bool
	keepChars  int
	filtersOpt string
)

func main() {

	flag.StringVarP(&outPath, "output", "O", "", "Sets stdin cache location, if not set tmp file used, if set file preserved")
	flag.BoolVar(&logging.Config.Enabled, "debug", false, "Enables debug messages, written to /tmp/slit.log")
	flag.BoolVarP(&follow, "follow", "f", false, "Will follow file/stdin")
	showVersion := false
	flag.BoolVar(&showVersion, "version", false, "Print version")
	flag.IntVarP(&keepChars, "keep-chars", "K", 0, "Initial num of chars kept during horizontal scrolling")
	flag.StringVarP(&filtersOpt, "filters", "", "", "Filters file names or inline filters separated by semicolon")
	flag.Parse()

	if showVersion {
		fmt.Println("Slit Version: ", VERSION)
		os.Exit(0)
	}

	stdinStat, _ := os.Stdin.Stat()
	stdoutStat, _ := os.Stdout.Stat()

	var s *core.Slit
	var err error

	if isPipe(stdinStat) && flag.NArg() == 0 {
		if isPipe(stdoutStat) {
			outputToStdout(os.Stdin)
			return
		}
		s, err = core.NewFromStdin()
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

		s, err = core.NewFromFilepath(path)
		exitOnErr(err)
	}

	if filtersOpt != "" {
		initFilters, err := filters.ParseFiltersOpt(filtersOpt)
		exitOnErr(err)
		s.SetFilters(initFilters)
	}

	s.SetOutPath(outPath)
	s.SetFollow(follow)
	s.SetKeepChars(keepChars)

	s.Display()
	s.Shutdown()
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
