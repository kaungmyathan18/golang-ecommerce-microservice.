FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.work go.work.sum* ./
COPY internal/go.mod internal/go.sum* ./internal/
COPY services/worker/go.mod services/worker/go.sum* ./services/worker/
RUN cd services/worker && go mod download
COPY internal/ ./internal/
COPY services/worker/ ./services/worker/
RUN cd services/worker && CGO_ENABLED=0 go build -o /out/worker ./cmd/worker

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=build /out/worker ./worker
EXPOSE 8081
ENTRYPOINT ["./worker"]
