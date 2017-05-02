package main

import (
	"errors"
	"os"
)

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

func openRewrite(path string) *os.File {
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

func validateRegularFile(filename string) error {
	fi, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return errors.New(filename + ": No such file or directory")
	} else if os.IsPermission(err) {
		return errors.New(filename + ": Permission denied")
	} else if err != nil {
		return err
	}
	switch fmode := fi.Mode(); {
	case fmode.IsDir():
		return errors.New(filename + " is a directory")
	case !fmode.IsRegular():
		return errors.New(filename + " is not a regular file")
	}
	return nil
}
