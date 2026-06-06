package main

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

const spinnerInterval = 100 * time.Millisecond

var brailleFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

type Spinner struct {
	total  int
	done   atomic.Int32
	stopCh chan struct{}
	doneCh chan struct{}
}

func (s *Spinner) Start(total int) {
	s.total = total
	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})
	go func() {
		defer close(s.doneCh)
		ticker := time.NewTicker(spinnerInterval)
		defer ticker.Stop()
		frame := 0
		for {
			select {
			case <-s.stopCh:
				fmt.Fprintf(os.Stderr, "\r%-40s\r", "")
				return
			case <-ticker.C:
				fmt.Fprintf(os.Stderr, "\r%c Checking %d/%d",
					brailleFrames[frame%len(brailleFrames)],
					s.done.Load(), s.total)
				frame++
			}
		}
	}()
}

func (s *Spinner) Increment() {
	s.done.Add(1)
}

func (s *Spinner) Stop() {
	close(s.stopCh)
	<-s.doneCh
}
