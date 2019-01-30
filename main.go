package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"plugin"
	"reflect"
	"strings"
)

// Middleware approach based on Mat Ryer article
// https://medium.com/@matryer/writing-middleware-in-golang-and-how-go-makes-it-so-much-fun-4375c1246e81

type Middleware func(http.Handler) http.Handler

func Chain(h http.Handler, mws ...Middleware) http.Handler {
	// Reverse order
	for i := len(mws) - 1; i >= 0; i-- {
		if mws[i] != nil {
			h = mws[i](h)
		}
	}
	return h
}

func BasicAuth(login, password string) Middleware {
	if login == "" || password == "" {
		return nil
	}

	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Println("Basic auth")

			token := r.Header.Get("Authorization")
			bits := strings.Split(token, " ")
			if len(bits) != 2 {
				w.Header().Add("WWW-Authenticate", "realm=proxy")
				http.Error(w, "Basic auth header not found or malformed", http.StatusUnauthorized)
				return
			}
			authvaluesStr, _ := base64.StdEncoding.DecodeString(bits[1])
			authValues := strings.Split(string(authvaluesStr), ":")

			if authValues[0] != login && authValues[1] != password {
				http.Error(w, "Basic auth header not found or malformed", http.StatusUnauthorized)
				return
			}

			// Set value which be available to all middlewares
			ctx := context.WithValue(r.Context(), "Username", login)
			h.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func LoadPatch(component string, symbol string) (interface{}, error) {
	plugin_path := "./patches/" + component + ".so"
	if _, err := os.Stat(plugin_path); err == nil {
		return LoadPlugin(plugin_path, symbol)
	}

	return nil, nil
}

func LoadMiddlewarePlugin(path string) Middleware {
	if path == "" {
		return nil
	}

	symbol, err := LoadPlugin(path, "Middleware")
	if err != nil {
		log.Fatal("Can't load plugin", path, err)
	}

	if mw, ok := symbol.(func(http.Handler) http.Handler); ok {
		return mw
	} else {
		log.Fatal("'Middleware' function should have `func(http.Handler) http.Handler` type", path, ok, reflect.TypeOf(symbol))
		return nil
	}
}

func LoadPlugin(path string, symbol string) (interface{}, error) {
	loadedPlugin, err := plugin.Open(path)

	if err != nil {
		return nil, err
	}

	funcSymbol, err := loadedPlugin.Lookup(symbol)
	if err != nil {
		return nil, fmt.Errorf("Can't find '%s' symbol in plugin %s %v", symbol, path, err)
	}

	return funcSymbol, nil
}

// Intentionally contains bug, which do not respect `prefix` variable
// Use `patch` to fix the code
func Proxy(target *url.URL, prefix string) http.Handler {
	obj, err := LoadPatch("reverse_proxy", "Proxy")
	if err != nil {
		log.Println(err)
	}
	if obj != nil {
		log.Println("Loading patched reverse_proxy module")
		if proxy, ok := obj.(func(*url.URL, string) http.Handler); !ok {
			log.Fatal("Function signature do not match", reflect.TypeOf(obj))
		} else {
			return proxy(target, prefix)
		}
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Director = func(r *http.Request) {
		r.URL.Scheme = target.Scheme
		r.URL.Host = target.Host
		r.Host = target.Host
	}

	return proxy
}

func main() {
	port := flag.String("port", ":9090", "Proxy listen address: ':9090'")
	target := flag.String("url", "https://httpbin.org", "Target for proxy. Default: https://httpbin.org")
	prefix := flag.String("prefix", "", "Root prefix")

	basicUser := flag.String("basic-user", "", "Set to non empty to enable basic auth")
	basicPassword := flag.String("basic-password", "", "Set to non empty to enable basic auth")

	prePlugin := flag.String("pre-plugin", "", "Path to pre plugin")
	postPlugin := flag.String("post-plugin", "", "Path to post plugin")

	flag.Parse()

	rpURL, err := url.Parse(*target)
	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/", Chain(Proxy(rpURL, *prefix), LoadMiddlewarePlugin(*prePlugin), BasicAuth(*basicUser, *basicPassword), LoadMiddlewarePlugin(*postPlugin)))
	log.Fatal(http.ListenAndServe(*port, nil))
}
