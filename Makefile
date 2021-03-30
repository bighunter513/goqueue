all:test

test:
	cd q && go test -v -race

.PHONY: test

