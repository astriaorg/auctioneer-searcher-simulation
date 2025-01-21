package sequencer_client

import (
	"buf.build/gen/go/astria/sequencerblock-apis/grpc/go/astria/sequencerblock/optimistic/v1alpha1/optimisticv1alpha1grpc"
	optimisticv1alpha1 "buf.build/gen/go/astria/sequencerblock-apis/protocolbuffers/go/astria/sequencerblock/optimistic/v1alpha1"
	"context"
	"google.golang.org/grpc"
)

type BlockCommitmentStreamConnectionInfo struct {
	sequencerClient *SequencerClient
}

func NewBlockCommitmentStreamConnectionInfo(sequencerClient *SequencerClient) *BlockCommitmentStreamConnectionInfo {
	return &BlockCommitmentStreamConnectionInfo{
		sequencerClient: sequencerClient,
	}
}

func (b *BlockCommitmentStreamConnectionInfo) GetSequencerClient() *SequencerClient {
	return b.sequencerClient
}

type BlockCommitmentStream struct {
	streamConnectionInfo *BlockCommitmentStreamConnectionInfo
	streamingClient      *grpc.ServerStreamingClient[optimisticv1alpha1.GetBlockCommitmentStreamResponse]
}

func (b *BlockCommitmentStreamConnectionInfo) GetBlockCommitmentStream() (*BlockCommitmentStream, error) {
	// create stream
	client := optimisticv1alpha1grpc.NewOptimisticBlockServiceClient(b.GetSequencerClient().GetConnection())

	committedBlockStreamReq := &optimisticv1alpha1.GetBlockCommitmentStreamRequest{}
	committedBlockStream, err := client.GetBlockCommitmentStream(context.Background(), committedBlockStreamReq)
	if err != nil {
		return nil, err
	}

	return &BlockCommitmentStream{
		streamConnectionInfo: b,
		streamingClient:      &committedBlockStream,
	}, nil
}

// TODO - abstract away only the methods
func (b *BlockCommitmentStream) GetStreamClient() grpc.ServerStreamingClient[optimisticv1alpha1.GetBlockCommitmentStreamResponse] {
	return *b.streamingClient
}
