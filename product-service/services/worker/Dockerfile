FROM golang:1.22-alpine AS build
WORKDIR /src
COPY product-service/ ./product-service/
WORKDIR /src/product-service/services/worker
ENV GOWORK=off
RUN go mod download
RUN CGO_ENABLED=0 go build -o /out/worker ./cmd/worker

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=build /out/worker ./worker
ENTRYPOINT ["./worker"]
