.PHONY: default
.DEFAULT_GOAL := default
default:
	-git autotag -commit 'modify' -f -p
	@echo current version:`git describe`
git:
	- git autotag -commit 'auto commit' -t -f -i -p
	@echo current version:`git describe`
retag:
	-git autotag -commit 'modify $(shell git describe)' -t -f -p
	@echo current version:`git describe`