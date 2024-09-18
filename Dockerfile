FROM golang:1.23 AS builder

WORKDIR /src

COPY go.mod go.sum .

RUN go mod download

COPY *.go Makefile .

RUN make build

FROM registry.access.redhat.com/ubi9/ubi-micro:9.4

COPY --from=builder /src/scrubbed /usr/local/bin/

EXPOSE 8080

EXPOSE 8443

ENTRYPOINT ["/usr/local/bin/scrubbed"]