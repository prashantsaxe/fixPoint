package main

import (
	"fmt"
	"log"
	"net"
	"os/exec"
	"time"
)

func StartDelveBackend() (port int, cmd *exec.Cmd, err error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, fmt.Errorf("resolve addr: %w", err)
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, nil, fmt.Errorf("listen ephemeral: %w", err)
	}
	port = listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	listenAddr := fmt.Sprintf("127.0.0.1:%d", port)
	cmd = exec.Command("dlv", "dap", "--listen="+listenAddr, "--log")

	if err := cmd.Start(); err != nil {
		return 0, nil, fmt.Errorf("start dlv: %w", err)
	}

	if VerboseLogging {
		log.Printf("waiting for dlv dap to initialize on %s...", listenAddr)
	}
	time.Sleep(2 * time.Second)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return 0, nil, fmt.Errorf("dlv dap exited early: %w", err)
	case <-time.After(100 * time.Millisecond):
		return port, cmd, nil
	}
}
