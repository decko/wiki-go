default:
    @just --list

# Build the binary
build:
    go build -o wiki-go .

# Build and run the wiki server
run: build
    ./wiki-go

# Link a knowledge base directory into wiki-go's data
link path:
    mkdir -p data/documents
    ln -sfn "$(realpath {{path}})" data/documents/knowledge
    @echo "Linked {{path}} → data/documents/knowledge"

# Stop the running wiki server
stop:
    -kill $(lsof -ti:8080) 2>/dev/null
    @echo "Stopped"

# Restart: stop, build, run
restart: stop build run
