FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.work go.work.sum* ./
COPY internal/go.mod internal/go.sum* ./internal/
COPY services/api/go.mod services/api/go.sum* ./services/api/
RUN cd services/api && go mod download
COPY internal/ ./internal/
COPY services/api/ ./services/api/
RUN cd services/api && CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=build /out/server ./server
EXPOSE 8080
ENTRYPOINT ["./server"]
