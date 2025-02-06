package sequencer_client

import (
	primitivev1 "buf.build/gen/go/astria/primitives/protocolbuffers/go/astria/primitive/v1"
	"buf.build/gen/go/astria/sequencerblock-apis/grpc/go/astria/sequencerblock/optimistic/v1alpha1/optimisticv1alpha1grpc"
	optimisticv1alpha1 "buf.build/gen/go/astria/sequencerblock-apis/protocolbuffers/go/astria/sequencerblock/optimistic/v1alpha1"
	"context"
	"crypto/sha256"
	"google.golang.org/grpc"
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

	optimisticBlockStreamReq := &optimisticv1alpha1.GetOptimisticBlockStreamRequest{
		RollupId: o.GetRollupId(),
	}
	optimisticBlockStream, err := client.GetOptimisticBlockStream(context.Background(), optimisticBlockStreamReq)
	if err != nil {
		slog.Error("open stream error", "err", err)
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
