##
# This image uses multi-stage builds
##
FROM golang:1.11.2-alpine3.8 AS build-env

RUN mkdir -p /app
WORKDIR /app

##
# System dependencies
##
RUN apk update \
	&& apk add \
		git \
		protobuf \
		protobuf-dev \
		gcc \
		musl-dev \
	&& rm -rf /var/cache/apk/*

##
# Language-specific tools and global dependencies
##
RUN go get -u \
	github.com/golang/protobuf/protoc-gen-go \
	github.com/twitchtv/twirp/protoc-gen-twirp \
	github.com/go-task/task/cmd/task \
	golang.org/x/lint/golint

##
# Install/update project dependencies
##
COPY ./go.mod ./
COPY ./go.sum ./
RUN go get

##
# Generate code for messages
##
COPY ./Taskfile.yml ./
COPY ./src/proto  ./src/proto
RUN task proto

##
# Add project source code to the image
##
COPY ./src ./src
COPY ./app.go ./app.go

##
# Build
##
RUN task build:binary

##
# Build runtime image
##
FROM alpine:latest
ARG pkg

WORKDIR /app
COPY --from=build-env /app/builds .

CMD ["./server"]
