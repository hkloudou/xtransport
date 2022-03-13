.PHONY: default
.DEFAULT_GOAL := default
ifneq ($(shell pwd),$(shell git rev-parse --show-toplevel))
	GIT_SUBPATH=$(subst $(shell git rev-parse --show-toplevel)/,"",$(shell pwd))
	GIT_CLOSEDVERSION = $(shell git describe --match 'packets/mqtt/v[0-9]*\.[0-9]*\.[0-9]*')
endif
xx:
	@echo ${GIT_CLOSEDVERSION}
	@echo ${GIT_SUBPATH}
default:
	-git autotag -commit 'modify' -f -p
	@echo current version:`git describe`
git:
	- git autotag -commit 'auto commit' -t -f -i -p -s packets/mqtt
	@echo current version:`git describe`
retag:
	-git autotag -commit 'retag $(shell git describe --match 'packets/mqtt/v[0-9]*\.[0-9]*\.[0-9]*')' -t -f -p -s packets/mqtt
	@echo current version:`git describe`
git-minor:
	git autotag -commit 'auto commit' -t -f -i -p -l minor  -s packets/mqtt