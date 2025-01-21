package sequencer_client

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log/slog"
)

type SequencerClient struct {
	connection *grpc.ClientConn
}

func NewSequencerClient(url string) (*SequencerClient, error) {
	conn, err := grpc.NewClient(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("can not connect with server %v", err)
		return nil, err
	}
	return &SequencerClient{
		connection: conn,
	}, nil
}

func (s *SequencerClient) GetConnection() *grpc.ClientConn {
	return s.connection
}
