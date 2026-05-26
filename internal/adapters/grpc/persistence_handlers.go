package grpc

import (
	"context"
	"sort"
	"time"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/app/engine"
	"github.com/kainhuck/signalix/internal/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultStrategyLogListLimit = 100
	maxStrategyLogListLimit     = 1000
	defaultStrategyLogTail      = 50
	maxStrategyLogTail          = 200
	defaultAccountSnapshotLimit = 5000
	maxAccountSnapshotLimit     = 50000
)

func requireRunningWithStore(eng *engine.Engine) error {
	if err := requireRunning(eng); err != nil {
		return err
	}
	if eng.PersistenceStore() == nil {
		return status.Error(codes.FailedPrecondition, "persistence store not configured")
	}
	return nil
}

func clampStrategyLogListLimit(limit int32) (int, error) {
	if limit < 0 {
		return 0, status.Error(codes.InvalidArgument, "invalid limit")
	}
	if limit == 0 {
		return defaultStrategyLogListLimit, nil
	}
	if int(limit) > maxStrategyLogListLimit {
		return maxStrategyLogListLimit, nil
	}
	return int(limit), nil
}

func clampStrategyLogTail(tail int32) (int, error) {
	if tail < 0 {
		return 0, status.Error(codes.InvalidArgument, "invalid tail")
	}
	if tail == 0 {
		return defaultStrategyLogTail, nil
	}
	if int(tail) > maxStrategyLogTail {
		return maxStrategyLogTail, nil
	}
	return int(tail), nil
}

func clampAccountSnapshotLimit(limit int32) (int, error) {
	if limit < 0 {
		return 0, status.Error(codes.InvalidArgument, "invalid limit")
	}
	if limit == 0 {
		return defaultAccountSnapshotLimit, nil
	}
	if int(limit) > maxAccountSnapshotLimit {
		return maxAccountSnapshotLimit, nil
	}
	return int(limit), nil
}

func optionalTimeFromUnixMs(ms int64) *time.Time {
	if ms == 0 {
		return nil
	}
	t := time.UnixMilli(ms).UTC()
	return &t
}

func (s *EngineService) ListStrategyLogs(ctx context.Context, req *enginev1.ListStrategyLogsRequest) (*enginev1.ListStrategyLogsReply, error) {
	if err := requireRunningWithStore(s.eng); err != nil {
		return nil, err
	}
	if req == nil {
		req = &enginev1.ListStrategyLogsRequest{}
	}
	start := optionalTimeFromUnixMs(req.GetStartAtUnixMs())
	end := optionalTimeFromUnixMs(req.GetEndAtUnixMs())
	if start != nil && end != nil && start.After(*end) {
		return nil, status.Error(codes.InvalidArgument, "invalid time range")
	}
	limit, err := clampStrategyLogListLimit(req.GetLimit())
	if err != nil {
		return nil, err
	}

	rows, err := s.eng.PersistenceStore().ListStrategyLogs(ctx, models.StrategyLogListFilter{
		StrategyName: req.GetStrategyName(),
		StartAt:      start,
		EndAt:        end,
		Limit:        limit,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list strategy logs: %v", err)
	}
	out := make([]*enginev1.StrategyLog, 0, len(rows))
	for _, row := range rows {
		out = append(out, strategyLogToProto(row))
	}
	return &enginev1.ListStrategyLogsReply{Logs: out}, nil
}

func (s *EngineService) SubscribeStrategyLogs(req *enginev1.SubscribeStrategyLogsRequest, stream grpc.ServerStreamingServer[enginev1.StrategyLog]) error {
	if err := requireRunningWithStore(s.eng); err != nil {
		return err
	}
	if req == nil {
		req = &enginev1.SubscribeStrategyLogsRequest{}
	}
	tail, err := clampStrategyLogTail(req.GetTail())
	if err != nil {
		return err
	}
	ctx := stream.Context()
	rows, err := s.eng.PersistenceStore().ListRecentStrategyLogs(ctx, req.GetStrategyName(), tail)
	if err != nil {
		return status.Errorf(codes.Internal, "list recent strategy logs: %v", err)
	}
	tailMaxID := int64(0)
	for _, row := range rows {
		if row != nil && row.ID > tailMaxID {
			tailMaxID = row.ID
		}
	}
	rowsASC := append([]*models.StrategyLogRow(nil), rows...)
	sort.Slice(rowsASC, func(i, j int) bool {
		if rowsASC[i].CreatedAt.Equal(rowsASC[j].CreatedAt) {
			return rowsASC[i].ID < rowsASC[j].ID
		}
		return rowsASC[i].CreatedAt.Before(rowsASC[j].CreatedAt)
	})

	ch := s.eng.RegisterStrategyLogSubscriber(ctx, req.GetStrategyName(), tailMaxID, 64)
	for _, row := range rowsASC {
		if row == nil {
			continue
		}
		if err := stream.Send(strategyLogToProto(row)); err != nil {
			return err
		}
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case row, ok := <-ch:
			if !ok {
				return nil
			}
			if row == nil {
				continue
			}
			if err := stream.Send(strategyLogToProto(row)); err != nil {
				return err
			}
		}
	}
}

func (s *EngineService) ListAccountSnapshots(ctx context.Context, req *enginev1.ListAccountSnapshotsRequest) (*enginev1.ListAccountSnapshotsReply, error) {
	if err := requireRunningWithStore(s.eng); err != nil {
		return nil, err
	}
	if req == nil {
		req = &enginev1.ListAccountSnapshotsRequest{}
	}
	start := optionalTimeFromUnixMs(req.GetStartAtUnixMs())
	end := optionalTimeFromUnixMs(req.GetEndAtUnixMs())
	if start != nil && end != nil && start.After(*end) {
		return nil, status.Error(codes.InvalidArgument, "invalid time range")
	}
	limit, err := clampAccountSnapshotLimit(req.GetLimit())
	if err != nil {
		return nil, err
	}

	rows, err := s.eng.PersistenceStore().ListAccountSnapshots(ctx, models.AccountSnapshotListFilter{
		StartAt: start,
		EndAt:   end,
		Limit:   limit,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list account snapshots: %v", err)
	}
	out := make([]*enginev1.AccountSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, accountSnapshotToProto(ctx, row, req.GetIncludeDetails()))
	}
	return &enginev1.ListAccountSnapshotsReply{Snapshots: out}, nil
}

func (s *EngineService) GetLatestAccountSnapshot(ctx context.Context, req *enginev1.GetLatestAccountSnapshotRequest) (*enginev1.GetLatestAccountSnapshotReply, error) {
	if err := requireRunningWithStore(s.eng); err != nil {
		return nil, err
	}
	includeDetails := false
	if req != nil {
		includeDetails = req.GetIncludeDetails()
	}
	row, err := s.eng.PersistenceStore().GetLatestAccountSnapshot(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get latest account snapshot: %v", err)
	}
	if row == nil {
		return &enginev1.GetLatestAccountSnapshotReply{}, nil
	}
	return &enginev1.GetLatestAccountSnapshotReply{
		Snapshot: accountSnapshotToProto(ctx, row, includeDetails),
	}, nil
}
