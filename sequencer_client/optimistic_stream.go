package sequencer_client

import (
	primitivev1 "buf.build/gen/go/astria/primitives/protocolbuffers/go/astria/primitive/v1"
	"buf.build/gen/go/astria/sequencerblock-apis/grpc/go/astria/sequencerblock/optimistic/v1alpha1/optimisticv1alpha1grpc"
	"buf.build/gen/go/astria/sequencerblock-apis/grpc/go/astria/sequencerblock/v1/sequencerblockv1grpc"
	optimisticv1alpha1 "buf.build/gen/go/astria/sequencerblock-apis/protocolbuffers/go/astria/sequencerblock/optimistic/v1alpha1"
	sequencerblockv1 "buf.build/gen/go/astria/sequencerblock-apis/protocolbuffers/go/astria/sequencerblock/v1"
	"context"
	"crypto/sha256"
	"google.golang.org/grpc"
	"log"
	"log/slog"
)

type OptimisticStreamConnectionInfo struct {
	sequencerClient *SequencerClient
	rollupId        *primitivev1.RollupId
}

func NewOptimisticStreamConnectionInfo(sequencerClient *SequencerClient, rollupName string) *OptimisticStreamConnectionInfo {
	rollupHash := sha256.Sum256([]byte(rollupName))
	rollupId := primitivev1.RollupId{Inner: rollupHash[:]}

	return &OptimisticStreamConnectionInfo{
		sequencerClient: sequencerClient,
		rollupId:        &rollupId,
	}
}

func (o *OptimisticStreamConnectionInfo) GetRollupId() *primitivev1.RollupId {
	return o.rollupId
}

func (o *OptimisticStreamConnectionInfo) GetSequencerClient() *SequencerClient {
	return o.sequencerClient
}

type OptimisticStream struct {
	streamConnectionInfo *OptimisticStreamConnectionInfo
	streamingClient      *grpc.ServerStreamingClient[optimisticv1alpha1.GetOptimisticBlockStreamResponse]
}

func (o *OptimisticStreamConnectionInfo) GetOptimisticStream() (*OptimisticStream, error) {
	// create stream
	client := optimisticv1alpha1grpc.NewOptimisticBlockServiceClient(o.GetSequencerClient().GetConnection())

	seqClient := sequencerblockv1grpc.NewSequencerServiceClient(o.GetSequencerClient().GetConnection())

	slog.Info("querying sequencer block", "sequencer_url", o.GetSequencerClient().GetConnection().Target())
	_, err := seqClient.GetSequencerBlock(context.Background(), &sequencerblockv1.GetSequencerBlockRequest{
		Height: 3000000,
	})
	if err != nil {
		log.Fatalf("cannot query sequencer block %v", err)
		return nil, err
	}

	optimisticBlockStreamReq := &optimisticv1alpha1.GetOptimisticBlockStreamRequest{
		RollupId: o.GetRollupId(),
	}
	optimisticBlockStream, err := client.GetOptimisticBlockStream(context.Background(), optimisticBlockStreamReq)
	if err != nil {
		log.Fatalf("open stream error %v", err)
		return nil, err
	}

	return &OptimisticStream{
		streamConnectionInfo: o,
		streamingClient:      &optimisticBlockStream,
	}, nil
}

// TODO - abstract this better
func (o *OptimisticStream) GetStreamClient() grpc.ServerStreamingClient[optimisticv1alpha1.GetOptimisticBlockStreamResponse] {
	return *o.streamingClient
}
