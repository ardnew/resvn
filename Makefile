SHELL := $(firstword $(shell which bash sh))

modpkg := $(shell basename $(shell go list -f '{{.Target}}' .))
moddir := $(shell go list -f '{{.Dir}}' .)
modimp := $(shell go list -f '{{.ImportPath}}' .)

_ := $(shell git fetch --tags --quiet 2>/dev/null)
semver := $(patsubst v%,%,$(or $(VERSION),$(shell git describe --tags --abbrev=0 2>/dev/null)))

PROJECT   ?= $(modpkg)
IMPORT    ?= $(modimp)
VERSION   ?= $(semver)
BUILDTIME ?= $(shell date -u '+%FT%TZ')

exports   ?= PROJECT IMPORT VERSION BUILDTIME
goldflags ?= -ldflags='$(if $(strip $(DEBUG)),,-w -s )$(foreach %,$(exports),-X "main.$(%)=$($(%))")'
gogcflags ?= $(and $(strip $(DEBUG)),-gcflags=all='-N -l')
goflags   ?= -v $(goldflags) $(gogcflags)

output := dist
assets := README.md LICENSE

platforms := $(foreach os,linux darwin windows,$(foreach arch,amd64 arm64,$(os)-$(arch)))
platform  := $(or $(PLATFORM),$(platforms))

dist      := $(foreach p,$(platform),dist-$(p))
clean     := $(foreach p,$(platform),clean-$(p))
distclean := $(foreach p,$(platform),distclean-$(p))

os = $(firstword $(subst -, ,$(1)))
arch = $(lastword $(subst -, ,$(1)))

.PHONY: all generate version dist clean distclean $(platform) $(dist) $(clean) $(distclean)

# double-hyphen prevents usage from command-line.
# Make will interpret it as an invalid option and exit.
.PHONY: --force

all: $(platform)

# An empty recipe is always considered out of date.
# Any targets that depend on it will always be rebuilt.
--force:

bump-major bump-minor bump-patch:
	@v=$$(go tool over --$(subst bump-,,$@) < $(moddir)/VERSION) && echo "$$v" > $(moddir)/VERSION
	@echo "$(moddir) version $$(cat $(moddir)/VERSION)"

generate: version
	go generate -v ./...

version: $(moddir)/VERSION
	@echo "$(moddir) version $(shell cat $(moddir)/VERSION)"

dist: $(dist)

clean: $(clean)

distclean: $(distclean)

$(moddir)/VERSION: --force
ifeq ($(strip $(semver)),)
	$(error unknown version: set VERSION or tag the repository)
endif
	@echo $(semver) > $@

$(platform): generate
	@echo
	@echo build $@
	@echo
	@mkdir -p $(output)/$(modpkg)$(semver).$@
	GOOS=$(call os,$@) GOARCH=$(call arch,$@) go build $(goflags) -o $(output)/$(modpkg)$(semver).$@/$(modpkg) .

.SECONDEXPANSION:
$(dist): $$(subst dist-,,$$@)
	@echo
	@echo dist $<
	@echo
	@cp $(assets) $(output)/$(modpkg)$(semver).$</
	tar -czf $(output)/$(modpkg)$(semver).$<.tar.gz -C $(output) $(modpkg)$(semver).$<

$(clean):
	@echo
	@echo clean $(subst clean-,,$@)
	@echo
	GOOS=$(call os,$@) GOARCH=$(call arch,$@) go clean -i -r $(modimp)

.SECONDEXPANSION:
$(distclean): $$(subst distclean-,clean-,$$@)
	@echo
	@echo distclean $(subst clean-,,$<)
	@echo
	rm -rf $(output)/$(modpkg)$(semver).$(subst clean-,,$<)
	rm -f $(output)/$(modpkg)$(semver).$(subst clean-,,$<).tar.gz
