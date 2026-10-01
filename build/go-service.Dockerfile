FROM golang:1.26.5-alpine AS build

ARG SERVICE
ARG COMMAND

WORKDIR /src

COPY platform/go.mod platform/go.sum ./platform/
COPY services/${SERVICE}/go.mod services/${SERVICE}/go.sum ./services/${SERVICE}/
RUN cd services/${SERVICE} && go mod download

COPY platform ./platform
COPY services/${SERVICE} ./services/${SERVICE}

RUN cd services/${SERVICE} && \
    CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/service \
    ./cmd/${COMMAND}

FROM alpine:3.22

RUN apk add --no-cache ca-certificates && \
    addgroup -S -g 65532 app && \
    adduser -S -D -H -u 65532 -G app app

WORKDIR /app
COPY --from=build --chown=65532:65532 /out/service ./service

USER 65532:65532
ENTRYPOINT ["/app/service"]
