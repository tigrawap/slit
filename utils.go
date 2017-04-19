package main

import "os"

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func min64(a, b int64) int64 {
	if a > b {
		return b
	}
	return b
}


func openRewrite(path string) *os.File{
	var err error
	var f *os.File
	openFile := func() error {
		f, err = os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		return err
	}
	if err = openFile(); os.IsExist(err) {
		os.Remove(path)
		err = openFile()
	}
	check(err)
	return f

}