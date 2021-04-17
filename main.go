package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/beevik/terminfo"
	"github.com/containerd/console"
)

type TermInfoNotInDir struct {
	TermInfoName    string
	SearchDirectory string
}

func (m *TermInfoNotInDir) Error() string {
	return fmt.Sprintf("Could not find terminfo %q in %q", m.TermInfoName, m.SearchDirectory)
}

type TermInfoNotFound struct {
	TermInfoName      string
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
		TermInfoName:    name,
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
		TermInfoName:      name,
		SearchDirectories: strings.Join(searchDirs, ", "),
	}
}

func LoadFromEnv() (*terminfo.TermInfo, error) {
	return LoadFromName(os.Getenv("TERM"))
}

type Application struct {
	ti     *terminfo.TermInfo

	mu     *sync.Mutex

	input  io.Reader
	output io.Writer
}

func (a *Application) EnterFullScreen() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if smcup, ok := a.ti.GetStringCap("smcup"); ok {
		fmt.Print(smcup)
	}
}

func (a *Application) ExitFullScreen() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if rmcup, ok := a.ti.GetStringCap("rmcup"); ok {
		fmt.Print(rmcup)
	}
}

func (a *Application) Start() {
	// First we set enable cbreak and noecho, this helps us get user input
	// as the user types and doesn't show the input to screen.
	current := console.Current()
	defer current.Reset()
	current.SetRaw()
	current.DisableEcho()

	for {
		r := readInput(a.input)
		if r == 'q' {
			break
		}
	}
}

func NewApplication() *Application {
	ti, err := LoadFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	a := &Application{
		ti:     ti,
		mu:    &sync.Mutex{},
		input:  os.Stdin,
		output: os.Stdout,
	}

	return a
}

func readInput(input io.Reader) rune {
	var buf [256]byte
	numBytes, _ := input.Read(buf[:])

	var runes []rune
	b := buf[:numBytes]

	// Translate input into runes. In most cases we'll receive exactly one
	// rune, but there are cases, particularly when an input method editor is
	// used, where we can receive multiple runes at once.
	for i, w := 0, 0; i < len(b); i += w {
		r, width := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError {
			fmt.Println("Could not decode rune")
		}
		runes = append(runes, r)
		w = width
	}

	if len(runes) == 1 {
		return runes[0]
	}
	return ' '
}

func main() {
	app := NewApplication()
	app.EnterFullScreen()
	app.Start()
	app.ExitFullScreen()
}
