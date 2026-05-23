FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache gcc musl-dev
COPY notification-service/ ./notification-service/
WORKDIR /src/notification-service/services/worker
ENV GOWORK=off
RUN go mod download
RUN CGO_ENABLED=1 go build -o /out/worker ./cmd/worker

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=build /out/worker ./worker
COPY notification-service/migrations ./migrations
EXPOSE 8083
ENTRYPOINT ["./worker"]
