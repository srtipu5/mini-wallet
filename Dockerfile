# --- build stage ---
FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/mini-wallet ./cmd/api

# --- runtime stage ---
FROM alpine:3.20
RUN adduser -D -H app
USER app
COPY --from=build /out/mini-wallet /usr/local/bin/mini-wallet
EXPOSE 8080
ENTRYPOINT ["mini-wallet"]
