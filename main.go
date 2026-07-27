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

var VerboseLogging bool

func main() {
	listenAddr := flag.String("listen", defaultListenAddr, "address for IDE connections")
	debuggerAddr := flag.String("debugger", "", "debugger address (skip auto-spawn, e.g. 127.0.0.1:36281)")
	verbose := flag.Bool("verbose", false, "enable detailed debugging logs")
	flag.Parse()

	VerboseLogging = *verbose

	if err := godotenv.Load(); err != nil {
		if VerboseLogging {
			log.Printf("warning: no .env file found: %v", err)
		}
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
		if VerboseLogging {
			log.Printf("Delve DAP backend spawned on %s", *debuggerAddr)
		}
	} else {
		if VerboseLogging {
			log.Printf("connecting to external debugger at %s", *debuggerAddr)
		}
	}

	proxy := NewProxy(*listenAddr, *debuggerAddr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		fmt.Println(RenderInfo("\nShutting down FixPoint..."))
		if delveCmd != nil && delveCmd.Process != nil {
			delveCmd.Process.Kill()
		}
		os.Exit(0)
	}()

	if err := proxy.ListenAndServe(); err != nil {
		log.Fatalf("proxy failed: %v", err)
	}
}
