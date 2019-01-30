package main

import (
    "net/http"
    "log"
    "strconv"
    "math/rand"
)

// Example plugin which adds X-Trace-ID header with unique value, for tracing purpose
func Middleware(h http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        log.Println("Running PRE plugin")
        r.Header.Set("X-Trace-ID", strconv.Itoa(int(rand.Int63())))
        h.ServeHTTP(w, r)
    })
}
