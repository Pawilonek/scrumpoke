# -------------------
# Builder
# -------------------
FROM golang:1.26-alpine AS builder

RUN mkdir /app
WORKDIR /app

COPY go.mod go.sum /app/
RUN go mod download

ADD . /app

RUN go build -o bin/scrumpoke ./cmd/scrumpoke



# -------------------
# Production
# -------------------
FROM alpine:latest AS production
RUN mkdir /app
WORKDIR /app

COPY --from=builder /app/bin /app
COPY --from=builder /app/static /app/static

CMD ["./scrumpoke"]
