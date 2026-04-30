FROM golang:1.26-alpine3.22@sha256:07e91d24f6330432729082bb580983181809e0a48f0f38ecde26868d4568c6ac AS builder
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

FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11
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
