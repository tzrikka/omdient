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

// LinkData returns the template name and saved secrets of a given Thrippy link.
// This function reports gRPC errors, but if the link is not found it returns nothing.
func LinkData(ctx context.Context, grpcAddr string, creds credentials.TransportCredentials, linkID string) (string, map[string]string, error) {
	l := zerolog.Ctx(ctx)

	conn, err := Connection(grpcAddr, creds)
	if err != nil {
		l.Error().Stack().Err(err).Send()
		return "", nil, err
	}
	defer conn.Close()

	c := thrippypb.NewThrippyServiceClient(conn)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Template.
	resp1, err := c.GetLink(ctx, thrippypb.GetLinkRequest_builder{
		LinkId: proto.String(linkID),
	}.Build())
	if err != nil {
		if status.Code(err) != codes.NotFound {
			l.Error().Stack().Err(err).Send()
			return "", nil, err
		}
		return "", nil, nil
	}

	// Credentials.
	resp2, err := c.GetCredentials(ctx, thrippypb.GetCredentialsRequest_builder{
		LinkId: proto.String(linkID),
	}.Build())
	if err != nil {
		l.Error().Stack().Err(err).Send()
		return "", nil, err
	}

	return resp1.GetTemplate(), resp2.GetCredentials(), nil
}

// LinkTemplate returns the template name of a given Thrippy link. This function
// reports gRPC errors, but if the link is not found it returns an empty string.
func LinkTemplate(ctx context.Context, grpcAddr string, creds credentials.TransportCredentials, linkID string) (string, error) {
	l := zerolog.Ctx(ctx)

	conn, err := Connection(grpcAddr, creds)
	if err != nil {
		l.Error().Stack().Err(err).Send()
		return "", err
	}
	defer conn.Close()

	c := thrippypb.NewThrippyServiceClient(conn)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resp, err := c.GetLink(ctx, thrippypb.GetLinkRequest_builder{
		LinkId: proto.String(linkID),
	}.Build())
	if err != nil {
		if status.Code(err) != codes.NotFound {
			l.Error().Stack().Err(err).Send()
			return "", err
		}
		return "", nil
	}

	return resp.GetTemplate(), nil
}
