# Use an official Go devcontainer base
FROM mcr.microsoft.com/devcontainers/go:1.24

# Install useful tools
RUN apt-get update && apt-get install -y \
    make \
    git \
    curl \
    && rm -rf /var/lib/apt/lists/*

RUN sudo apt update
RUN sudo apt install -y protobuf-compiler
RUN protoc --version

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

RUN export PATH="$PATH:$(go env GOPATH)/bin"

# Optional: set up workspace
WORKDIR /workspace/echopost
