# +----------------------------------------------------------------------------+
# | system configuration                                                       |
# +----------------------------------------------------------------------------+

# Call to return the first non-empty value from "go env" or a default value.
getgoenv = $(firstword $(foreach e,$(1),$(shell go env $(e))) $(2))

# Call to return each instance of a subpath in each given parent paths.
each = $(wildcard $(addsuffix $(2),$(subst :, ,$(1))))
which = $(firstword $(call each,$(1),$(2)))

ifeq "cygwin" "$(shell uname -o | tr 'A-Z' 'a-z')"
  cygoroot := $(shell cygpath --windows --long-name --absolute $(call getgoenv,GOROOT,$(GOROOT)))
  cygopath := $(shell cygpath --windows --long-name --absolute $(call getgoenv,GOPATH,$(GOPATH)))
  GOROOT   := $(shell cygpath --unix --absolute "$(cygoroot)")
  GOPATH   := $(shell cygpath --unix --absolute "$(cygopath)")
else
  GOROOT   := $(call getgoenv,GOROOT,$(GOROOT))
  GOPATH   := $(call getgoenv,GOPATH,$(GOPATH))
endif

# Verify we have a valid GOPATH, GOROOT that physically exist.
ifeq "" "$(strip $(wildcard $(subst :, ,$(GOPATH))))"
  $(error invalid GOPATH="$(GOPATH)")
else ifeq "" "$(strip $(wildcard $(subst :, ,$(GOROOT))))"
  $(error invalid GOROOT="$(GOROOT)")
endif

# Define which debugger to use (optional; disables optimizations).
# If undefined, optimizations are enabled and debug targets are empty.
# Recognized debuggers: gdb dlv
DEBUG ?= dlv

# Verify environment is suited for whichever debugger (if any) is selected.
ifeq "gdb" "$(DEBUG)"
  # Go source code ships with a GDB Python extension that enables:
  #   -- Inspection of runtime internals (e.g., goroutines)
  #   -- Pretty-printing built-in types (e.g., map, slice, and channel)
  GDBRTL ?= "$(wildcard $(GOROOT)/src/runtime/runtime-gdb.py)"
  # If you have gdb-dashboard available, specify its path here.
  GDBDSH ?= "$(wildcard $(HOME)/.gdb-dashboard)"
endif

# +----------------------------------------------------------------------------+
# | project symbols exported verbatim via go linker                            |
# +----------------------------------------------------------------------------+

PROJECT   ?= resvn
IMPORT    ?= github.com/ardnew/$(PROJECT)
VERSION   ?= 0.11.2
BUILDTIME ?= $(shell date -u '+%FT%TZ')
# If not defined, guess PLATFORM from current GOOS/GOHOSTOS, GOARCH/GOHOSTARCH.
# When none of these are set, fallback on linux-amd64.
PLATFORM  ?=                                                                   \
  $(call getgoenv,GOOS GOHOSTOS,linux)-$(call getgoenv,GOARCH GOHOSTARCH,amd64)

# Determine Git branch and revision (if metadata exists).
ifneq "" "$(GOPATH)/src/$(IMPORT)/.git"
  # Verify we have the Git executable installed on our PATH.
  ifneq "" "$(shell which git)"
    BRANCH   ?= $(shell git symbolic-ref --short HEAD)
    REVISION ?= $(shell git rev-parse --short HEAD)
  endif
endif

# Makefile identifiers to export (as strings) via Go linker. If the project is
# not contained within a Git repository, BRANCH and REVISION will not be defined
# or exported for the application.
EXPORTS ?= PROJECT IMPORT VERSION BUILDTIME PLATFORM                           \
  $(and $(BRANCH),BRANCH) $(and $(REVISION),REVISION)

# +----------------------------------------------------------------------------+
# | build paths and project files                                              |
# +----------------------------------------------------------------------------+

# If the command being built is different than the project import path, define
# GOCMD as that import path. This will be used as the output executable when
# making targets "build", "run", "install", etc. For example, a common practice
# is to place the project's main package in a "cmd" subdirectory.
ifneq "" "$(wildcard cmd/$(PROJECT))"
  # If a directory named PROJECT is found in the "cmd" subdirectory, use it as
  # the main package.
  GOSRC ?= cmd/$(PROJECT)
  GOCMD ?= $(IMPORT)/$(GOSRC)
else
  GOSRC ?= .
  GOCMD ?= # Otherwise, if GOCMD left undefined, use IMPORT.
endif

# Command executable (e.g., targets "build", "run")
BINPATH ?= bin
# Release package (e.g., targets "zip", "tgz", "tbz")
PKGPATH ?= dist

# Consider all Go source files recursively from working directory.
SOURCES ?= $(shell find . -type f -name '*.go')

# Other non-Go source files that may affect build staleness.
METASOURCES ?= Makefile $(wildcard go.mod)

# Other files to include with distribution packages (sort removes duplicates)
EXTRAFILES ?= $(sort $(wildcard LICENSE*) $(wildcard README*)                  \
  $(wildcard *.md) $(wildcard *.rst) $(wildcard *.adoc))

# Go package where the exported symbols will be defined.
EXPORTPATH ?= main

# +----------------------------------------------------------------------------+
# | build flags and configuration                                              |
# +----------------------------------------------------------------------------+

# Append any other build tags needed, separated by a single comma (no space!).
#GOTAGS ?= byollvm
GOTAGS ?=

# Export variables as strings to the selected package, and, if a debugger was
# NOT selected, strip symbol table and omit DWARF debug segments.
GOLDFLAGS ?= -ldflags='$(if $(strip $(DEBUG)),,-w -s )$(foreach                \
  %,$(EXPORTS),-X "$(EXPORTPATH).$(%)=$($(%))")'

# If a debugger was selected, disable most compiler optimizations.
GOGCFLAGS ?= $(and $(strip $(DEBUG)),-gcflags=all='-N -l')

# This is also a good place to add any global build flags you need.
GOFLAGS ?= -v -tags='$(GOTAGS)' $(GOLDFLAGS) $(GOGCFLAGS)

#        +==========================================================+
#   --=<])  YOU SHOULD NOT NEED TO MODIFY ANYTHING BELOW THIS LINE  ([>=--
#        +==========================================================+

# +----------------------------------------------------------------------------+
# | constants and derived variables                                            |
# +----------------------------------------------------------------------------+

# Supported platforms (GOARCH-GOOS):
platforms :=                                                                   \
  linux-amd64 linux-386 linux-arm64 linux-arm                                  \
  darwin-amd64 darwin-arm64                                                    \
  windows-amd64 windows-386                                                    \
  freebsd-amd64 freebsd-386 freebsd-arm                                        \
  android-amd64 android-386 android-arm64 android-arm

# Verify a valid build target was provided.
ifeq "" "$(strip $(filter $(platforms),$(PLATFORM)))"
  $(error unsupported PLATFORM "$(PLATFORM)" (see: "make help"))
endif

# Parse arch (386, amd64, ...) and OS (linux, darwin, ...) from platform.
os   := $(word 1,$(subst -, ,$(PLATFORM)))
arch := $(word 2,$(subst -, ,$(PLATFORM)))

# Output file extensions:
binext := $(and $(filter windows,$(os)),.exe)
tgzext := .tar.gz
tbzext := .tar.bz2
zipext := .zip

env    := \env
echo   := \echo
test   := \test
cd     := \cd
rm     := \rm -rvf
rmdir  := \rmdir
cp     := \cp -rv
mkdir  := \mkdir -p
chmod  := \chmod -v
tail   := \tail
ls     := \ls
grep   := \grep
sed    := \sed
jq     := \jq
gdb    := \gdb
dlv    := \dlv
tmux   := \tmux
tgz    := \tar -czvf
tbz    := \tar -cjvf
zip    := \zip -vr

# Define any environment variables used for each "go" command invocation.
# This would be a good place to set "go env" and cgo/clang variables.
goenv := GOOS="$(os)" GOARCH="$(arch)"

ifneq "" "$(strip $(cygoroot))"
  goenv += GOROOT="$(cygoroot)"
endif
ifneq "" "$(strip $(cygopath))"
  goenv += GOPATH="$(cygopath)"
endif

goenv += $(cgoflag)

# Always call "go" with our relevant clang/cgo flags as well as our Makefile-
# defined target platform, which enables cross-compilation support.
go    := $(goenv) \go
gofmt := $(goenv) \gofmt

# Output paths derived from current configuration:
srcdir := $(or $(GOCMD),$(IMPORT))
bindir := $(GOPATH)/bin
binexe := $(bindir)/$(PROJECT)$(binext)
outdir := $(BINPATH)/$(PLATFORM)
outexe := $(outdir)/$(PROJECT)$(binext)
pkgver := $(PKGPATH)/$(VERSION)
triple := $(PROJECT)-$(VERSION)-$(PLATFORM)

# Since it isn't possible to pass arguments from "make" to the target executable
# (without, e.g., inline variable definitions), we simply use a separate shell
# script that builds the project and calls the executable.
# You can thus call this shell script, and all arguments will be passed along.
# Use "make run" to generate this script in the project root.
runsh := run.sh
define __RUNSH__
#!/bin/sh
# Description:
# 	Rebuild (via "make build") and run $(PROJECT)$(binext) with given arguments.
#
# Usage:
# 	./$(runsh) [arg ...]
#
if make -s build; then
	"$(outexe)" "$${@}"
fi
endef
export __RUNSH__

# The following script is a `jq` filter that will translate sh-style environment
# variable definitions into JSON syntax: 'KEY=VALUE' -> '{"KEY": "VALUE"}'
define __JQFILTER__
  def parse: capture("(?<ident>[^=]*)=(?<value>.*)");
  reduce inputs as $$line ({};
    ($$line | parse) as $$p
    | .[$$p.ident] = ($$p.value) )

endef
export __JQFILTER__

# +----------------------------------------------------------------------------+
# | make targets                                                               |
# +----------------------------------------------------------------------------+

.PHONY: all
all: build

# clean-DIR calls rmdir on DIR if and only if it is an empty directory.
clean-%:
	@$(test) ! -d "$(*)" || $(test) `$(ls) -v "$(*)"` || $(rmdir) -v "$(*)"

.PHONY: flush
flush:
	$(go) clean
	$(rm) "$(outdir)" "$(pkgver)"

.PHONY: clean
clean: tidy flush $(addprefix clean-,$(BINPATH) $(PKGPATH))

.PHONY: tidy
ifneq "" "$(strip $(filter %go.mod,$(METASOURCES)))"
tidy: $(and "$(strip $(filter %go.mod,$(METASOURCES)))",mod) fmt
	@$(go) mod tidy
else
tidy: fmt
endif

.PHONY: fmt
fmt: $(SOURCES) $(METASOURCES)
	@$(gofmt) -e -l -s -w .

.PHONY: mod
mod: go.mod

.PHONY: build
build: tidy $(outexe)

.PHONY: install
install: tidy
	$(go) install $(GOFLAGS) "$(srcdir)"
	@$(echo) " -- success: $(binexe)"

.PHONY: vet
vet: tidy
	$(go) vet "$(IMPORT)" $(and $(GOCMD),"$(GOCMD)")

.PHONY: run
run: $(runsh)

.PHONY: goenv
goenv:
	@# Drop the pipe to generate a sh-compatible environment:
	@#   $(env) -i $(goenv) env
	@$(env) -i $(goenv) env | $(jq) -nR "$$__JQFILTER__"

.PHONY: goflags
goflags:
	@$(echo) $(GOFLAGS)

go.mod:
	@$(go) mod init "$(IMPORT)"

$(outdir) $(pkgver) $(pkgver)/$(triple):
	@$(test) -d "$(@)" || $(mkdir) -v "$(@)"

$(outexe): $(SOURCES) $(METASOURCES) $(outdir)
	$(go) build -o "$(@)" $(GOFLAGS) "$(srcdir)"
	@$(echo) " -- success: $(@)"

$(runsh):
	@$(echo) "$$__RUNSH__" > "$(@)"
	@$(chmod) +x "$(@)"
	@$(echo) " -- success: $(@)"
	@$(echo)
	@# Print the comment block at top of shell script for usage details
	@$(sed) -nE '/^#!/,/^\s*[^#]/{/^\s*#([^!]|$$)/{s/^(\s*)#/  |\1/;p;};}' "$(@)"

.PHONY: debug
debug: vet build
ifeq "gdb" "$(DEBUG)"
	@$(gdb) "$(outexe)"
else ifeq "dlv" "$(DEBUG)"
	@$(dlv) debug "$(or $(GOCMD),$(IMPORT))"
else
	@$(echo) "none: $(DEBUG)"
endif

# +----------------------------------------------------------------------------+
# | targets for creating versioned packages (.zip, .tar.gz, or .tar.bz2)       |
# +----------------------------------------------------------------------------+

.PHONY: zip
zip: $(EXTRAFILES) $(pkgver)/$(triple)$(zipext)

$(pkgver)/%$(zipext): $(outexe) $(pkgver)/%
	$(cp) "$(<)" $(EXTRAFILES) "$(@D)/$(*)"
	@$(cd) "$(@D)" && $(zip) "$(*)$(zipext)" "$(*)"

.PHONY: tgz
tgz: $(EXTRAFILES) $(pkgver)/$(triple)$(tgzext)

$(pkgver)/%$(tgzext): $(outexe) $(pkgver)/%
	$(cp) "$(<)" $(EXTRAFILES) "$(@D)/$(*)"
	@$(cd) "$(@D)" && $(tgz) "$(*)$(tgzext)" "$(*)"

.PHONY: tbz
tbz: $(EXTRAFILES) $(pkgver)/$(triple)$(tbzext)

$(pkgver)/%$(tbzext): $(outexe) $(pkgver)/%
	$(cp) "$(<)" $(EXTRAFILES) "$(@D)/$(*)"
	@$(cd) "$(@D)" && $(tbz) "$(*)$(tbzext)" "$(*)"
