##
# This image uses multi-stage builds
##
FROM golang:1.11.2-alpine3.8 AS build-env
ARG pkg

RUN mkdir -p /go/src/${pkg}
WORKDIR /go/src/${pkg}

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
	github.com/golang/dep/cmd/dep \
	golang.org/x/lint/golint

##
# Install/update project dependencies
##
COPY ./Gopkg.lock ./
COPY ./Gopkg.toml ./
RUN dep ensure -vendor-only

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
COPY ./server.go ./server.go

##
# Build
##
RUN task test build:binary

##
# Build runtime image
##
FROM alpine:latest
ARG pkg

WORKDIR /app
COPY --from=build-env /go/src/${pkg}/builds .

CMD ["./server"]
