package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"os/exec"
	"syscall"

	"github.com/joho/godotenv"
)

func main() {
	listenAddr := flag.String("listen", defaultListenAddr, "address for IDE connections")
	debuggerAddr := flag.String("debugger", "", "debugger address (skip auto-spawn, e.g. 127.0.0.1:36281)")
	flag.Parse()

	if err := godotenv.Load(); err != nil {
		log.Printf("warning: no .env file found: %v", err)
	}

	var delveCmd *exec.Cmd

	if *debuggerAddr == "" {
		delvePort, cmd, err := StartDelveBackend()
		if err != nil {
			log.Fatalf("failed to start delve backend: %v", err)
		}
		delveCmd = cmd
		addr := fmt.Sprintf("127.0.0.1:%d", delvePort)
		debuggerAddr = &addr
		log.Printf("Delve DAP backend spawned on %s", *debuggerAddr)
	} else {
		log.Printf("connecting to external debugger at %s", *debuggerAddr)
	}

	proxy := NewProxy(*listenAddr, *debuggerAddr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Println("shutting down...")
		if delveCmd != nil && delveCmd.Process != nil {
			delveCmd.Process.Kill()
		}
		os.Exit(0)
	}()

	if err := proxy.ListenAndServe(); err != nil {
		log.Fatalf("proxy failed: %v", err)
	}
}
