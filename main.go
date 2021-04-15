package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/beevik/terminfo"
)

func main() {
	ansifile, err := os.Open("xterm-256color")
	if err != nil {
		log.Fatal(err)
	}
	defer ansifile.Close()

	ansireader := bufio.NewReader(ansifile)

	ti, err := terminfo.Read(ansireader)
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
