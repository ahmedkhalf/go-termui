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

type Application struct {
	ti      *terminfo.Terminfo
	Console console.Console

	mu *sync.Mutex

	input  io.Reader
	output io.Writer

	events chan Event
}

func (a *Application) EnterFullScreen() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.ti.Fprintf(a.output, terminfo.EnterCaMode)
}

func (a *Application) ExitFullScreen() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.ti.Fprintf(a.output, terminfo.ExitCaMode)
}

func (a *Application) Goto(y, x int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.ti.Fprintf(a.output, terminfo.CursorAddress, y, x)
}

func (a *Application) Start() {
	// First we set enable raw mode and noecho, this helps us get user input
	// as the user types and doesn't show the input to screen.
	a.Console = console.Current()
	a.Console.SetRaw()
	a.Console.DisableEcho()

	a.events = make(chan Event)

	// Resize Loop
	go func() {
		// Initial Size
		size, _ := a.Console.Size()
		a.events <- ResizeEvent{Width: size.Width, Height: size.Height}

		// Resize
		resizeSignal := make(chan os.Signal, 1)
		signal.Notify(resizeSignal, syscall.SIGWINCH)
		for {
			<-resizeSignal
			size, _ := a.Console.Size()
			a.events <- ResizeEvent{Width: size.Width, Height: size.Height}
		}
	}()

	// Input Loop
	go func() {
		for {
			r := readInput(a.input)
			a.events <- KeyEvent{r}
		}
	}()
}

func (a *Application) GetEvent() Event {
	select {
	case ev := <-a.events:
		return ev
	}
}

func (a *Application) End() {
	if a.Console != nil {
		a.Console.Reset()
	}
}

func NewApplication() *Application {
	ti, err := terminfo.LoadFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	a := &Application{
		ti:     ti,
		mu:     &sync.Mutex{},
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

	app.Goto(1, 3)
	fmt.Fprint(app.output, "Test")

	defer app.ExitFullScreen()

	app.Start()
	defer app.End()

mainloop:
	for {
		switch ev := app.GetEvent().(type) {
		case KeyEvent:
			if ev.r == 'q' {
				break mainloop
			}
		}
	}
}
