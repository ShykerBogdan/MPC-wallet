# Justfiles are better Makefiles. 
# Install the `just` command from here https://github.com/casey/just

default:
  @just --list

compile: clean
	GOOS=darwin GOARCH=amd64  go build -ldflags  -o bin/thresher main.go

compile-windows:
	GOOS=windows GOARCH=amd64 go build -o ./bin/thresher.exe main.go

compile-linux:
	GOOS=linux GOARCH=amd64   go build -ldflags -o ./bin/thresher-linux-x64 main.go

compile-linux-arm:
	GOOS=linux GOARCH=arm64   go build -ldflags -o ./bin/thresher-linux-arm64 main.go

clean:
	/bin/rm -f bin/*

# Quickly show the main UI for development
ui: 
	bin/thresher --config alice.json testui

# Initialize alice, bob, and cam as users.
initusers:
	bin/thresher init ethereum goerli DAO-Treasury alice cc3412da-0240-4b3a-8c98-557ee614ded2
	bin/thresher init ethereum goerli DAO-Treasury bob be211327-d388-4ba2-8a7d-6c7be21d3c72
	bin/thresher init ethereum goerli DAO-Treasury cam ac6c532b-941a-4328-8896-bb85805c1ccc

alice:
	bin/thresher --config DAO-Treasury-alice.json --log alice.log wallet

bob:
	bin/thresher --config DAO-Treasury-bob.json --log bob.log wallet

cam: 
	bin/thresher --config DAO-Treasury-cam.json --log cam.log wallet

