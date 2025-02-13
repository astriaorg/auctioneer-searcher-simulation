package sequencer_client

import (
	"crypto/tls"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"net/url"
)

type SequencerClient struct {
	connection *grpc.ClientConn
}

func NewSequencerClient(sequencerUrl string) (*SequencerClient, error) {
	parsedSequencerUrl, err := url.Parse(sequencerUrl)
	if err != nil {
		fmt.Printf("can not parse url: err: %s\n", err.Error())
		return nil, err
	}

	transportCreds := credentials.NewTLS(&tls.Config{
		InsecureSkipVerify: true,
	})

	conn, err := grpc.NewClient(parsedSequencerUrl.String(), grpc.WithTransportCredentials(transportCreds))
	if err != nil {
		fmt.Printf("can not connect with server: err: %s\n", err.Error())
		return nil, err
	}
	return &SequencerClient{
		connection: conn,
	}, nil
}

func (s *SequencerClient) GetConnection() *grpc.ClientConn {
	return s.connection
}
