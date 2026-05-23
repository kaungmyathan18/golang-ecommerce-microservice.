package grpcserver

import (
	"context"

	pb "github.com/kaungmyathan18/golang-ecommerce-microservice/proto/product/pb"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/repository"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/services/api/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ProductServer struct {
	pb.UnimplementedProductServiceServer
	svc *service.CatalogService
}

func NewProductServer(svc *service.CatalogService) *ProductServer {
	return &ProductServer{svc: svc}
}

func (s *ProductServer) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.ProductResponse, error) {
	p, err := s.svc.GetProduct(ctx, req.GetId())
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, status.Error(codes.NotFound, "product not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.ProductResponse{
		Id: p.ID, Name: p.Name, Description: p.Description,
		Price: p.Price, CategoryId: p.CategoryID, Stock: int32(p.Stock),
	}, nil
}

func (s *ProductServer) CheckStock(ctx context.Context, req *pb.CheckStockRequest) (*pb.CheckStockResponse, error) {
	return nil, status.Error(codes.Unimplemented, "use inventory-service CheckStock")
}

func (s *ProductServer) DecrementStock(ctx context.Context, req *pb.DecrementStockRequest) (*pb.DecrementStockResponse, error) {
	return nil, status.Error(codes.Unimplemented, "use inventory-service DecrementStock")
}
