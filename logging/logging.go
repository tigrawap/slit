package logging

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

var Config struct {
	Enabled bool
	LogPath string
}

func init() {
	Config.LogPath = filepath.Join(os.TempDir(), "debug.log")
}

func Debug(l ...interface{}) {
	if !Config.Enabled {
		return
	}
	f, err := os.OpenFile(Config.LogPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()
	log.SetOutput(f)
	log.Println(l...)
}

func Timeit(l ...interface{}) func() {
	start := time.Now()
	Debug("->", l)
	return func() {
		Debug("<- ", l, time.Since(start))
	}
}
