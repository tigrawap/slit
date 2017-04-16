##**Slit** - modern $PAGER for noisy logs


The goal is to get **more** from logs, then **most** of other pagers can do. And do so in **less** time. Basically slit is a verb, applied to logs


Supports opening single(for now) file or retrieving input from stdin  
Output is not readline analogue, but term mode, so your terminal won't get clogged by all the logs you are reading

###Keybindings:  

- **/** - forward search  
- **?** - backsearch  
- **&** - Inclusive filter
- **-** - Exclusive filter
- **+** - Appending filter
- **=** - Remove all filters
- **W** - Wrap/Unwrap lines
- **f/PageDown** - Page Down
- **b/PageUp** - Page Up
- **g/Home** - Go to first line
- **G/End** - Go to last line
- **q** - quit

#### Keybindings TODOs:
- Switch filters - disable all of them temorary to get context of current line and then reenable them

### Filters
- Inclusive(&): Will keep only lines that match the pattern AND included by previous filters
- Exclusive: Filters out lines that match the pattern  
- Appending: Filters in lines that match pattern, even if they were excluded by previous filters  


Filters can be chained, first append filter(if first to be used) will work as inclusive filter.   
When adding filters position line(first on screen) will be the same(if possible) as before adding filter

Chaining of filters gives ability to filter out all the noise dynamically and get what actually is needed

Imagine you have huge log file with hundreds of thousands of lines from multiple threads.      
And all that you are interested in are logs from "Thread-10, "MainThread", not interested in "send" and "receive" messages  
In addition, you want to see Exception. Even if it occured during previously excluded actions


Such chain of filters will give required result:  

```
&Thread-10  
+MainThread  
-receive  
-send  
+Exception

```

####Filter TODOs:
- Complex include/exclude filters, that will allow: (DEBUG OR INFO) AND NOT (send OR recieve OR "pipe closed") 
- Filters menu for overviewing current filters, removal, reordering or disable some temporary

MIT License



