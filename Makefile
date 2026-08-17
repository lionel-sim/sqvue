APP_NAME := sqvue
BIN_DIR := bin

.PHONY: run build clean db-setup

run:
	go run ./...

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

build: $(BIN_DIR)
	go build -o $(BIN_DIR)/$(APP_NAME) .

db-setup:
	./scripts/db-setup.sh

clean:
	rm -rf $(BIN_DIR)
