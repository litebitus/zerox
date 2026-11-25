COVERAGE_FLOOR = 90
GO_MODULES = . client/go
COMPLETION_DIR ?= $(HOME)/.zsh/completions

.PHONY: build completion completion-clean test

build: test
	go build -o bin/zerox .

completion: build
	@mkdir -p $(COMPLETION_DIR)
	@{ printf '#compdef zerox ./bin/zerox\n'; ./bin/zerox completion zsh | tail -n +2; } > $(COMPLETION_DIR)/_zerox
	@echo "wrote $(COMPLETION_DIR)/_zerox; binds zerox and ./bin/zerox"
	@grep -q '$(patsubst $(HOME)/%,%,$(COMPLETION_DIR))' $(HOME)/.zshrc 2>/dev/null || \
		echo "add to ~/.zshrc before compinit:  fpath=($(COMPLETION_DIR) \$$fpath)"
	@echo "it shadows a brew-installed zerox's completion until 'make completion-clean'"
	@printf 'run now: rm -f ~/.zcompdump*; exec zsh\n'

completion-clean:
	rm -f $(COMPLETION_DIR)/_zerox
	@echo "removed; run 'exec zsh' (rm -f ~/.zcompdump* if the old one lingers)"

test:
	go -C client/go generate ./...
	@unformatted="$$(gofmt -l .)"; if [ -n "$$unformatted" ]; then echo "$$unformatted" >&2; exit 1; fi
	@for module in $(GO_MODULES); do \
		go -C $$module vet ./... || exit 1; \
		packages="$$(go -C $$module list ./... | paste -sd , -)"; \
		go -C $$module test -count=1 -timeout 5s -coverpkg="$$packages" -coverprofile=coverage.out ./... || exit 1; \
		go -C $$module tool cover -func=coverage.out | \
			awk -v floor=$(COVERAGE_FLOOR) -v module=$$module \
				'/^total:/ { sub(/%/, "", $$3); print "Go coverage of " module ": " $$3 "%"; exit !($$3 >= floor) }' || \
			{ echo "Go coverage of $$module is below $(COVERAGE_FLOOR)%" >&2; exit 1; }; \
	done
