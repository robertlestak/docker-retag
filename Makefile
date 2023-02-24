VERSION=v0.0.1

bin: bin/docker-retag_darwin bin/docker-retag_linux bin/docker-retag_windows

bin/docker-retag_darwin:
	mkdir -p bin
	GOOS=darwin GOARCH=amd64 go build -ldflags="-X 'main.Version=$(VERSION)'" -o bin/docker-retag_darwin cmd/docker-retag/*.go
	openssl sha512 bin/docker-retag_darwin > bin/docker-retag_darwin.sha512

bin/docker-retag_linux:
	mkdir -p bin
	GOOS=linux GOARCH=amd64 go build -ldflags="-X 'main.Version=$(VERSION)'" -o bin/docker-retag_linux cmd/docker-retag/*.go
	openssl sha512 bin/docker-retag_linux > bin/docker-retag_linux.sha512

bin/docker-retag_windows:
	mkdir -p bin
	GOOS=windows GOARCH=amd64 go build -ldflags="-X 'main.Version=$(VERSION)'" -o bin/docker-retag_windows cmd/docker-retag/*.go
	openssl sha512 bin/docker-retag_windows > bin/docker-retag_windows.sha512

.PHONY: docker
docker:
	docker build -t docker-retag .