FROM golang:1.26.0 AS builder

WORKDIR /src

COPY go.mod go.sum .

RUN go mod download

COPY *.go Makefile .

RUN make build

FROM scratch

COPY --from=builder /src/scrubbed /

USER 65532:65532

ENTRYPOINT ["/scrubbed"]
