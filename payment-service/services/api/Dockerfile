FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache gcc musl-dev
COPY payment-service/ ./payment-service/
WORKDIR /src/payment-service/services/api
ENV GOWORK=off
RUN go mod download
RUN CGO_ENABLED=1 go build -o /out/server ./cmd/server

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY payment-service/migrations ./migrations
COPY --from=build /out/server ./server
EXPOSE 8085
ENTRYPOINT ["./server"]
