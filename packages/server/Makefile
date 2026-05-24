VERSION := 1.0.0
BUILDDIR := build
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

all: clean windows linux darwin

clean:
	rm -rf $(BUILDDIR)
	mkdir -p $(BUILDDIR)

windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILDDIR)/server-windows-amd64.exe .
	GOOS=windows GOARCH=386   go build $(LDFLAGS) -o $(BUILDDIR)/server-windows-386.exe .

linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILDDIR)/server-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILDDIR)/server-linux-arm64 .

darwin:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILDDIR)/server-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILDDIR)/server-darwin-arm64 .

.PHONY: all clean windows linux darwin