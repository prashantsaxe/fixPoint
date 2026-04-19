package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	dap "github.com/google/go-dap"
)

const (
	defaultListenAddr   = "127.0.0.1:4000"
	defaultDebuggerAddr = "127.0.0.1:36281"
)

type Proxy struct {
	listenAddr   string
	debuggerAddr string
}

func NewProxy(listenAddr, debuggerAddr string) *Proxy {
	return &Proxy{listenAddr: listenAddr, debuggerAddr: debuggerAddr}
}

func (p *Proxy) ListenAndServe() error {
	listener, err := net.Listen("tcp", p.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", p.listenAddr, err)
	}
	defer listener.Close()

	log.Printf("FixPoint proxy listening on %s; forwarding to %s", p.listenAddr, p.debuggerAddr)

	for {
		ideConn, err := listener.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		go p.handleSession(ideConn)
	}
}

type session struct {
	proxy        *Proxy
	ideConn      net.Conn
	debuggerConn net.Conn

	debuggerWriteMu sync.Mutex

	interrogator *Interrogator
}

func (p *Proxy) handleSession(ideConn net.Conn) {
	defer ideConn.Close()

	debuggerConn, err := net.Dial("tcp", p.debuggerAddr)
	if err != nil {
		log.Printf("failed to connect debugger for %s: %v", ideConn.RemoteAddr(), err)
		return
	}

	s := &session{
		proxy:        p,
		ideConn:      ideConn,
		debuggerConn: debuggerConn,
	}
	s.interrogator = NewInterrogator(s.writeToDebugger, NewSourceReader())

	log.Printf("Session established with IDE")

	log.Printf("session started: IDE=%s <-> Debugger=%s", ideConn.RemoteAddr(), p.debuggerAddr)

	var once sync.Once
	closeBoth := func() {
		once.Do(func() {
			_ = s.ideConn.Close()
			_ = s.debuggerConn.Close()
			s.interrogator.Close()
		})
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if err := s.processDAPStream(s.debuggerConn, s.ideConn, "IDE->Debugger", s.writeToDebugger, false); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Printf("forward error IDE->Debugger (%s): %v", s.ideConn.RemoteAddr(), err)
		}
		closeBoth()
	}()

	go func() {
		defer wg.Done()
		if err := s.processDAPStream(s.ideConn, s.debuggerConn, "Debugger->IDE", s.writeToIDE, true); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Printf("forward error Debugger->IDE (%s): %v", s.ideConn.RemoteAddr(), err)
		}
		closeBoth()
	}()

	wg.Wait()
	log.Printf("session ended: IDE=%s", s.ideConn.RemoteAddr())
}

func (s *session) processDAPStream(dst net.Conn, src net.Conn, direction string, writeFn func(dap.Message) error, inspectStopped bool) error {
	_ = dst
	reader := bufio.NewReader(src)

	for {
		msg, err := dap.ReadProtocolMessage(reader)
		if err != nil {
			log.Printf("DAP read error (%s): %v", direction, err)
			return err
		}

		log.Printf("[%s] Incoming: %T", direction, msg)

		if inspectStopped {
			if resp, ok := msg.(dap.ResponseMessage); ok && s.interrogator.DeliverResponse(resp) {
				continue
			}
		}

		if err := writeFn(msg); err != nil {
			return err
		}

		if inspectStopped {
			s.handleStoppedEvent(msg)
		}
	}
}

func (s *session) writeToDebugger(msg dap.Message) error {
	s.debuggerWriteMu.Lock()
	defer s.debuggerWriteMu.Unlock()
	return dap.WriteProtocolMessage(s.debuggerConn, msg)
}

func (s *session) writeToIDE(msg dap.Message) error {
	return dap.WriteProtocolMessage(s.ideConn, msg)
}

func (s *session) handleStoppedEvent(msg dap.Message) {
	stopped, ok := msg.(*dap.StoppedEvent)
	if !ok {
		return
	}
	if !strings.EqualFold(stopped.Body.Reason, "breakpoint") {
		return
	}

	body := stopped.Body
	log.Printf("🎯 Breakpoint Hit! Reason: %s, ThreadId: %d", body.Reason, body.ThreadId)

	raw, err := json.Marshal(stopped)
	if err != nil {
		log.Printf("failed to marshal stopped event: %v", err)
	} else {
		log.Printf("StoppedEvent raw JSON: %s", raw)
	}

	threadID := body.ThreadId
	go func() {
		ctx, err := s.interrogator.CaptureContext(threadID)
		if err != nil {
			log.Printf("context capture failed: %v", err)
			return
		}
		ctxJSON, err := json.Marshal(ctx)
		if err != nil {
			log.Printf("context marshal failed: %v", err)
			return
		}
		log.Printf("Captured DebugContext: %s", ctxJSON)
	}()
}
