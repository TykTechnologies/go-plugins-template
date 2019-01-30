package main

import (
    "net/http"
    "log"
)

// Post plugin which reads value from Auth middleware, and embeds it to header
func Middleware(h http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        log.Println("Running POST plugin")
        if user := r.Context().Value("Username"); user != nil {
            r.Header.Set("Username", r.Context().Value("Username").(string))
        }

        h.ServeHTTP(w, r)
    })
}
