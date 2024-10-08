FROM golang:1.23.2 AS builder

WORKDIR /x
COPY go.mod go.sum main.go .
RUN go build -ldflags='-s -w' .

FROM gcr.io/distroless/static-debian12

WORKDIR /
COPY --from=builder /x/gol .
