package processor

import (
	"context"
	"time"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/config"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/queue"

	"go.uber.org/zap"
)

// Processor polls the queue and handles messages.
type Processor struct {
	cfg   config.WorkerConfig
	queue *queue.Client
	log   *zap.Logger
}

func New(cfg config.WorkerConfig, q *queue.Client, log *zap.Logger) *Processor {
	return &Processor{cfg: cfg, queue: q, log: log}
}

func (p *Processor) Start(ctx context.Context) {
	for i := 0; i < p.cfg.NumWorkers; i++ {
		i := i
		go p.loop(ctx, i)
	}
}

func (p *Processor) loop(ctx context.Context, id int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		msg, err := p.queue.Dequeue(ctx, p.cfg.QueueName, 2*time.Second)
		if err != nil {
			continue
		}
		p.log.Info("processing", zap.Int("worker", id), zap.String("payload", msg))
		if err := p.Handle(ctx, msg); err != nil {
			p.log.Error("handle failed", zap.Int("worker", id), zap.Error(err))
		}
	}
}

// Handle processes a single queue message. Extend this with your business logic.
func (p *Processor) Handle(_ context.Context, payload string) error {
	p.log.Info("handled payload", zap.String("payload", payload))
	return nil
}
