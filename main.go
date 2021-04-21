package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"unicode/utf8"

	"github.com/containerd/console"
	"github.com/xo/terminfo"
)

type Event interface{}

type ResizeEvent struct {
	Width  uint16
	Height uint16
}

type KeyEvent struct {
	r rune
}

type Screen struct {
	ti      *terminfo.Terminfo
	Console console.Console

	mu *sync.Mutex

	input  io.Reader
	output io.Writer

	events chan Event
}

func (s *Screen) EnterFullScreen() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ti.Fprintf(s.output, terminfo.EnterCaMode)
}

func (s *Screen) ExitFullScreen() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ti.Fprintf(s.output, terminfo.ExitCaMode)
}

func (s *Screen) Goto(y, x int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ti.Fprintf(s.output, terminfo.CursorAddress, y, x)
}

func (s *Screen) Start() {
	// First we set enable raw mode and noecho, this helps us get user input
	// as the user types and doesn't show the input to screen.
	s.Console = console.Current()
	s.Console.SetRaw()
	s.Console.DisableEcho()

	s.events = make(chan Event)

	// Resize Loop
	go func() {
		// Initial Size
		size, _ := s.Console.Size()
		s.events <- ResizeEvent{Width: size.Width, Height: size.Height}

		// Resize
		resizeSignal := make(chan os.Signal, 1)
		signal.Notify(resizeSignal, syscall.SIGWINCH)
		for {
			<-resizeSignal
			size, _ := s.Console.Size()
			s.events <- ResizeEvent{Width: size.Width, Height: size.Height}
		}
	}()

	// Input Loop
	go func() {
		for {
			r := readInput(s.input)
			s.events <- KeyEvent{r}
		}
	}()
}

func (s *Screen) GetEvent() Event {
	select {
	case ev := <-s.events:
		return ev
	}
}

func (s *Screen) End() {
	if s.Console != nil {
		s.Console.Reset()
	}
}

func NewScreen() *Screen {
	ti, err := terminfo.LoadFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	s := &Screen{
		ti:     ti,
		mu:     &sync.Mutex{},
		input:  os.Stdin,
		output: os.Stdout,
	}

	return s
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
	scr := NewScreen()

	scr.EnterFullScreen()

	scr.Goto(1, 3)
	fmt.Fprint(scr.output, "Test")

	defer scr.ExitFullScreen()

	scr.Start()
	defer scr.End()

mainloop:
	for {
		switch ev := scr.GetEvent().(type) {
		case KeyEvent:
			if ev.r == 'q' {
				break mainloop
			}
		}
	}
}
