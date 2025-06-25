BINARIES := $(notdir $(shell find cmd -mindepth 1 -maxdepth 1 -type d))

.PHONY: $(BINARIES)
.PHONY: all
.PHONY: build
.PHONY: vet
.PHONY: staticcheck
.PHONY: lint
.PHONY: clean
.PHONY: install-deps
.PHONY: gen
.PHONY: gen-clean

all: vet lint staticcheck test build
build: $(BINARIES)

$(BINARIES):
	@echo "*** building $@"
	@cd cmd/$@ && go build -o ../../bin/$@

test:
	@echo "*** $@"
	@go test ./...

vet:
	@echo "*** $@"
	@go vet ./...

staticcheck:
	@echo "*** $@"
	@staticcheck ./...

lint:
	@echo "*** $@"
	@revive ./...

clean:
	@echo "*** $@"
	@rm -rf bin
	
gen-clean:
	@echo "*** $@"
	@rm -rf gen

gen: gen-clean
	@echo "*** generating gRPC interface"
	@buf generate

count:
	@gocloc --not-match-d='gen/' .

install-deps:
	@go install github.com/mgechev/revive@latest
	@go install honnef.co/go/tools/cmd/staticcheck@latest
	@go install github.com/bufbuild/buf/cmd/buf@v1.55.1
