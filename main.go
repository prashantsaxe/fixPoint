package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	dap "github.com/google/go-dap"
)

const (
	listenAddr   = "127.0.0.1:4000"
	debuggerAddr = "127.0.0.1:2345"
)

func main() {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", listenAddr, err)
	}
	defer listener.Close()

	log.Printf("FixPoint proxy listening on %s; forwarding to %s", listenAddr, debuggerAddr)

	for {
		ideConn, err := listener.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}

		go handleSession(ideConn)
	}
}

func handleSession(ideConn net.Conn) {
	defer ideConn.Close()

	debuggerConn, err := net.Dial("tcp", debuggerAddr)
	if err != nil {
		log.Printf("failed to connect debugger for %s: %v", ideConn.RemoteAddr(), err)
		return
	}

	log.Printf("session started: IDE=%s <-> Debugger=%s", ideConn.RemoteAddr(), debuggerAddr)

	var once sync.Once
	closeBoth := func() {
		once.Do(func() {
			_ = ideConn.Close()
			_ = debuggerConn.Close()
		})
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if err := processDAPStream(debuggerConn, ideConn, "IDE->Debugger", false); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Printf("forward error IDE->Debugger (%s): %v", ideConn.RemoteAddr(), err)
		}
		closeBoth()
	}()

	go func() {
		defer wg.Done()
		if err := processDAPStream(ideConn, debuggerConn, "Debugger->IDE", true); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Printf("forward error Debugger->IDE (%s): %v", ideConn.RemoteAddr(), err)
		}
		closeBoth()
	}()

	wg.Wait()
	log.Printf("session ended: IDE=%s", ideConn.RemoteAddr())
}

func processDAPStream(dst net.Conn, src net.Conn, direction string, inspectStopped bool) error {
	reader := bufio.NewReader(src)

	for {
		content, err := dap.ReadBaseMessage(reader)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return err
			}
			log.Printf("DAP read error (%s): %v", direction, err)
			continue
		}

		if err := writeDAPMessage(dst, content); err != nil {
			return err
		}

		msg, err := dap.DecodeProtocolMessage(content)
		if err != nil {
			log.Printf("DAP decode error (%s): %v", direction, err)
			continue
		}

		if inspectStopped {
			logStoppedEvent(msg)
		}
	}
}

func writeDAPMessage(dst net.Conn, content []byte) error {
	header := []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(content)))

	if err := writeAll(dst, header); err != nil {
		return err
	}
	return writeAll(dst, content)
}

func writeAll(dst net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := dst.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

func logStoppedEvent(msg dap.Message) {
	stopped, ok := msg.(*dap.StoppedEvent)
	if !ok {
		return
	}

	body := stopped.Body
	log.Printf("🎯 Breakpoint Hit! Reason: %s, ThreadId: %d", body.Reason, body.ThreadId)

	raw, err := json.Marshal(stopped)
	if err != nil {
		log.Printf("failed to marshal stopped event: %v", err)
		return
	}
	log.Printf("StoppedEvent raw JSON: %s", raw)
}
