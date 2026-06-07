.PHONY: all cli gui clean

DIST   := dist
CLI    := $(DIST)/ancestry-tag-converter
GUI    := $(DIST)/ancestry-tag-converter-gui
GUIPKG := ./cmd/ancestry-tag-converter-gui/

# On Windows (detected by GNU Make via OS=Windows_NT) add .exe suffixes and
# the GUI-subsystem linker flag that suppresses the background console window.
ifeq ($(OS),Windows_NT)
  CLI    := $(CLI).exe
  GUI    := $(GUI).exe
  LFLAGS := -ldflags="-H windowsgui"
endif

all: cli gui

cli: | $(DIST)
	go build -o $(CLI) .

gui: | $(DIST)
	go build $(LFLAGS) -o $(GUI) $(GUIPKG)

$(DIST):
	mkdir -p $@

clean:
	rm -rf $(DIST)
