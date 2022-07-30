## **Slit** - a modern $PAGER for noisy logs


The goal is to get **more** from logs than **most** other pagers can - and to do so in **less** time.


Slit supports opening a single file (for now), or reading input from stdin.
Slit is runs in terminal mode, writing directly to the screen, without cluttering the terminal buffer with all the logs you are reading.

### Live demo
![Live demo](https://habrastorage.org/files/a64/704/82b/a6470482b6b04f548998b57df088ebb6.gif)

### Installation
The best way is to get **Go** on your system and compile yourself. It's easier than it sounds:
- Download and install from https://golang.org/dl/  
- Make sure that you have `$GOPATH/bin` in your `PATH`.
- `go get github.com/tigrawap/slit/cmd/slit`
- Done!

If you prefer pre-built binaries, head over to the releases page - https://github.com/tigrawap/slit/releases.
Keep in mind, however, that they might be some commits behind master branch.
  

### Key bindings:  

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

### Search modes
Both search and filters currently support the `CaseSensitive` and `RegEx` modes.
To switch between modes press `CTRL + /` in search/filter input.

***TODO**: History does not preserve mode of previous searches. Will be improved soon*

**Note**: For case-insensitive search in `RegEx*` mode use `(?i)cOnDiTiOn`  
**TODO:** This will be replaced with a separate toggle in the future  

### Command line arguments
- `--always-term` - Always opens in term mode, even if output is short
- `--debug` - Enables debug messages, written to /tmp/slit.log
- `--filters=nginx_php_errors` - Specifies path to the file containing predefined filters or inline filters separated by semicolon *(see ["Filters"](#filters))*
- `--follow -f` - Follow file/stdin. All filters are applied to new data
When navigating up from the end, following will be stopped and resumed upon navigating to the end <kbd>shift+g</kbd>, or just by scrolling down till the end
- `--keep-chars=10`, `-K 10` - Predefines number of kept chars *(see K in ["Key bindings"](#key-bindings))*
- `--output=/output/path`, `-O /output/path` - Sets stdin cache location, if not set tmp file used, if set file preserved
- `--short-stdin-timeout=10000` - Sets maximum duration (ms) to wait for delayed short stdin
- `--version` - Displays version

### Highlighting
- ``` ` ``` - (Backtick) Mark top line for highlighting (i.e. will be shown no matter what other filters are active)
- ``` ~ ``` - Highlight filter, i.e. search and highlight everything that matches
- `h` - Move to next highlighted line
- `H` - Move to previous highlighted line
- `ctrl+h` - Remove all highlights
- `=` - Removes filters only. Does not remove highlights via `~`

### Filters

- Inclusive(&): Will keep only the lines that match the pattern AND are included by previous filters
- Exclusive(-): Filters out lines that match the pattern
- Appending(+): Filters in lines that match the pattern, even if they were excluded by previous filters

Filters can be chained - the first 'append' filter (if it is the first to be used) will work as an inclusive filter.
When adding filters the active line position (at the top of the screen) will remain the same (…as possible).

Chaining of filters gives you the ability to filter out all the 'noise' dynamically, and get to what you're actually looking for.

Imagine: you have a huge log file with hundreds of thousands of lines from multiple threads. 
All that you're interested in are logs from "Thread-10, "MainThread"; you're not interested in "send" and "receive" messages. 
In addition, you want to see "Exception"s, even if they're on lines that were excluded by previous filters.

The following chain of filters will output the expected result:

```
&Thread-10
+MainThread
-receive
-send
+Exception
```

#### Filter files

You can save the lines above to a separate file and specify its name using the command line argument `--filters`.

Notes:
- Empty lines are ignored
- Leading spaces before a filter sign (`&`, `+` or `-`) are ignored
- Trailing spaces (if present) are also part of the search string
- All filters are case-sensitive

#### Inline filters

You can also add semicolon separated inline filters in the argument `--filters` with or without filter file names (the last  
should also be separated by semicolon). e.g.:

```
$ slit --filters="nginx_php_errors;-debug.php;+WARN" /var/log/nginx/error.log
```

…will apply filters from the file "nginx\_php\_errors", then remove lines without substring "debug.php" and add the lines  
with substring "WARN".

Notes:
- The first non-whitespace character should be a valid filter sign *(see ["Filters"](#filters))*, otherwise this option item will be treated as a file name
- Leading semicolon characters are ignored
- All other rules are the same as for filters in the separate files *(see ["Filter files"](#filter-files))*

#### Filter TODOs:

- Complex include/exclude filters, which will allow: `(DEBUG OR INFO) AND NOT (send OR receive OR "pipe closed")`
- Filters menu for overviewing current filters, removal, reordering or disable some temporary

MIT License
