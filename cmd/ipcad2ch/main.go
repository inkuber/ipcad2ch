package main

import (
	"context"
	"fmt"
	"github.com/inkuber/ipcad2ch/pkg/ipcad"
	"os"
)

func main() {
	entries := make(chan ipcad.Entry, 100)
	ctx := context.Background()
	go ipcad.Read(ctx, os.Stdin, entries)

	for {
		done := false
		select {
		case entry, ok := <-entries:
			if ok {
				fmt.Printf("%v\n", entry)
			} else {
				done = true
			}
		}

		if done {
			break
		}
	}
}
