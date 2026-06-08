package main

import (
	"fmt"
	"sync"
	"time"
)

type Spinner struct {
	mu     sync.Mutex
	stopCh chan struct{}
}

func NewSpinner() *Spinner {
	return &Spinner{}
}

func (s *Spinner) Start() {
	s.mu.Lock()
	if s.stopCh != nil {
		s.mu.Unlock()
		return
	}
	s.stopCh = make(chan struct{})
	stopCh := s.stopCh
	s.mu.Unlock()

	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-stopCh:
				// Clear the line when stopping
				fmt.Print("\r\033[K")
				return
			case <-ticker.C:
				// \r moves cursor to beginning of line, \033[K clears the line
				fmt.Printf("\r\033[K%s FixPoint AI is analyzing...", frames[i%len(frames)])
				i++
			}
		}
	}()
}

func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopCh != nil {
		close(s.stopCh)
		s.stopCh = nil
	}
}
