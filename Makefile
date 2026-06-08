.PHONY: all cli gui clean

DIST   := dist
CLI    := $(DIST)/ancestry-tag-converter
GUI    := $(DIST)/ancestry-tag-converter-gui
GUIPKG := ./cmd/ancestry-tag-converter-gui/

# Version reported by the GUI's About dialog. Derived from the latest git tag
# (leading "v" stripped); falls back to "dev" outside a git checkout. Override
# with `make gui VERSION=1.2.3`.
VERSION    ?= $(patsubst v%,%,$(shell git describe --tags --always --dirty 2>/dev/null || echo dev))
GUI_LFLAGS := -X main.version=$(VERSION)

# On Windows (detected by GNU Make via OS=Windows_NT) add .exe suffixes and
# the GUI-subsystem linker flag that suppresses the background console window.
ifeq ($(OS),Windows_NT)
  CLI        := $(CLI).exe
  GUI        := $(GUI).exe
  GUI_LFLAGS := $(GUI_LFLAGS) -H windowsgui
endif

all: cli gui

cli: | $(DIST)
	go build -o $(CLI) .

gui: | $(DIST)
	go build -ldflags="$(GUI_LFLAGS)" -o $(GUI) $(GUIPKG)

$(DIST):
	mkdir -p $@

clean:
	rm -rf $(DIST)
