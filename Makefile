ARCH ?= amd64

WINDOWS_BINS := bin/collector-windows-$(ARCH).exe bin/probe-windows-$(ARCH).exe
LINUX_BINS := bin/collector-linux-$(ARCH) bin/probe-linux-$(ARCH)

ifeq ($(OS),Windows_NT)
SHELL := powershell.exe
.SHELLFLAGS := -NoProfile -ExecutionPolicy Bypass -Command

define BUILD
$$env:GOOS='$(1)'; $$env:GOARCH='$(ARCH)'; go build -o $(2) ./cmd/$(3)
endef

else
SHELL := /bin/sh
.SHELLFLAGS := -c

define BUILD
GOOS=$(1) GOARCH=$(ARCH) go build -o $(2) ./cmd/$(3)
endef
endif

.PHONY: all build build-windows build-linux clean

all: build

build: build-windows build-linux

build-windows: | bin
	$(call BUILD,windows,$(word 1,$(WINDOWS_BINS)),collector)
	$(call BUILD,windows,$(word 2,$(WINDOWS_BINS)),probe)

build-linux: | bin
	$(call BUILD,linux,$(word 1,$(LINUX_BINS)),collector)
	$(call BUILD,linux,$(word 2,$(LINUX_BINS)),probe)

bin:
ifeq ($(OS),Windows_NT)
	New-Item -ItemType Directory -Force -Path bin | Out-Null
else
	mkdir -p bin
endif

clean:
ifeq ($(OS),Windows_NT)
	if (Test-Path bin) { Remove-Item -Recurse -Force -LiteralPath bin }
else
	rm -rf bin
endif
