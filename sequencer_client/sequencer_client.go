package sequencer_client

import (
	"crypto/tls"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"log/slog"
)

type SequencerClient struct {
	connection *grpc.ClientConn
}

func NewSequencerClient(url string) (*SequencerClient, error) {
	creds := credentials.NewTLS(&tls.Config{
		InsecureSkipVerify: false, // Ensure server certificate validation
	})

	conn, err := grpc.NewClient(url, grpc.WithTransportCredentials(creds))
	if err != nil {
		slog.Error("can not connect with server", "err", err)
		return nil, err
	}
	return &SequencerClient{
		connection: conn,
	}, nil
}

func (s *SequencerClient) GetConnection() *grpc.ClientConn {
	return s.connection
}
