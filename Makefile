APP     := flapjack
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"
OUTDIR  := dist

.PHONY: all clean local run

all: windows linux mac local

$(OUTDIR):
	mkdir -p $(OUTDIR)

windows: $(OUTDIR)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(OUTDIR)/$(APP)-windows-amd64.exe .

linux: $(OUTDIR)
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o $(OUTDIR)/$(APP)-linux-amd64 .

mac: $(OUTDIR)
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o $(OUTDIR)/$(APP)-darwin-amd64 .

mac-arm: $(OUTDIR)
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o $(OUTDIR)/$(APP)-darwin-arm64 .

local: $(OUTDIR)
	go build $(LDFLAGS) -o $(OUTDIR)/$(APP).exe .

run: local
	$(OUTDIR)/$(APP).exe

tidy:
	go mod tidy

clean:
	rm -rf $(OUTDIR)
