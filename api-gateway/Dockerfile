FROM golang:1.22-alpine AS build
WORKDIR /src
COPY api-gateway/ ./
RUN go mod download
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=build /out/server ./server
EXPOSE 8080
ENTRYPOINT ["./server"]
