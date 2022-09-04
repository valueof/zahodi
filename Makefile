all: build

dev:
	go run . -addr localhost:9999 -dev

watch:
	nodemon --exec go run . -addr=localhost:9999 -dev --signal SIGTERM --ext html,go

prod:
	go run . --addr localhost:9999

build:
	go build .