package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"fixpoint"
	"fixpoint/config"

	"github.com/joho/godotenv"
)

const version = "0.1.0"

func main() {
	// Attempt to load .env for any legacy environment variables
	if err := godotenv.Load(); err != nil {
		// Ignore error
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version":
			fmt.Printf("FixPoint v%s\n", version)
			os.Exit(0)
		case "config":
			runConfigSetup()
			os.Exit(0)
		}
	}

	fs := flag.NewFlagSet("fixpoint", flag.ExitOnError)
	listenAddr := fs.String("listen", "127.0.0.1:4000", "address for IDE connections")
	debuggerAddr := fs.String("debugger", "", "debugger address (skip auto-spawn, e.g. 127.0.0.1:36281)")
	verbose := fs.Bool("verbose", false, "enable detailed debugging logs")

	// Parse flags starting after the binary name (if no subcommand is used)
	// If a subcommand was used (but not matching above), we still parse normally.
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		// Ignore unknown subcommand and just parse
		fs.Parse(os.Args[2:])
	} else {
		fs.Parse(os.Args[1:])
	}

	fixpoint.VerboseLogging = *verbose

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if cfg.OpenRouterAPIKey == "" {
		fmt.Println(fixpoint.RenderInfo("No API key found in config. Let's set it up!"))
		runConfigSetup()
		
		cfg, err = config.LoadConfig()
		if err != nil {
			log.Fatalf("failed to load config after setup: %v", err)
		}
	}

	var delveCmd *exec.Cmd

	if *debuggerAddr == "" {
		delvePort, cmd, err := fixpoint.StartDelveBackend()
		if err != nil {
			log.Fatalf("failed to start delve backend: %v", err)
		}
		delveCmd = cmd
		addr := fmt.Sprintf("127.0.0.1:%d", delvePort)
		debuggerAddr = &addr
		if fixpoint.VerboseLogging {
			log.Printf("Delve DAP backend spawned on %s", *debuggerAddr)
		}
	} else {
		if fixpoint.VerboseLogging {
			log.Printf("connecting to external debugger at %s", *debuggerAddr)
		}
	}

	proxy := fixpoint.NewProxy(*listenAddr, *debuggerAddr, cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		fmt.Println(fixpoint.RenderInfo("\nShutting down FixPoint..."))
		if delveCmd != nil && delveCmd.Process != nil {
			delveCmd.Process.Kill()
		}
		os.Exit(0)
	}()

	if err := proxy.ListenAndServe(); err != nil {
		log.Fatalf("proxy failed: %v", err)
	}
}

func runConfigSetup() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter your OpenRouter API Key: ")
	apiKey, _ := reader.ReadString('\n')
	apiKey = strings.TrimSpace(apiKey)

	fmt.Print("Enter preferred model (default: google/gemini-2.5-flash): ")
	model, _ := reader.ReadString('\n')
	model = strings.TrimSpace(model)
	if model == "" {
		model = "google/gemini-2.5-flash"
	}

	cfg := &config.Config{
		OpenRouterAPIKey: apiKey,
		Model:            model,
	}

	if err := config.SaveConfig(cfg); err != nil {
		fmt.Printf("Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(fixpoint.RenderInfo("Configuration saved successfully!"))
}
