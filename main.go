package main

import (
	"flag"
	"log"
)

func main() {
	listenAddr := flag.String("listen", defaultListenAddr, "address for IDE connections")
	debuggerAddr := flag.String("debugger", defaultDebuggerAddr, "address for debugger (Delve) connections")
	flag.Parse()

	proxy := NewProxy(*listenAddr, *debuggerAddr)
	if err := proxy.ListenAndServe(); err != nil {
		log.Fatalf("proxy failed: %v", err)
	}
}
