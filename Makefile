APP_NAME := sqvue
BIN_DIR := bin

.PHONY: run build clean

run:
	go run ./...

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

build: $(BIN_DIR)
	go build -o $(BIN_DIR)/$(APP_NAME) .

clean:
	rm -rf $(BIN_DIR)
