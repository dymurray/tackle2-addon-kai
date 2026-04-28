GOPATH    ?= $(HOME)/go
GOBIN     ?= $(GOPATH)/bin
GOIMPORTS = $(GOBIN)/goimports
IMG       ?= quay.io/djzager/tackle2-addon-kai:latest

PKG = ./cmd/...
PKGDIR = $(subst /...,,$(PKG))

cmd: fmt vet
	go build -ldflags="-w -s" -o bin/addon ./cmd/addon
	go build -ldflags="-w -s" -o bin/fetch-analysis ./cmd/fetch-analysis

image-docker:
	docker build -t $(IMG) .

image-podman:
	podman build -t $(IMG) .

fmt: $(GOIMPORTS)
	$(GOIMPORTS) -w $(PKGDIR)

vet:
	go vet $(PKG)

# Ensure goimports installed.
$(GOIMPORTS):
	go install golang.org/x/tools/cmd/goimports@v0.24.0
