## **Slit** - modern $PAGER for noisy logs


The goal is to get **more** from logs than **most** of other pagers can. And to do so in **less** time.


Slit supports opening a single file (for now), or reading input from stdin.
Slit is **not** readline-compatible; it runs in terminal mode, which means your terminal doesn't get clogged by all the logs you are reading. This also means you shouldn't try to pipe its output into other commands, as you might have done with `less`.

### Live demo
![Live demo](https://habrastorage.org/files/a64/704/82b/a6470482b6b04f548998b57df088ebb6.gif)

### Installation
The best way is to get **Go** on your system and compile yourself. It's easier then it sounds:
- download and install from https://golang.org/dl/  
- make sure that you got `$GOPATH/bin` in your `PATH`.
- `go get github.com/tigrawap/slit`
- done!

If you prefer pre-built binaries, head over to the releases page - https://github.com/tigrawap/slit/releases.
Keep in mind however they might be some commits behind master branch.
  

### Key Bindings:  

- **/** - forward search  
- **?** - backsearch  
- **n** - next match
- **N** - previous match
- **CTRL + /** - Switch between `CaseSensitive` search and `RegEx`
- **&** - Filter: intersect
- **-** - Filter: exclude
- **+** - Filter: union
- **=** - Remove all filters
- **U** - Removes last filter
- **C** - Stands for "Context", switches off/on all filters, helpful to get context of current line (which is the first line, at the top of the screen)
- **W** - Wrap/Unwrap lines
- **f/PageDown/Space** - Page Down
- **b/PageUp** - Page Up
- **g/Home** - Go to first line
- **G/End** - Go to last line
- **K** - Keep N first characters(usually containing timestamp) when navigating horizontally  
    Up/Down arrows during K-mode will adjust N of kept chars 
- **q** - quit

### Search Modes
Both search and filters currently support the `CaseSensitive` and `RegEx` modes.
To switch between modes press "CTRL + /" in search/filter input.

*TODO: History does not preserve mode of previous searches. Will be improved soon*

**Note**: For case-insensitive search in **RegEx** use `(?i)cOnDiTiOn`  
**TODO:** This will be replaced with separate toggle in the future  

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

#### Filter TODOs:
- Complex include/exclude filters, that will allow: (DEBUG OR INFO) AND NOT (send OR receive OR "pipe closed") 
- Filters menu for overviewing current filters, removal, reordering or disable some temporary

MIT License
