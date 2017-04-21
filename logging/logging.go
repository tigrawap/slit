package logging

import (
	"log"
	"os"
	"time"
)

var Config struct {
	Enabled bool
	LogPath string
}

func init() {
	Config.LogPath = "/tmp/debug.log"
}

func Debug(l ...interface{}) {
	if !Config.Enabled {
		return
	}
	f, _ := os.OpenFile(Config.LogPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	defer f.Close()
	log.SetOutput(f)
	log.Println(l)
}

func Timeit(l ...interface{}) func() {
	start := time.Now()
	Debug("->", l)
	return func() {
		Debug("<- ", l, time.Since(start))
	}
}
