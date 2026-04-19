package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	dap "github.com/google/go-dap"
)

type Interrogator struct {
	mu               sync.Mutex
	nextSeq          int
	pendingResponses map[int]chan dap.Message

	sendRequest func(req dap.Message) error
	source      *SourceReader
}

func NewInterrogator(sendRequest func(req dap.Message) error, source *SourceReader) *Interrogator {
	return &Interrogator{
		nextSeq:          999,
		pendingResponses: make(map[int]chan dap.Message),
		sendRequest:      sendRequest,
		source:           source,
	}
}

func (i *Interrogator) RegisterRequest(seq int) chan dap.Message {
	i.mu.Lock()
	defer i.mu.Unlock()

	ch := make(chan dap.Message, 1)
	i.pendingResponses[seq] = ch
	return ch
}

func (i *Interrogator) DeliverResponse(resp dap.ResponseMessage) bool {
	seq := resp.GetResponse().RequestSeq

	i.mu.Lock()
	ch, ok := i.pendingResponses[seq]
	if ok {
		delete(i.pendingResponses, seq)
	}
	i.mu.Unlock()

	if !ok {
		return false
	}

	ch <- resp
	close(ch)
	return true
}

func (i *Interrogator) Close() {
	i.mu.Lock()
	defer i.mu.Unlock()

	for seq, ch := range i.pendingResponses {
		delete(i.pendingResponses, seq)
		close(ch)
	}
}

func (i *Interrogator) CaptureContext(threadID int) (*DebugContext, error) {
	stackReq := &dap.StackTraceRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "stackTrace",
		},
		Arguments: dap.StackTraceArguments{
			ThreadId:   threadID,
			StartFrame: 0,
			Levels:     20,
		},
	}

	stackMsg, err := i.sendAndWait(stackReq)
	if err != nil {
		return nil, fmt.Errorf("stackTrace request failed: %w", err)
	}

	stackResp, ok := stackMsg.(*dap.StackTraceResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected stackTrace response type: %T", stackMsg)
	}
	if !stackResp.Success {
		return nil, fmt.Errorf("stackTrace response failed: %s", stackResp.Message)
	}
	if len(stackResp.Body.StackFrames) == 0 {
		return nil, fmt.Errorf("stackTrace returned no frames")
	}

	frame := stackResp.Body.StackFrames[0]
	sourcePath := ""
	if frame.Source != nil {
		sourcePath = frame.Source.Path
	}

	scopesReq := &dap.ScopesRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "scopes",
		},
		Arguments: dap.ScopesArguments{FrameId: frame.Id},
	}

	scopesMsg, err := i.sendAndWait(scopesReq)
	if err != nil {
		return nil, fmt.Errorf("scopes request failed: %w", err)
	}

	scopesResp, ok := scopesMsg.(*dap.ScopesResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected scopes response type: %T", scopesMsg)
	}
	if !scopesResp.Success {
		return nil, fmt.Errorf("scopes response failed: %s", scopesResp.Message)
	}

	localsRef := findLocalsReference(scopesResp.Body.Scopes)
	if localsRef == 0 {
		return nil, fmt.Errorf("locals scope not found")
	}

	varsReq := &dap.VariablesRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "variables",
		},
		Arguments: dap.VariablesArguments{VariablesReference: localsRef},
	}

	varsMsg, err := i.sendAndWait(varsReq)
	if err != nil {
		return nil, fmt.Errorf("variables request failed: %w", err)
	}

	varsResp, ok := varsMsg.(*dap.VariablesResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected variables response type: %T", varsMsg)
	}
	if !varsResp.Success {
		return nil, fmt.Errorf("variables response failed: %s", varsResp.Message)
	}

	context := &DebugContext{
		Reason:     "breakpoint",
		ThreadID:   threadID,
		FrameID:    frame.Id,
		SourcePath: sourcePath,
		SourceLine: frame.Line,
		StackTrace: mapStackFrames(stackResp.Body.StackFrames),
		Variables:  mapVariables(varsResp.Body.Variables),
	}

	if sourcePath != "" {
		snippet, err := GetWindow(sourcePath, frame.Line)
		if err != nil {
			return context, fmt.Errorf("source window read failed: %w", err)
		}
		context.SourceSnippet = snippet
	}

	return context, nil
}

func (i *Interrogator) sendAndWait(req dap.Message) (dap.Message, error) {
	requestMsg, ok := req.(dap.RequestMessage)
	if !ok {
		return nil, fmt.Errorf("private request must implement dap.RequestMessage, got %T", req)
	}

	seq := i.nextSequence()
	request := requestMsg.GetRequest()
	request.Seq = seq
	request.Type = "request"

	waitCh := i.RegisterRequest(seq)
	if err := i.sendRequest(req); err != nil {
		i.unregisterRequest(seq)
		return nil, err
	}

	select {
	case msg, ok := <-waitCh:
		if !ok || msg == nil {
			return nil, fmt.Errorf("private response wait interrupted for %q (seq=%d)", request.Command, seq)
		}
		return msg, nil
	case <-time.After(5 * time.Second):
		i.unregisterRequest(seq)
		return nil, fmt.Errorf("timeout waiting for private response to %q (seq=%d)", request.Command, seq)
	}
}

func (i *Interrogator) nextSequence() int {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.nextSeq++
	return i.nextSeq
}

func (i *Interrogator) unregisterRequest(seq int) {
	i.mu.Lock()
	defer i.mu.Unlock()

	ch, ok := i.pendingResponses[seq]
	if !ok {
		return
	}
	delete(i.pendingResponses, seq)
	close(ch)
}

func findLocalsReference(scopes []dap.Scope) int {
	for _, scope := range scopes {
		if strings.EqualFold(scope.Name, "locals") {
			return scope.VariablesReference
		}
	}
	for _, scope := range scopes {
		if scope.VariablesReference != 0 {
			return scope.VariablesReference
		}
	}
	return 0
}

func mapStackFrames(frames []dap.StackFrame) []StackFrameInfo {
	out := make([]StackFrameInfo, 0, len(frames))
	for _, frame := range frames {
		sourcePath := ""
		if frame.Source != nil {
			sourcePath = frame.Source.Path
		}
		out = append(out, StackFrameInfo{
			ID:         frame.Id,
			Name:       frame.Name,
			SourcePath: sourcePath,
			Line:       frame.Line,
			Column:     frame.Column,
		})
	}
	return out
}

func mapVariables(vars []dap.Variable) []VariableInfo {
	out := make([]VariableInfo, 0, len(vars))
	for _, variable := range vars {
		out = append(out, VariableInfo{
			Name:  variable.Name,
			Type:  variable.Type,
			Value: variable.Value,
		})
	}
	return out
}
