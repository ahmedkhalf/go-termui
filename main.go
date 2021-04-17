package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/beevik/terminfo"
)

type TermInfoNotInDir struct{
	TermInfoName string
	SearchDirectory string
}

func (m *TermInfoNotInDir) Error() string {
    return fmt.Sprintf("Could not find terminfo %q in %q", m.TermInfoName, m.SearchDirectory)
}

type TermInfoNotFound struct{
	TermInfoName string
	SearchDirectories string
}

func (m *TermInfoNotFound) Error() string {
    return fmt.Sprintf("Could not find terminfo %q in any of %q", m.TermInfoName, m.SearchDirectories)
}

func LoadFromFile(file string) (*terminfo.TermInfo, error) {
	ansifile, err := os.Open(file)
	if err != nil {
		log.Fatal(err)
	}
	defer ansifile.Close()

	ansireader := bufio.NewReader(ansifile)

	ti, err := terminfo.Read(ansireader)
	return ti, err
}

func LoadFromDir(dir string, name string) (*terminfo.TermInfo, error) {
	rootFile := filepath.Join(dir, name)
	nestedFile := filepath.Join(dir, string(name[0]), name)

	if _, err := os.Stat(rootFile); err == nil {
		return LoadFromFile(rootFile)
	} else if os.IsNotExist(err) {
		if _, err := os.Stat(nestedFile); err == nil {
			return LoadFromFile(nestedFile)
		}
	}

	return nil, &TermInfoNotInDir{
		TermInfoName: name,
		SearchDirectory: dir,
	}
}

func LoadFromName(name string) (*terminfo.TermInfo, error) {
	var searchDirs []string

	// $TERMINFO
	if dir := os.Getenv("TERMINFO"); dir != "" {
		searchDirs = append(searchDirs, dir)
	}

	// $HOME/.terminfo
	u, err := user.Current()
	if err != nil {
		return nil, err
	}
	searchDirs = append(searchDirs, filepath.Join(u.HomeDir, ".terminfo"))

	// $TERMINFO_DIRS
	if dirs := os.Getenv("TERMINFO_DIRS"); dirs != "" {
		searchDirs = append(searchDirs, strings.Split(dirs, ":")...)
	}

	searchDirs = append(searchDirs, "/etc/terminfo", "/lib/terminfo", "/usr/share/terminfo")

	var notindir *TermInfoNotInDir
	for _, dir := range searchDirs {
		ti, err := LoadFromDir(dir, name)
		if err != nil {
			if ok := errors.As(err, &notindir); !ok {
				return ti, err
			}
		} else {
			return ti, err
		}
	}

	return nil, &TermInfoNotFound{
		TermInfoName: name,
		SearchDirectories: strings.Join(searchDirs, ", "),
	}
}

func LoadFromEnv() (*terminfo.TermInfo, error) {
	return LoadFromName(os.Getenv("TERM"))
}

func main() {
	ti, err := LoadFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	smcup, ok := ti.GetStringCap("smcup") // Enter FullScreen
	if ok {
		fmt.Print(smcup)
	}

	time.Sleep(2 * time.Second)

	rmcup, ok := ti.GetStringCap("rmcup") // Exit FullScreen
	if ok {
		fmt.Print(rmcup)
	}
}
