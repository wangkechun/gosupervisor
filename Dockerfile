FROM golang:1.12
RUN curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(go env GOPATH)/bin v1.16.0
WORKDIR /src
COPY /go.mod /go.sum /src/
RUN go mod download
COPY / /src
RUN go test ./...
RUN golangci-lint run
