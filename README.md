# Go plugins showcase

Aim of this project it to show native Go plugin usage patterns, which you can apply to dynamically extend and even patch your Go apps. 

Project itself is a simple reverse proxy, with HTTP basic auth support. You can define PRE auth and POST auth hooks to run custom logic, and dynamically replace, e.g patch, parts of the application (reverse proxy logic, which by default intentionally contains a bug).

Benefit of having separate hooks in our case, is that using PRE plugin we can write our authentication layer, and implement something like IP filtering or JWT authentication, or just some request transformation. And with using POST plugin, we know that user is already logged, we have access to his session, and can do some expensive logic, or for example, override transport layer and add support for talking with gRPC services.

Additionally our application automatically looks for a `so` files in `patches` folder, and if it finds the patch in proper format, it loads it, overriding default built-in behavior. 

Example usage: 
```bash
go run main.go -port ":9090" -pre-plugin pre.so -post-plugin post.so -basic-user=test -basic-password=test --prefix="/" -target="https://httpbin.org"
```

The command above, will start the server on port 9090, and all incoming requests will be proxied as it is to httpbin.org service. On top of that it restrict access to proxy using HTTP Basic Auth, and loads PRE and POST plugins from specified `so` files, which contain hooks running before and after authentication.

Additionally repository include docker build environment in `build-env` folder, which can be used to build both main binaries and plugins, so they will be always compatible to each other, and `Makefile` with useful commands, you can use to simplify build process.

## Building blocks

In order to be extendable, your application should be modular, and each module should have strict simple interface, and as less dependencies as possible. If we talking about web applications, middleware approach is a quite common pattern: when requests goes though chain middlewares, where each one can somehow alter request or response behavior. 

One of the ways to implement middleware is to follow simple pattern describe below:
```go
type Middleware func(http.Handler) http.Handler

// Example: http.Handle("/", Chain(indexHandler, BasicAuth(), Tracing(true)))
func Chain(h http.Handler, mws ...Middleware) http.Handler {
    // Reverse order
    for i := len(mws)-1; i>=0; i-- {
	if mws[i] != nil {
            h = mws[i](h)
        }
    }
    return h
}

// Example middleware which adds X-Trace-ID header with unique value, for tracing purpose
func Tracing(enabled bool) Middleware {
    if !enabled {
	    return nil
    }
    return func(h http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            r.Header.Set("X-Trace-ID", strconv.Itoa(int(rand.Int63())))
	    // Pass request to next middleware in chain
            // Or respond and return early
            h.ServeHTTP(w, r)
        })
    }
}
```

If `-pre-plugin` or `-post-plugin` are passed, it will try to load Go Plugin from specified file. Loading the plugins is done a standard way, according to Go documentation.  See below:
```go
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

```

For reverse proxying we use standard `net/http/httputil` package. Code intentionally contains bug, preventing it work with non empty prefix, and we going to dynamically patch it later, without touching main code. As you can see function looks for `reverse_proxy.so` file in `patches` folder, and tries to load it, instead of using built-in code.

```go
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
```

Additionally there is support for HTTP basic auth, which is also implemented as standard `Middleware` interface.  Its job is to extract value from Authorization header, and compare user and password with defined values.  Another interesting bit is that it writes data to request context, which is the way to share data between middleware.
```go
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
```

The final request chain looks as simple as:
```go
http.Handle("/", Chain(Proxy(target, prefix), LoadMiddlewarePlugin(prePluginPath), BasicAuth(basicUser, basicPassword), LoadMiddlewarePlugin(postPluginPath)))
```

## How to run this Demo

To fully understand concept, it is recommended if you go though the steps described below.

You will learn how to create and build PRE and POST hook plugins, how to fix issue in reverse proxy code without touching main binary, and how to use build environment in order to compile plugins which are compatible to existing binaries, e.g. use same build environment.

If you run the command at start of the readme, you will be get an error that, post and pre plugins are not found. It happens because you first need to compile them. 

This repository already contains samples for plugins. PRE plugin located at `./plugin-examples/pre/plugin.go` and looks like:
```go
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
```
It share the same `Middleware` interface we described above. Plugin runs before authentication has happened, has access to both request and response.
Goal of this middleware is just embed tracing header to request. To build it run:
```bash
go build -buildmode=plugin -o pre.so ./plugin-examples/pre
```

Our post plugin looks very similar, except that it runs after Authentication middleware, which shared info about currently logged user via Context. We can access it inside plugin, and add the header with its value.
```go
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
```

Lets compile it:
```go
go build -buildmode=plugin -o post.so ./plugin-examples/post
```

Now we have everything to run our app. Start the proxy:
```go
go run main.go -port ":9090" -pre-plugin pre.so -post-plugin post.so -basic-user=test -basic-password=test --prefix="/" -target="https://httpbin.org"
```

Request to `http://test:test@localhost:9090/get` should forward request to `https://httpbin.org/get` and you should see the following output:
```go
curl http://test:tes@localhost:9090/get

{
  "args": {}, 
  "headers": {
    "Accept": "*/*", 
    "Accept-Encoding": "gzip", 
    "Authorization": "Basic dGVzdDp0ZXN0", 
    "Connection": "close", 
    "Host": "httpbin.org", 
    "User-Agent": "curl/7.54.0", 
    "Username": "test", 
    "X-Trace-Id": "5577006791947779410"
  }, 
  "origin": "::1, 79.159.85.192", 
  "url": "https://httpbin.org/get"
}
```
You should notice our 2 custom headers: `X-Trace-Id` and `Username`, which was added by our middleware. 

### Fixing the bug in existing binary
Our proxy functionality allows you to specify prefix, so if you set it to `/prefix`, proxy request to ``/prefix/get``` should be transformed to `/get`. However if you try to specify it now, it will not work, because we intentionally added a bug in this code. Lets try to fix it by dynamically patching our application.

Patch itself is located at `./patches/reverse_proxy.go`, and looks like this:
```go
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

        // Our fix
        r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)

		r.Host = target.Host
	}

	return proxy
}
```
Patch itself is defined in exactly the same interface as original Proxy object. When proxy code is gets loaded, it automatically scans `patches` folder, and tries to find plugin with `reverse_proxy.so` name, and interface matching the Proxy object. Lets compile it:
```go
go build -buildmode=plugin -o patches/reverse-proxy.so ./patches/reverse-proxy.go
```

Lets run our app with non empty prefix, and verify that request to `/test/get` gets properly proxied:
```bash
go run main.go -port ":9090" -pre-plugin pre.so -post-plugin post.so -basic-user=test -basic-password=test -prefix=“/prefx” -target="https://httpbin.org"
```
While running the code above, you should see the `Loading patched reverse_proxy module ` log message, which means our patch was successfully applied.

## Universal Build Environment
One of the pitfalls of using plugins is that you have to compile the in exactly the same environment, as your main binary was build. And by environment it means anything from $GOPATH, Go version to even different vendored modules. In order to solve this issue, you should create either Docker image, or use the same server, but build all production binaries and plugins. 

Except benefits mentioned above, you can share your build pipeline with your users, so they can build and share they own plugins, which will be guaranteed to be compatible with binaries you release.

This repository `build_env` folder contains Docker based example of such environment, and provides a more advanced way of building plugins, including `vendor` support.  As first step go to `build_env` folder and build the Docker image:
```bash
cd build_env
docker build -t build-env-test .
```

To building the main binary you should run `make build`, from root repo folder, which internally runs the following command:
```bash
docker run --rm -i -e GOOS=linux -e GOARCH=amd64 -v `pwd`:/go/src/github.com/TykTechnologies/go-plugins-template build-env-test main > app && chmod +x app && echo "Build 'app'"
```
We mount our current source code to the image, and script included as entrypoint compiles the binary, and out outputs it to SDTOUT.  We redirect output to file and make it executable. 

To build a plugin we should run `make build-plugin PLUGIN_PATH=$(pwd)/plugin-examples/pre PLUGIN_OUTPUT=pre.so`, which internally runs:
```bash
docker run --rm -i -e GOOS=linux -e GOARCH=amd64 -v `pwd`:/go/src/github.com/TykTechnologies/go-plugins-template -v $(pwd)/go-plugins-template/plugin-examples/pre:/plugin build-env-test plugin > pre.so && echo "Build plugin'

```
Interesting part here is that unlike examples before, it supports plugins which depend on external packages. So first it runs `go get` and after, it copies all files from plugin folder, including `vendor` to root of your project (ensuring that none of existing files will be overridden). If there is conflict between vendored packages, it will pick package version used by your main application. 

### Limitations
At the moment build pipeline described above, has a few pitfalls you must know about.

Right now both plugins and main binaries share the the same Docker image: which means that your main binary exposes its source code. While for open source apps it may work, it will be a blocker for closed sourced ones. One of the solutions here, will be separating those pipelines. And the only difference of plugin build pipeline Docker image will be that it will contain only vendor folder of your app (excluding private vendored repos). An alternative to that, will be creating a simple HTTP service, where user can upload plugin source code, and as output receive `so` file.

Another limitation, is that if you try to build binary for `Darvin` (OSX) platform, you will get an error, because Go plugins require to have CGO cross-compilation toolkit, for each platform you want to support. Thankfully there are projects like [https://github.com/karalabe/xgo](https://github.com/karalabe/xgo) which provide pre-build Docker images for exactly this case. 

In future this repository may fix pitfalls described above.
