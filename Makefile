test:
	go test ./... -count=1 -v

build:
	go build -o bin/chaos-master main.go

run:
	go run main.go --config.file=config/example/example_simple_config.yml