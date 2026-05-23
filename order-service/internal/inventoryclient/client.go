package inventoryclient

import (
	"context"
	"fmt"
	"time"

	pb "github.com/kaungmyathan18/golang-ecommerce-microservice/proto/inventory/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn   *grpc.ClientConn
	client pb.InventoryServiceClient
}

func New(addr string) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("inventory grpc dial: %w", err)
	}
	return &Client{conn: conn, client: pb.NewInventoryServiceClient(conn)}, nil
}

func (c *Client) Close() error { return c.conn.Close() }

func (c *Client) CheckStock(ctx context.Context, productID string, qty int32) (*pb.CheckStockResponse, error) {
	return c.client.CheckStock(ctx, &pb.CheckStockRequest{ProductId: productID, Quantity: qty})
}

func (c *Client) DecrementStock(ctx context.Context, productID string, qty int32) (*pb.DecrementStockResponse, error) {
	return c.client.DecrementStock(ctx, &pb.DecrementStockRequest{ProductId: productID, Quantity: qty})
}
