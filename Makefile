include ./include.mk

MODULES = . ./packets/mqtt ./transports/tcp ./transports/ws ./transports/quic

.PHONY: build test vet race

build:
	@set -e; for m in $(MODULES); do echo "build $$m"; (cd $$m && go build ./...); done

test:
	@set -e; for m in $(MODULES); do echo "test $$m"; (cd $$m && go test ./...); done

race:
	@set -e; for m in $(MODULES); do echo "race $$m"; (cd $$m && go test -race -count=1 ./...); done

vet:
	@set -e; for m in $(MODULES); do echo "vet $$m"; (cd $$m && go vet ./...); done
