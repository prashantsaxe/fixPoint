package fixpoint

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"

	"fixpoint/config"
	dap "github.com/google/go-dap"
)

const (
	defaultListenAddr   = "127.0.0.1:4000"
	defaultDebuggerAddr = "127.0.0.1:36281"
)

type Proxy struct {
	listenAddr   string
	debuggerAddr string
	cfg          *config.Config
}

var VerboseLogging bool

func NewProxy(listenAddr, debuggerAddr string, cfg *config.Config) *Proxy {
	return &Proxy{listenAddr: listenAddr, debuggerAddr: debuggerAddr, cfg: cfg}
}

func (p *Proxy) ListenAndServe() error {
	listener, err := net.Listen("tcp", p.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", p.listenAddr, err)
	}
	defer listener.Close()

	if VerboseLogging {
		log.Printf("FixPoint proxy listening on %s; forwarding to %s", p.listenAddr, p.debuggerAddr)
	} else {
		fmt.Println(RenderInfo(fmt.Sprintf("FixPoint listening on %s", p.listenAddr)))
	}

	for {
		ideConn, err := listener.Accept()
		if err != nil {
			if VerboseLogging {
				log.Printf("accept error: %v", err)
			}
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

	stdinCh   chan string
	promptMu  sync.Mutex
	promptCtx context.Context
	promptCf  context.CancelFunc
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
		stdinCh:      make(chan string, 1),
	}
	s.interrogator = NewInterrogator(s.writeToDebugger, NewSourceReader())

	s.promptCtx, s.promptCf = context.WithCancel(context.Background())

	go s.readStdin()

	if VerboseLogging {
		log.Printf("Session established with IDE")
		log.Printf("session started: IDE=%s <-> Debugger=%s", ideConn.RemoteAddr(), p.debuggerAddr)
	} else {
		fmt.Println(RenderInfo("FixPoint attached to IDE."))
	}

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
			if VerboseLogging {
				log.Printf("forward error IDE->Debugger (%s): %v", s.ideConn.RemoteAddr(), err)
			}
		}
		closeBoth()
	}()

	go func() {
		defer wg.Done()
		if err := s.processDAPStream(s.ideConn, s.debuggerConn, "Debugger->IDE", s.writeToIDE, true); err != nil && !errors.Is(err, net.ErrClosed) {
			if VerboseLogging {
				log.Printf("forward error Debugger->IDE (%s): %v", s.ideConn.RemoteAddr(), err)
			}
		}
		closeBoth()
	}()

	wg.Wait()
	if VerboseLogging {
		log.Printf("session ended: IDE=%s", s.ideConn.RemoteAddr())
	} else {
		fmt.Println(RenderInfo("FixPoint detached from IDE."))
	}
}

func (s *session) readStdin() {
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		s.stdinCh <- strings.TrimSpace(line)
	}
}

func (s *session) processDAPStream(dst net.Conn, src net.Conn, direction string, writeFn func(dap.Message) error, inspectStopped bool) error {
	_ = dst
	reader := bufio.NewReader(src)

	for {
		msg, err := dap.ReadProtocolMessage(reader)
		if err != nil {
			if VerboseLogging {
				log.Printf("DAP read error (%s): %v", direction, err)
			}
			return err
		}

		if VerboseLogging {
			log.Printf("[%s] Incoming: %T", direction, msg)

			switch m := msg.(type) {
			case *dap.LaunchRequest:
				if args, err := json.Marshal(m.Arguments); err == nil {
					log.Printf("[%s] LaunchRequest arguments: %s", direction, string(args))
				}
			case *dap.ErrorResponse:
				log.Printf("[%s] ErrorResponse: success=%v message=%q body.Error.Format=%q", direction, m.Success, m.Message, m.Body.Error.Format)
			}
		}

		if !inspectStopped {
			s.handleIDEResume(msg)
		}

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

func (s *session) handleIDEResume(msg dap.Message) {
	switch msg.(type) {
	case *dap.ContinueRequest, *dap.NextRequest, *dap.StepInRequest, *dap.StepOutRequest, *dap.DisconnectRequest:
		s.cancelPrompt()
	}
}

func (s *session) cancelPrompt() {
	s.promptMu.Lock()
	defer s.promptMu.Unlock()
	if s.promptCf != nil {
		s.promptCf()
		s.promptCf = nil
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

		if s.proxy.cfg == nil || s.proxy.cfg.OpenRouterAPIKey == "" {
			fmt.Println(RenderWarning("OPENROUTER_API_KEY not set in config; skipping AI analysis"))
			return
		}

		runAI := func() {
			spinner := NewSpinner()
			spinner.Start()

			analysis, err := GetFixFromAI(ctx, s.proxy.cfg)

			spinner.Stop()

			if err != nil {
				log.Printf("AI analysis failed: %v", err)
				fmt.Println(RenderWarning(fmt.Sprintf("AI analysis failed: %v", err)))
				return
			}

			fmt.Println(RenderAIResponseCard(analysis))
		}

		if reason == "exception" || reason == "panic" || reason == "error" {
			runAI()
		} else if reason == "breakpoint" || reason == "step" {
			fmt.Println(RenderInfo("Local Variables:"))
			for _, v := range ctx.Variables {
				fmt.Printf("  %s = %s\n", v.Name, v.Value)
			}

			promptCtx, promptCf := context.WithCancel(context.Background())
			s.promptMu.Lock()
			s.promptCf = promptCf
			s.promptMu.Unlock()

			fmt.Print(RenderPrompt("\n[FixPoint] Press [Enter] for AI analysis, or type 'c' to skip: "))

			select {
			case <-s.stdinCh:
			default:
			}

			select {
			case input := <-s.stdinCh:
				if input == "" || input == "a" {
					runAI()
				} else {
					fmt.Println(RenderInfo("Skipping AI analysis."))
				}
			case <-promptCtx.Done():
				fmt.Println(RenderInfo("Prompt cancelled (debugger resumed)."))
			}
		}
	}()
}
