package grpcserver

import (
	"context"

	pb "github.com/kaungmyathan18/golang-ecommerce-microservice/proto/inventory/pb"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/inventory-service/internal/repository"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/inventory-service/services/api/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type InventoryServer struct {
	pb.UnimplementedInventoryServiceServer
	svc *service.InventoryService
}

func NewInventoryServer(svc *service.InventoryService) *InventoryServer {
	return &InventoryServer{svc: svc}
}

func (s *InventoryServer) GetStock(ctx context.Context, req *pb.GetStockRequest) (*pb.GetStockResponse, error) {
	qty, err := s.svc.GetStock(ctx, req.GetProductId())
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, status.Error(codes.NotFound, "stock not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.GetStockResponse{ProductId: req.GetProductId(), Quantity: int32(qty)}, nil
}

func (s *InventoryServer) CheckStock(ctx context.Context, req *pb.CheckStockRequest) (*pb.CheckStockResponse, error) {
	ok, stock, err := s.svc.CheckStock(ctx, req.GetProductId(), int(req.GetQuantity()))
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, status.Error(codes.NotFound, "stock not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.CheckStockResponse{Available: ok, CurrentStock: int32(stock)}, nil
}

func (s *InventoryServer) SetStock(ctx context.Context, req *pb.SetStockRequest) (*pb.SetStockResponse, error) {
	qty, err := s.svc.SetStock(ctx, req.GetProductId(), int(req.GetQuantity()))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.SetStockResponse{ProductId: req.GetProductId(), Quantity: int32(qty)}, nil
}

func (s *InventoryServer) DecrementStock(ctx context.Context, req *pb.DecrementStockRequest) (*pb.DecrementStockResponse, error) {
	remaining, err := s.svc.DecrementStock(ctx, req.GetProductId(), int(req.GetQuantity()))
	if err != nil {
		if err == repository.ErrInsufficientStock {
			return nil, status.Error(codes.FailedPrecondition, "insufficient stock")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.DecrementStockResponse{Success: true, RemainingStock: int32(remaining)}, nil
}

func (s *InventoryServer) IncrementStock(ctx context.Context, req *pb.IncrementStockRequest) (*pb.IncrementStockResponse, error) {
	remaining, err := s.svc.IncrementStock(ctx, req.GetProductId(), int(req.GetQuantity()))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.IncrementStockResponse{Success: true, RemainingStock: int32(remaining)}, nil
}
