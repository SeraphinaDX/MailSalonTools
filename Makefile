.PHONY: build test fmt

build:
	go build -o MailSalonTools ./cmd/MailSalonTools

test:
	go test ./...

fmt:
	gofmt -w $$(find . -name '*.go' -type f)
