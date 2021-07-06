# 1: build executable binary
FROM golang:alpine AS builder

# Install git, which is required for fetching the dependencies.
RUN apk update && apk add --no-cache git

WORKDIR /bin

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go test -tags=acceptance -c -v -o acceptance-tests

# 2: build small image
FROM scratch

# Copy out static executable.
COPY --from=builder /bin/acceptance-tests /bin/acceptance-tests

CMD [ "/bin/acceptance-tests", "-test.v", "-test.failfast" ]
