FROM golang:1.13-alpine3.11 as builder

RUN apk update && apk upgrade && \
    apk add --no-cache bash build-base

WORKDIR /opt/db-operator

# to reduce docker build time download dependency first before building
COPY go.mod .
COPY go.sum .
RUN go mod download

# build
COPY . .
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags build -o /usr/local/bin/db-operator ./cmd/manager

FROM alpine:3.11
LABEL maintainer="dev@kloeckner-i.com"

ENV USER_UID=1001
ENV USER_NAME=db-operator

# # install operator binary
COPY --from=builder /usr/local/bin/db-operator /usr/local/bin/db-operator
COPY ./build/bin /usr/local/bin
RUN /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]
USER ${USER_UID}