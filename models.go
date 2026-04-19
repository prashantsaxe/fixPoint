package main

// SourceLine stores one line of source with its original line number.
type SourceLine struct {
	LineNumber int    `json:"lineNumber"`
	Text       string `json:"text"`
}

// StackFrameInfo is a compact stack frame representation for captured context.
type StackFrameInfo struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	SourcePath string `json:"sourcePath"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
}

// VariableInfo is a simplified variable view for captured context.
type VariableInfo struct {
	Name  string `json:"name"`
	Type  string `json:"type,omitempty"`
	Value string `json:"value"`
}

// DebugContext combines stack, locals, and source snippet near a breakpoint.
type DebugContext struct {
	Reason        string           `json:"reason"`
	ThreadID      int              `json:"threadId"`
	FrameID       int              `json:"frameId"`
	SourcePath    string           `json:"sourcePath"`
	SourceLine    int              `json:"sourceLine"`
	StackTrace    []StackFrameInfo `json:"stackTrace"`
	Variables     []VariableInfo   `json:"variables"`
	SourceSnippet []SourceLine     `json:"sourceSnippet"`
}
