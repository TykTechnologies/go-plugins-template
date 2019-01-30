package main

import (
    "net/url"
    "net/http"
    "strings"
    "net/http/httputil"
)

func Proxy(target *url.URL, prefix string) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Director = func(r *http.Request) {
		r.URL.Scheme = target.Scheme
		r.URL.Host = target.Host
        r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
		r.Host = target.Host
	}

	return proxy
}
