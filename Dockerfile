FROM golang:1.26.1-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/smartrun ./cmd/app

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /out/smartrun /app/smartrun
COPY ui ./ui
COPY migrations ./migrations

EXPOSE 8080

ENTRYPOINT ["/app/smartrun"]