package productclient

import (
	"context"
	"fmt"
	"time"

	pb "github.com/kaungmyathan18/golang-ecommerce-microservice/proto/product/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn   *grpc.ClientConn
	client pb.ProductServiceClient
}

func New(addr string) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %w", err)
	}
	return &Client{conn: conn, client: pb.NewProductServiceClient(conn)}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) GetProduct(ctx context.Context, id string) (*pb.ProductResponse, error) {
	return c.client.GetProduct(ctx, &pb.GetProductRequest{Id: id})
}
