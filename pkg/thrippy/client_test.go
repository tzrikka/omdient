package thrippy

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	thrippypb "github.com/tzrikka/thrippy-api/thrippy/v1"
)

type server struct {
	thrippypb.UnimplementedThrippyServiceServer
	resp *thrippypb.GetCredentialsResponse
	err  error
}

func (s *server) GetCredentials(_ context.Context, _ *thrippypb.GetCredentialsRequest) (*thrippypb.GetCredentialsResponse, error) {
	return s.resp, s.err
}

func TestLinkSecrets(t *testing.T) {
	tests := []struct {
		name    string
		resp    *thrippypb.GetCredentialsResponse
		respErr error
		want    map[string]string
		wantErr bool
	}{
		{
			name: "nil",
		},
		{
			name:    "grpc_error",
			respErr: errors.New("error"),
			wantErr: true,
		},
		{
			name: "no_secrets",
			resp: thrippypb.GetCredentialsResponse_builder{}.Build(),
		},
		{
			name:    "link_not_found",
			respErr: status.Error(codes.NotFound, "link not found"),
		},
		{
			name: "happy_path",
			resp: thrippypb.GetCredentialsResponse_builder{
				Credentials: map[string]string{"aaa": "111", "bbb": "222"},
			}.Build(),
			want: map[string]string{"aaa": "111", "bbb": "222"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lis, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			s := grpc.NewServer()
			thrippypb.RegisterThrippyServiceServer(s, &server{resp: tt.resp, err: tt.respErr})
			go func() {
				_ = s.Serve(lis)
			}()

			got, err := LinkSecrets(t.Context(), lis.Addr().String(), insecureCreds(), "link ID")
			if (err != nil) != tt.wantErr {
				t.Errorf("LinkSecrets() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LinkSecrets() = %v, want %v", got, tt.want)
			}
		})
	}
}
