package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
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
	apiKey       string
}

func NewProxy(listenAddr, debuggerAddr, apiKey string) *Proxy {
	return &Proxy{listenAddr: listenAddr, debuggerAddr: debuggerAddr, apiKey: apiKey}
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
	
	reason := strings.ToLower(stopped.Body.Reason)
	validReasons := map[string]bool{
		"breakpoint": true, "step": true, "exception": true, "panic": true, "error": true,
	}
	if !validReasons[reason] {
		return
	}

	body := stopped.Body
	fmt.Println(RenderBreakpointHeader(body.Reason, body.ThreadId))

	threadID := body.ThreadId
	go func() {
		ctx, err := s.interrogator.CaptureContext(threadID, body.Reason)
		if err != nil {
			log.Printf("context capture failed: %v", err)
			return
		}
		if sourceWindow := RenderSourceWindow(ctx); sourceWindow != "" {
			fmt.Println(sourceWindow)
		}

		if strings.TrimSpace(s.proxy.apiKey) == "" {
			fmt.Println(RenderWarning("-apikey not provided; skipping AI analysis"))
			return
		}

		runAI := func() {
			spinner := NewSpinner()
			spinner.Start()

			analysis, err := GetFixFromAI(ctx, s.proxy.apiKey)
			
			spinner.Stop()

			if err != nil {
				log.Printf("AI analysis failed: %v", err)
				return
			}

			fmt.Println(RenderAIResponseCard(analysis))
		}

		if reason == "exception" || reason == "panic" || reason == "error" {
			runAI()
		} else if reason == "breakpoint" || reason == "step" {
			fmt.Println("\nLocal Variables:")
			for _, v := range ctx.Variables {
				fmt.Printf("  %s = %s\n", v.Name, v.Value)
			}
			
			fmt.Print("\n[FixPoint] Breakpoint hit. Press [Enter] to run AI analysis, or type 'c' to skip: ")
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input == "" {
				runAI()
			} else {
				fmt.Println("Skipping AI analysis.")
			}
		}
	}()
}
