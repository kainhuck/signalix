package spot

import (
	"context"
	"fmt"

	"github.com/kainhuck/signalix/internal/models"
)

type safeExecutor struct {
	events chan *models.OrderEvent
}

func newSafeExecutor() *safeExecutor {
	ch := make(chan *models.OrderEvent)
	close(ch)
	return &safeExecutor{events: ch}
}

func (s *safeExecutor) Place(ctx context.Context, o *models.Order) (string, error) {
	_ = ctx
	_ = o
	return "", fmt.Errorf("spot executor not implemented")
}

func (s *safeExecutor) Cancel(ctx context.Context, o *models.Order) error {
	_ = ctx
	_ = o
	return fmt.Errorf("spot executor not implemented")
}

func (s *safeExecutor) Sync(ctx context.Context, o *models.Order) (*models.OrderEvent, error) {
	_ = ctx
	_ = o
	return nil, fmt.Errorf("spot executor not implemented")
}

func (s *safeExecutor) OrderEvents() <-chan *models.OrderEvent {
	return s.events
}
