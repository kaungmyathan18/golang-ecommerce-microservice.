.PHONY: proto proto-install e2e loadtest loadtest-realistic k8s-build k8s-deploy k8s-delete scaffold-all

PROTO_DIR := proto/product
PROTO_OUT := proto/product/pb

proto-install:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

proto:
	@mkdir -p proto/product/pb proto/inventory/pb proto/payment/pb
	protoc --go_out=proto/product/pb --go_opt=paths=source_relative \
		--go-grpc_out=proto/product/pb --go-grpc_opt=paths=source_relative \
		-I proto/product proto/product/product.proto
	protoc --go_out=proto/inventory/pb --go_opt=paths=source_relative \
		--go-grpc_out=proto/inventory/pb --go-grpc_opt=paths=source_relative \
		-I proto/inventory proto/inventory/inventory.proto
	protoc --go_out=proto/payment/pb --go_opt=paths=source_relative \
		--go-grpc_out=proto/payment/pb --go-grpc_opt=paths=source_relative \
		-I proto/payment proto/payment/payment.proto

e2e:
	@chmod +x scripts/e2e-test.sh
	@./scripts/e2e-test.sh

loadtest:
	@chmod +x scripts/load-test.sh
	@./scripts/load-test.sh

loadtest-realistic:
	@chmod +x scripts/load-test.sh
	@LOAD_SCENARIO=realistic LOAD_WORKERS=20 LOAD_DURATION_SEC=30 ./scripts/load-test.sh

k8s-build:
	@chmod +x scripts/k8s-build-images.sh
	@./scripts/k8s-build-images.sh

k8s-deploy:
	@chmod +x scripts/k8s-deploy-kind.sh
	@./scripts/k8s-deploy-kind.sh

k8s-delete:
	kubectl delete -k k8s/overlays/local --ignore-not-found

scaffold-all:
	@echo "Run scaffold commands manually - see README"
