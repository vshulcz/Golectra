package agent

import (
	"context"
	"log"
	"sync"

	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/ports"
)

type BatchPublisher struct {
	pub     ports.Publisher
	workers int
	jobs    chan []domain.Metrics
	wg      sync.WaitGroup
}

func NewBatchPublisher(pub ports.Publisher, workers int) *BatchPublisher {
	if workers < 1 {
		workers = 1
	}
	buf := workers * 2
	return &BatchPublisher{
		pub:     pub,
		workers: workers,
		jobs:    make(chan []domain.Metrics, buf),
	}
}

func (bp *BatchPublisher) Start(ctx context.Context) {
	for i := 0; i < bp.workers; i++ {
		bp.wg.Add(1)
		go func(id int) {
			defer bp.wg.Done()
			for batch := range bp.jobs {
				if len(batch) == 0 {
					continue
				}
				if err := bp.pub.SendBatch(ctx, batch); err != nil {
					log.Printf("agent: worker[%d]: batch send failed (%v), fallback to single requests", id, err)
					for _, m := range batch {
						if err := bp.pub.SendOne(ctx, m); err != nil {
							log.Printf("agent: worker[%d]: send single failed (%s/%s): %v", id, m.MType, m.ID, err)
						}
					}
				}
			}
		}(i + 1)
	}
}

func (bp *BatchPublisher) Stop() {
	close(bp.jobs)
	bp.wg.Wait()
}

func (bp *BatchPublisher) Submit(batch []domain.Metrics) {
	bp.jobs <- batch
}
