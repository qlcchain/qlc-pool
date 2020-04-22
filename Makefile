.PHONY: clean lint changelog snapshot release
.PHONY: build
.PHONY: deps

# Check for required command tools to build or stop immediately
EXECUTABLES = git go find pwd
K := $(foreach exec,$(EXECUTABLES),\
        $(if $(shell which $(exec)),some string,$(error "No $(exec) in PATH)))

# miner
VERSION ?= v1.3.2
BINARY = gqlc-pool

BUILDDIR = build
GITREV = $(shell git rev-parse --short HEAD)
BUILDTIME = $(shell date +'%FT%TZ%z')
GO_BUILDER_VERSION=v1.14.2

deps:
	go get -u github.com/golangci/golangci-lint/cmd/golangci-lint
	go get -u github.com/git-chglog/git-chglog/cmd/git-chglog

build:
	go build -ldflags "-X main.Version=${VERSION} \
		-X main.GitRev=${GITREV} \
		-X main.BuildTime=${BUILDTIME} \
		-X main.Mode=MainNet" \
		-v -i -o $(shell pwd)/$(BUILDDIR)/gqlc-pool $(shell pwd)/
	@echo "Build pool done."

clean:
	rm -rf $(shell pwd)/$(BUILDDIR)/

changelog:
	git-chglog $(VERSION) > CHANGELOG.md

snapshot:
	docker run --rm --privileged \
		-v $(CURDIR):/qlc-pool \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(GOPATH)/src:/go/src \
		-w /qlc-pool \
		goreng/golang-cross:$(GO_BUILDER_VERSION) --snapshot --rm-dist

release: changelog
	docker run --rm --privileged \
		-e GITHUB_TOKEN=$(GITHUB_TOKEN) \
		-v $(CURDIR):/qlc-pool \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(GOPATH)/src:/go/src \
		-w /qlc-pool \
		goreng/golang-cross:$(GO_BUILDER_VERSION) --rm-dist --release-notes=CHANGELOG.md

lint: 
	golangci-lint run --fix
