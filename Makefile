build:
	@if not exist data mkdir data
	go build -o mariazinha.exe ./cmd/bot

run: build
	./mariazinha.exe

dev:
	@if not exist data mkdir data
	go run ./cmd/bot

deps:
	go mod tidy

.PHONY: build run dev deps
