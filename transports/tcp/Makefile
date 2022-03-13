.PHONY: default
.DEFAULT_GOAL := default
default:
	-git autotag -commit 'modify' -f -p
	@echo current version:`git describe`
git:
	- git autotag -commit 'auto commit' -t -f -i -p -s transports/tcp
	@echo current version:`git describe`
retag:
	-git autotag -commit 'retag $(shell git describe --match 'transports/tcp/v[0-9]*\.[0-9]*\.[0-9]*'')' -t -f -p -s transports/tcp
	@echo current version:`git describe`
git-minor:
	git autotag -commit 'auto commit' -t -f -i -p -l minor  -s transports/tcp