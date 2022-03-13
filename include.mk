.PHONY: default
.DEFAULT_GOAL := default

ifneq ($(shell pwd),$(shell git rev-parse --show-toplevel))
	GIT_SUBPATH=$(subst $(shell git rev-parse --show-toplevel)/,"",$(shell pwd))
	GIT_CLOSEDVERSION = $(shell git describe --abbrev=0  --match ${GIT_SUBPATH}/v[0-9]*\.[0-9]*\.[0-9]*)
else
	GIT_SUBPATH=""
	GIT_CLOSEDVERSION = $(shell git describe --abbrev=0  --match v[0-9]*\.[0-9]*\.[0-9]*)
endif
print:
	@echo sub: ${GIT_SUBPATH}
	@echo close: ${GIT_CLOSEDVERSION}
default:
	-git autotag -commit 'modify ${GIT_SUBPATH}' -f -p
	@echo current version:`git describe`
git:
	- git autotag -commit 'auto commit ${GIT_SUBPATH}' -t -f -i -p -s ${GIT_SUBPATH}
	@echo current version:`git describe`
retag:
	-git autotag -commit 'retag $(GIT_CLOSEDVERSION)' -t -f -p -s ${GIT_SUBPATH}
	@echo current version:`git describe`
git-minor:
	git autotag -commit 'auto commit ${GIT_SUBPATH}' -t -f -i -p -l minor  -s ${GIT_SUBPATH}