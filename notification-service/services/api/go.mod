module github.com/kaungmyathan18/golang-ecommerce-microservice/notification-service/services/api

go 1.22

require (
	github.com/kaungmyathan18/golang-ecommerce-microservice/notification-service/internal v0.0.0
	github.com/go-chi/chi/v5 v5.0.12
	github.com/go-chi/cors v1.2.1
	go.uber.org/zap v1.27.0
)

replace github.com/kaungmyathan18/golang-ecommerce-microservice/notification-service/internal => ../../internal
