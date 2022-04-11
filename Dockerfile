# Build application first
FROM docker.io/golang:1.18.0 AS builder

ENV CGO_ENABLED=0
ARG GIT_BUILD_HASH
ENV GIT_BUILD_HASH=$GIT_BUILD_HASH

COPY . /go/src/beryju.org/docker-to-oauth

RUN cd /go/src/beryju.org/docker-to-oauth && \
    go build -ldflags "-X main.buildCommit=$GIT_BUILD_HASH" -v -o /go/bin/docker-oauth

# Final container
FROM gcr.io/distroless/static-debian11:debug

COPY --from=builder /go/bin/docker-oauth /docker-oauth

EXPOSE 8001

ENTRYPOINT [ "/docker-oauth" ]
