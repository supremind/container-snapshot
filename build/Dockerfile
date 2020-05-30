FROM golang:1.13-alpine AS provision

ARG GOPROXY
ENV GO111MODULE=on
WORKDIR /src

COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY . .
RUN go mod verify


FROM provision AS build-operator
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/_output/operator ./cmd/manager

FROM provision AS build-worker
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/_output/worker ./cmd/worker


FROM alpine:3.11 AS release-operator
COPY --from=build-operator /src/build/_output/operator /usr/local/bin/container-snapshot-operator
ENTRYPOINT [ "container-snapshot-operator" ]

FROM alpine:3.11 AS release-worker
COPY --from=build-worker /src/build/_output/worker /usr/local/bin/container-snapshot-worker
ENTRYPOINT [ "container-snapshot-worker" ]
