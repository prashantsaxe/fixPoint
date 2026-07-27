package source

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"

	"fixpoint/internal/models"
)

type SourceReader struct{}

func NewSourceReader() *SourceReader {
	return &SourceReader{}
}

func (s *SourceReader) GetEnclosingFunction(path string, targetLine int) ([]models.SourceLine, error) {
	if path == "" {
		return nil, fmt.Errorf("source path is empty")
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		// Fallback to ±10 lines
		return s.getFallbackWindow(path, targetLine)
	}

	var startLine, endLine int
	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		
		switch fn := n.(type) {
		case *ast.FuncDecl:
			pos := fset.Position(fn.Pos()).Line
			end := fset.Position(fn.End()).Line
			if targetLine >= pos && targetLine <= end {
				startLine = pos
				endLine = end
			}
		case *ast.FuncLit:
			pos := fset.Position(fn.Pos()).Line
			end := fset.Position(fn.End()).Line
			if targetLine >= pos && targetLine <= end {
				startLine = pos
				endLine = end
			}
		}
		return true
	})

	if startLine == 0 || endLine == 0 {
		return s.getFallbackWindow(path, targetLine)
	}

	return s.readLines(path, startLine, endLine)
}

func (s *SourceReader) getFallbackWindow(path string, targetLine int) ([]models.SourceLine, error) {
	startLine := targetLine - 10
	if startLine < 1 {
		startLine = 1
	}
	endLine := targetLine + 10
	return s.readLines(path, startLine, endLine)
}

func (s *SourceReader) readLines(path string, startLine, endLine int) ([]models.SourceLine, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []models.SourceLine
	scanner := bufio.NewScanner(f)
	currentLine := 1
	for scanner.Scan() {
		if currentLine >= startLine && currentLine <= endLine {
			out = append(out, models.SourceLine{LineNumber: currentLine, Text: scanner.Text()})
		}
		if currentLine > endLine {
			break
		}
		currentLine++
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
