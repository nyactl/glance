BIN      := ~/.local/bin/glance
COV_OUT  := coverage.out
COV_MIN  := 80

.PHONY: build install test vet cover cover-html

build:
	go build -o glance .

install:
	go build -o $(BIN) .

test:
	go test ./...

vet:
	go vet ./...

# The end-to-end tests run a binary built with -cover; what it covers is added to the unit tests.
cover:
	@rm -rf .cover && mkdir -p .cover
	GLANCE_COVERDIR=$(CURDIR)/.cover go test -count=1 ./... -coverpkg=./... -coverprofile=$(COV_OUT) -covermode=atomic
	@go tool covdata textfmt -i=.cover -o .cover/e2e.out
	@tail -n +2 .cover/e2e.out >> $(COV_OUT)
	@go tool cover -func=$(COV_OUT) | tail -1
	@total=$$(go tool cover -func=$(COV_OUT) | tail -1 | awk '{print $$3}' | tr -d '%'); \
	 echo "Coverage: $${total}% (minimum: $(COV_MIN)%)"; \
	 if [ "$$(echo "$${total} < $(COV_MIN)" | bc)" = "1" ]; then \
	   echo "FAIL: coverage below minimum"; exit 1; \
	 fi

cover-html: cover
	go tool cover -html=$(COV_OUT)
