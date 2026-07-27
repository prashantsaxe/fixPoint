package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

func main() {
	http.HandleFunc("/divide", divideHandler)
	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/crash", crashHandler)
	fmt.Println("Test server listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}

func divideHandler(w http.ResponseWriter, r *http.Request) {
	a := rand.Intn(10)
	b := rand.Intn(3) // will produce division-by-zero when b==0
	result := divide(a, b)
	fmt.Fprintf(w, "%d / %d = %d\n", a, b, result)
}

func divide(x, y int) int {
	res := x / y // intentional bug: no zero check
	fmt.Printf("response of x/y : %d", res)
	return res
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	// Set breakpoint on this line to inspect requests
	userAgent := r.UserAgent()
	now := time.Now().Format(time.RFC3339)
	fmt.Fprintf(w, "OK | %s | %s\n", now, userAgent)
}

func crashHandler(w http.ResponseWriter, r *http.Request) {
	names := []string{"alice", "bob"}
	// Intentional: idx=2 causes panic
	idx := rand.Intn(3)
	fmt.Fprintf(w, "Hello, %s\n", names[idx])
}
