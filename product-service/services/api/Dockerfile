FROM golang:1.22-alpine AS build
WORKDIR /src
COPY product-service/ ./product-service/
COPY proto/ ./proto/
WORKDIR /src/product-service/services/api
ENV GOWORK=off
RUN go mod download
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=build /out/server ./server
EXPOSE 8081 9091
ENTRYPOINT ["./server"]
