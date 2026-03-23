FROM golang:1.22-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /resolver ./cmd/resolver

FROM alpine:3.19

RUN apk add --no-cache ca-certificates
COPY --from=builder /resolver /usr/local/bin/resolver

ENTRYPOINT ["resolver"]
