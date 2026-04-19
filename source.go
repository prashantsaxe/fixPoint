package main

import (
	"bufio"
	"fmt"
	"os"
)

const sourceWindowSize = 15

type SourceReader struct{}

func NewSourceReader() *SourceReader {
	return &SourceReader{}
}

func GetWindow(path string, line int) ([]SourceLine, error) {
	return NewSourceReader().GetWindow(path, line)
}

// GetWindow returns sourceWindowSize lines centered around the requested line.
func (s *SourceReader) GetWindow(path string, line int) ([]SourceLine, error) {
	if path == "" {
		return nil, fmt.Errorf("source path is empty")
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, nil
	}

	if line < 1 {
		line = 1
	}
	if line > len(lines) {
		line = len(lines)
	}

	half := sourceWindowSize / 2
	start := line - half
	if start < 1 {
		start = 1
	}
	end := start + sourceWindowSize - 1
	if end > len(lines) {
		end = len(lines)
		start = end - sourceWindowSize + 1
		if start < 1 {
			start = 1
		}
	}

	out := make([]SourceLine, 0, end-start+1)
	for i := start; i <= end; i++ {
		out = append(out, SourceLine{LineNumber: i, Text: lines[i-1]})
	}

	return out, nil
}
