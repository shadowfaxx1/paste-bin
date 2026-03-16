FROM golang:1.23-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/personal-api ./cmd/api

FROM alpine:3.21

RUN addgroup -S app && adduser -S -G app app

WORKDIR /app
COPY --from=build /out/personal-api /usr/local/bin/personal-api

USER app
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/personal-api"]