package main

import (
	"flag"
	"log"
)

func main() {
	listenAddr := flag.String("listen", defaultListenAddr, "address for IDE connections")
	debuggerAddr := flag.String("debugger", defaultDebuggerAddr, "address for debugger (Delve) connections")
	flag.Parse()

	apiKey, err := loadAPIKeyFromDotEnv(".env")
	if err != nil {
		log.Printf("warning: could not load API_KEY from .env: %v", err)
	}

	proxy := NewProxy(*listenAddr, *debuggerAddr, apiKey)
	if err := proxy.ListenAndServe(); err != nil {
		log.Fatalf("proxy failed: %v", err)
	}
}


