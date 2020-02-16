BUILDPATH=$(CURDIR)
BINPATH=$(BUILDPATH)/bin
PKGPATH=$(BUILDPATH)/pkg

GO=$(shell which go)
GOGET=$(GO) get

PLATFORMS := darwin/386 darwin/amd64 linux/386 linux/amd64 windows/386 windows/amd64 freebsd/386
PLATFORM = $(subst /, ,$@)
OS = $(word 1, $(PLATFORM))
ARCH = $(word 2, $(PLATFORM))

EXENAME=slit
CMDSOURCES = $(wildcard cmd/slit/*.go)
GOBUILD=$(GO) build

.PHONY: makedir build test clean prepare default all $(PLATFORMS)
.DEFAULT_GOAL := default

makedir:
	@echo -n "make directories... "
	@if [ ! -d $(BINPATH) ] ; then mkdir -p $(BINPATH) ; fi
	@if [ ! -d $(PKGPATH) ] ; then mkdir -p $(PKGPATH) ; fi
	@echo ok

build:
	@echo -n "run build... "
	@$(GOBUILD) -o $(BINPATH)/$(EXENAME) -ldflags="-w -s" $(CMDSOURCES)
	@echo ok

test:
	@echo -n "Validating with go fmt..."
	@go fmt $$(go list ./... | grep -v /vendor/)
	@echo ok
	@echo -n "Validating with go vet..."
	@go vet $$(go list ./... | grep -v /vendor/)
	@echo ok

clean:
	@echo -n "clean directories... "
	@rm -rf $(BINPATH)
	@rm -rf $(PKGPATH)
	@rm -rf $(BUILDPATH)/src
	@echo ok

prepare: test makedir

default: prepare build

$(PLATFORMS):
	@echo -n "build $(OS)/$(ARCH)... "
	$(eval EXT := $(shell if [ "$(OS)" = "windows" ]; then echo .exe; fi))
	@GOOS=$(OS) GOARCH=$(ARCH) $(GOBUILD) -o $(BINPATH)/$(EXENAME)_$(OS)_$(ARCH)$(EXT) $(CMDSOURCES)
	@echo ok

all: default $(PLATFORMS)
