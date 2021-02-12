.PHONY: all clean bin/netsoc

VERSION := latest

default: proxy

proxy:
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(GOLDFLAGS)" -o $@

dev:
	cat tools.go | sed -nr 's|^\t_ "(.+)"$$|\1|p' | xargs -tI % go get %
	CompileDaemon -exclude-dir=.git -build="go build " -command="./proxy" -graceful-kill

clean:
	-rm -f proxy
