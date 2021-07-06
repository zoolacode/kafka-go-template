# 1: build executable binary
FROM golang:alpine AS builder

# Install git, which is required for fetching the dependencies.
RUN apk update && apk add --no-cache git

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o main

# 2: build small image
FROM scratch

# Copy out static executable.
COPY --from=builder /app/main /app/main

ENTRYPOINT [ "/app/main" ]