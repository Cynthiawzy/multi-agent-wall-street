// Command server starts the MarketService gRPC server, publishing trade and
// order-book events to Redis for the WebSocket bridge (bridge/main.py) to
// relay to the dashboard.
package main

import (
	"context"
	"log"
	"net"
	"os"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	pb "github.com/cynthia/simulated-wall-street/proto/marketpb"
	"github.com/cynthia/simulated-wall-street/server"
)

const (
	defaultGRPCAddr  = ":50051"
	defaultRedisAddr = "localhost:6379"
)

// redisPublisher adapts *redis.Client to server.Publisher.
type redisPublisher struct {
	client *redis.Client
}

func (p *redisPublisher) Publish(ctx context.Context, channel string, payload []byte) error {
	return p.client.Publish(ctx, channel, payload).Err()
}

func main() {
	grpcAddr := envOrDefault("GRPC_ADDR", defaultGRPCAddr)
	redisAddr := envOrDefault("REDIS_ADDR", defaultRedisAddr)

	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("connecting to redis at %s: %v", redisAddr, err)
	}
	defer redisClient.Close()
	log.Printf("connected to redis at %s", redisAddr)

	srv := server.NewServer(&redisPublisher{client: redisClient})

	listener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("listening on %s: %v", grpcAddr, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterMarketServiceServer(grpcServer, srv)

	log.Printf("MarketService gRPC server listening on %s", grpcAddr)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
