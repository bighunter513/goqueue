all:test

test:
	cd q && go test -v -race

lfqueue:
	cd q && go test -v -race -run TestLFQueue


.PHONY: test lfqueue

