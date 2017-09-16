package utils

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
)

func Check(e error) {
	if e != nil {
		panic(e)
	}
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func Min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

func Max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func Min64(a, b int64) int64 {
	if a > b {
		return b
	}
	return b
}

func OpenRewrite(path string) *os.File {
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
	Check(err)
	return f
}

func ValidateRegularFile(filename string) error {
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

func GetHomeDir() string {
	currentUser, err := user.Current()
	var homedir string
	if err != nil {
		homedir = os.Getenv("HOME")
		if homedir == "" {
			homedir = os.TempDir()
		}
	} else {
		homedir = currentUser.HomeDir
	}
	return homedir
}

func ExpandHomePath(path string) string {
	if len(path) == 0 || path[0] != '~' {
		return path
	}

	return filepath.Join(GetHomeDir(), path[1:])
}
