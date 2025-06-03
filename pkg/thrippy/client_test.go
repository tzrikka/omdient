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
	"google.golang.org/protobuf/proto"

	thrippypb "github.com/tzrikka/thrippy-api/thrippy/v1"
)

type server struct {
	thrippypb.UnimplementedThrippyServiceServer
	linkResp  *thrippypb.GetLinkResponse
	credsResp *thrippypb.GetCredentialsResponse
	err       error
}

func (s *server) GetLink(_ context.Context, _ *thrippypb.GetLinkRequest) (*thrippypb.GetLinkResponse, error) {
	return s.linkResp, s.err
}

func (s *server) GetCredentials(_ context.Context, _ *thrippypb.GetCredentialsRequest) (*thrippypb.GetCredentialsResponse, error) {
	return s.credsResp, s.err
}

func TestLinkData(t *testing.T) {
	tests := []struct {
		name         string
		linkResp     *thrippypb.GetLinkResponse
		credsResp    *thrippypb.GetCredentialsResponse
		respErr      error
		wantTemplate string
		wantSecrets  map[string]string
		wantErr      bool
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
			name:    "link_not_found",
			respErr: status.Error(codes.NotFound, "link not found"),
		},
		{
			name: "existing_link_without_secrets",
			linkResp: thrippypb.GetLinkResponse_builder{
				Template: proto.String("template"),
			}.Build(),
			credsResp:    thrippypb.GetCredentialsResponse_builder{}.Build(),
			wantTemplate: "template",
		},
		{
			name: "happy_path",
			linkResp: thrippypb.GetLinkResponse_builder{
				Template: proto.String("template"),
			}.Build(),
			credsResp: thrippypb.GetCredentialsResponse_builder{
				Credentials: map[string]string{"aaa": "111", "bbb": "222"},
			}.Build(),
			wantTemplate: "template",
			wantSecrets:  map[string]string{"aaa": "111", "bbb": "222"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lis, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			s := grpc.NewServer()
			thrippypb.RegisterThrippyServiceServer(s, &server{
				linkResp:  tt.linkResp,
				credsResp: tt.credsResp,
				err:       tt.respErr,
			})
			go func() {
				_ = s.Serve(lis)
			}()

			template, secrets, err := LinkData(t.Context(), lis.Addr().String(), insecureCreds(), "link ID")
			if (err != nil) != tt.wantErr {
				t.Errorf("LinkData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if template != tt.wantTemplate {
				t.Errorf("LinkData() template = %q, want %q", template, tt.wantTemplate)
			}
			if !reflect.DeepEqual(secrets, tt.wantSecrets) {
				t.Errorf("LinkData() secrets = %v, want %v", secrets, tt.wantSecrets)
			}
		})
	}
}
