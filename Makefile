include ./include.mk

# include.mk sets its auto-tagging target as the default; a bare 'make'
# must build, not publish tags.
.DEFAULT_GOAL := build

MODULES = . ./packets/mqtt ./transports/tcp ./transports/ws ./transports/quic ./interop

.PHONY: build test vet race

build:
	@set -e; for m in $(MODULES); do echo "build $$m"; (cd $$m && go build ./...); done

test:
	@set -e; for m in $(MODULES); do echo "test $$m"; (cd $$m && go test ./...); done

race:
	@set -e; for m in $(MODULES); do echo "race $$m"; (cd $$m && go test -race -count=1 ./...); done

vet:
	@set -e; for m in $(MODULES); do echo "vet $$m"; (cd $$m && go vet ./...); done

fuzz:
	cd packets/mqtt && go test -fuzz=FuzzReadPacket -fuzztime=60s . && go test -fuzz=FuzzValidatePattern -fuzztime=30s .

# downstream simulates what a consumer's 'go get' resolves: every module
# built against its *tagged* dependencies instead of the workspace.
downstream:
	@set -e; for m in $(MODULES); do echo "downstream $$m"; (cd $$m && GOWORK=off go build ./... && GOWORK=off go vet ./...); done
