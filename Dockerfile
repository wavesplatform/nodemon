FROM golang:1.27-alpine3.24@sha256:cf6fca6641884b8433441b2b0652976f975e1d0fdd26d177eaaf8596087f3125 AS builder
ARG DIR=/app
WORKDIR ${DIR}

ARG APP=nodemon
ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache make git
# disable cgo for go build
ENV CGO_ENABLED=0

COPY go.mod .
COPY go.sum .

RUN go mod download

# Copy the .git directory and restore the worktree, also handle current possible changes in go.mod and go.sum
COPY .git .git
RUN git restore --source=HEAD --worktree .
COPY go.mod .
COPY go.sum .

# Copy the necessary files for building and override the restored worktree
COPY Makefile .
COPY cmd .
COPY pkg .
COPY internal internal

RUN make build-$APP-$TARGETOS-$TARGETARCH

FROM alpine:3.24@sha256:5b02b42e375f7426f8d65c3af331ca05d9878f9989230354504e0b9dfd431f60
ARG DIR=/app
ENV TZ=Etc/UTC \
    APP_USER=appuser

ARG APP=nodemon
ARG TARGETOS
ARG TARGETARCH

STOPSIGNAL SIGINT

RUN addgroup -S $APP_USER \
    && adduser -S $APP_USER -G $APP_USER

RUN apk add --no-cache bind-tools

USER $APP_USER
WORKDIR ${DIR}
# Considered as a default HTTP API Port, NATS embedded server port
EXPOSE 8080
EXPOSE 4222

COPY --from=builder ${DIR}/build/$TARGETOS-$TARGETARCH/$APP ${DIR}/$APP

ENTRYPOINT ["./$APP"]
