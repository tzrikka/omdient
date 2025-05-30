package thrippy

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	thrippypb "github.com/tzrikka/thrippy-api/thrippy/v1"
)

const (
	timeout = 3 * time.Second
)

// Connection creates a gRPC client connection to the given Thrippy server address.
// It supports both secure and insecure connections, based on the given credentials.
func Connection(addr string, creds credentials.TransportCredentials) (*grpc.ClientConn, error) {
	return grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
}

// LinkSecrets returns the saved secrets of a given Thrippy link. This
// function reports gRPC errors, but if the link is not found it returns nil.
func LinkSecrets(ctx context.Context, grpcAddr string, creds credentials.TransportCredentials, linkID string) (map[string]string, error) {
	l := zerolog.Ctx(ctx)

	conn, err := Connection(grpcAddr, creds)
	if err != nil {
		l.Error().Stack().Err(err).Send()
		return nil, err
	}
	defer conn.Close()

	c := thrippypb.NewThrippyServiceClient(conn)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resp, err := c.GetCredentials(ctx, thrippypb.GetCredentialsRequest_builder{
		LinkId: proto.String(linkID),
	}.Build())
	if err != nil {
		if status.Code(err) != codes.NotFound {
			l.Error().Stack().Err(err).Send()
			return nil, err
		}
		return nil, nil
	}

	return resp.GetCredentials(), nil
}
