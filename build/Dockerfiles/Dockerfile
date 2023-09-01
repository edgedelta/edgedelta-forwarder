# FROM public.ecr.aws/edgedelta/ed-lambda-extension:latest as extensions-layer
ARG BUILDPLATFORM=linux/amd64
FROM --platform=$BUILDPLATFORM golang:alpine AS builder

ENV GO111MODULE=on
ARG TARGETOS=linux
ARG TARGETARCH=amd64

# Required for establishing https calls
RUN apk update && apk add --no-cache ca-certificates && update-ca-certificates

# Copy files from local to workdir
WORKDIR /go/src/github.com/edgedelta/edgedelta
ADD . .

# Build and install log compactor
WORKDIR /go/src/github.com/edgedelta/edgedelta/tools/logs_compactor
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build \
    -ldflags="-s -w" \
    -o /go/bin/logcompactor

FROM scratch

# Import from builder.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Move already built binary.
COPY --from=builder /go/bin/logcompactor  /edgedelta/logcompactor

# Extension  code
# WORKDIR /opt/extensions
# COPY --from=extensions-layer /opt/extensions/ .

# No need use to entrypoint from parent image
ENTRYPOINT []

# Working directory can not be empty to run in aws lambda, based on below error
# IMAGE Launch error: the working directory '' is invalid, it needs to be an absolute path
WORKDIR /

# Run log compactor service
CMD  ["/edgedelta/logcompactor"]