## **Slit** - modern $PAGER for noisy logs


The goal is to get **more** from logs than **most** of other pagers can. And to do so in **less** time.


Slit supports opening a single file (for now), or reading input from stdin.
Slit is runs in terminal mode, writing directly to the screen, without cluttering the terminal buffer by all the logs you are reading.

### Live demo
![Live demo](https://habrastorage.org/files/a64/704/82b/a6470482b6b04f548998b57df088ebb6.gif)

### Installation
The best way is to get **Go** on your system and compile yourself. It's easier then it sounds:
- download and install from https://golang.org/dl/  
- make sure that you got `$GOPATH/bin` in your `PATH`.
- `go get github.com/tigrawap/slit/cmd/slit`
- done!

If you prefer pre-built binaries, head over to the releases page - https://github.com/tigrawap/slit/releases.
Keep in mind however they might be some commits behind master branch.
  

### Key Bindings:  

##### Search/Filters
- `/` - Forward search  
- `?` - Backsearch  
- `n` - Next match
- `N` - Previous match
- `CTRL + /` - Switch between `CaseSensitive` search and `RegEx`
- `&` - Filter: intersect
- `-` - Filter: exclude
- `+` - Filter: union
- `=` - Remove all filters
- `U` - Removes last filter
- `C` - Stands for "Context", switches off/on all filters, helpful to get context of current line (which is the first line, at the top of the screen)

##### Navigation
- `f`, `PageDown`, `Space`, `CTRL + F` - Page Down
- `CTRL + D` - Half page down
- `b`, `PageUp`, `CTRL + B` - Page Up
- `CTRL + U` - Half page up
- `g`, `Home` - Go to first line
- `G`, `End` - Go to last line
- `Arrow down`, `j` - Move one line down
- `Arrow up`, `k` - Move one line up
- `Arrow left`, `Arrow right` - Scroll horizontally
- `<`, `>` - Precise horizontal scrolling, 1 character a time
   
##### Misc
- `K` - Keep N first characters(usually containing timestamp) when navigating horizontally  
    Up/Down arrows during K-mode will adjust N of kept chars 
- `W` - Wrap/Unwrap lines
- `CTRL + S` - Save filtered version to file (will prompt for filepath)
- `q` - quit

### Search Modes
Both search and filters currently support the `CaseSensitive` and `RegEx` modes.
To switch between modes press `CTRL + /` in search/filter input.

*TODO: History does not preserve mode of previous searches. Will be improved soon*

**Note**: For case-insensitive search in **RegEx** use `(?i)cOnDiTiOn`  
**TODO:** This will be replaced with separate toggle in the future  

### Command line arguments
- `--always-term` - Always opens in term mode, even if output is short
- `--debug` - Enables debug messages, written to /tmp/slit.log
- `--filters=nginx_php_errors` - Specifies path to the file containing predefined filters or inline filters separated by semicolon *(see "Filters" section)*
- `--follow -f` - Follow file/stdin. All filters are applied to new data
When navigating up from the end, following will be stopped and resumed on navigating to the end(shift+g) or just by scrolling down till the end
- `--keep-chars=10`, `-K 10` - Predefines number of kept chars *(see K in key bindings)*
- `--output=/output/path`, `-O /output/path` - Sets stdin cache location, if not set tmp file used, if set file preserved
- `--short-stdin-timeout=10000` - Sets maximum duration (ms) to wait for delayed short stdin
- `--version` - Displays version

### Highlighting
- ``` ` ``` - (Backtick) Mark top line for highlighting (i.e will be shown no matter what are other filters
- ``` ~ ``` - Highlight filter. I.e search and highlight everything that matches

### Filters

- Inclusive(&): Will keep only lines that match the pattern AND included by previous filters
- Exclusive(-): Filters out lines that match the pattern
- Appending(+): Filters in lines that match pattern, even if they were excluded by previous filters

Filters can be chained - The first 'append' filter (if it is the first to be used) will work as inclusive filter.
When adding filters the active line position (at top of screen) will remain the same (as possible).

Chaining of filters gives ability to filter out all the 'noise' dynamically, and get to what you're actually looking for.

Imagine you have huge log file with hundreds of thousands of lines from multiple threads.      
And all that you are interested in are logs from "Thread-10, "MainThread", not interested in "send" and "receive" messages  
In addition, you want to see "Exception", even if it is on line that were excluded by previous filters.

The following chain of filters will output the expected result:

```
&Thread-10
+MainThread
-receive
-send
+Exception
```

#### Filter files

You can save the lines above to the separate file and specify its name in the command line argument `--filters`.

Notes:
- empty lines are ignored
- leading spaces before filter sign (`&`, `+` or `-`) are ignored
- trailing spaces (if present) are also part of the search string
- all filters are case sensitive

#### Inline filters

You can also add semicolon separated inline filters in the argument `--filters` with or without filter file names (the last  
should also be separated by semicolon). E.g.

```
$ slit --filters="nginx_php_errors;-debug.php;+WARN" /var/log/nginx/error.log
```

will apply filters from file "nginx\_php\_errors" and then remove lines without substring "debug.php" and add the lines  
with substring "WARN".

Notes:
- the first non-whitespace character should be a valid filter sign (see section "Filters"), otherwise this option item will be treated as a file name
- leading semicolon characters are ignored
- all other rules are the same as for filters in the separate files (see section "Filter files")

#### Filter TODOs:

- Complex include/exclude filters, that will allow: (DEBUG OR INFO) AND NOT (send OR receive OR "pipe closed") 
- Filters menu for overviewing current filters, removal, reordering or disable some temporary

MIT License
