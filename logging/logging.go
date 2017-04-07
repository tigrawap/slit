package logging

import (
	"os"
	"log"
	"time"
)

func Debug(l ...interface{}) {
	f, _ := os.OpenFile("err.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	defer f.Close()

	log.SetOutput(f)
	log.Println(l)
}

func Timeit(name string) func(){
	start := time.Now()
	Debug("->", name)
	return func(){
		Debug("<- ", name, time.Since(start))
	}
}