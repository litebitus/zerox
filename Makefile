COVERAGE_FLOOR = 90
GO_MODULES = . client/go

.PHONY: build test

build: test
	go build -o bin/zerox .

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
