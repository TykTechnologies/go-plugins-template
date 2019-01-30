BUILD_ENV_VERSION=latest
BUILD_ENV_IMAGE=build-env-test:$(BUILD_ENV_VERSION)
GOOS=linux
GOARCH=amd64
DOCKER_ARGS= -e GOOS=$(GOOS) -e GOARCH=$(GOARCH)
OUT=app
PLUGIN_OUT=plugin.so
ifeq ($(VERSION),)
	DOCKER_ARGS+=-v `pwd`:/go/src/github.com/TykTechnologies/go-plugins-template
else
	DOCKER_ARGS+=-v VERSION=$(VERSION)
endif

ifneq ($(PLUGIN_PATH),)
	DOCKER_ARGS+=-v $(PLUGIN_PATH):/plugin
endif


build:
	docker run --rm -i $(DOCKER_ARGS) build-env-test main > $(OUT) && chmod +x $(OUT) && echo "Build '$(OUT)'"

build-plugin:
	docker run --rm -i $(DOCKER_ARGS) build-env-test plugin > $(PLUGIN_OUT) && echo "Build plugin' $(PLUGIN_OUT)'"

build-local:
	go build -i -o $(OUT) && echo "Build '$(OUT)'"

build-local-plugin:
	go build -buildmode=plugin -o $(PLUGIN_OUT) $(PLUGIN_PATH)
