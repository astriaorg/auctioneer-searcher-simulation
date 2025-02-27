package sequencer_client

import (
	"crypto/tls"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"log/slog"
	"net/url"
)

type SequencerClient struct {
	connection *grpc.ClientConn
}

func NewSequencerClient(sequencerUrl string) (*SequencerClient, error) {
	parsedSequencerUrl, err := url.Parse(sequencerUrl)
	if err != nil {
		slog.Error("can not parse url", "err", err)
		return nil, err
	}

	transportCreds := credentials.NewTLS(&tls.Config{
		InsecureSkipVerify: false,
	})

	conn, err := grpc.NewClient(parsedSequencerUrl.String(), grpc.WithTransportCredentials(transportCreds))
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
