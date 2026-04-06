package main

import (
	"io"
	"log"
	"net"
	"sync"
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
		if _, err := io.Copy(debuggerConn, ideConn); err != nil {
			log.Printf("forward error IDE->Debugger (%s): %v", ideConn.RemoteAddr(), err)
		}
		closeBoth()
	}()

	go func() {
		defer wg.Done()
		if _, err := io.Copy(ideConn, debuggerConn); err != nil {
			log.Printf("forward error Debugger->IDE (%s): %v", ideConn.RemoteAddr(), err)
		}
		closeBoth()
	}()

	wg.Wait()
	log.Printf("session ended: IDE=%s", ideConn.RemoteAddr())
}
