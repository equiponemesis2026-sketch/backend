package services

import (
	"context"
	"sync"
)

// Job es una unidad de análisis asíncrono de un fragmento de audio.
type Job struct {
	AlertID string
	ChunkID string
	Format  string
	Audio   []byte
}

// Pool procesa trabajos de análisis vocal en un número fijo de goroutines.
// El cliente encola y sigue; el procesamiento ocurre en background.
type Pool struct {
	jobs    chan Job
	handler func(context.Context, Job)
	wg      sync.WaitGroup
	once    sync.Once
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewPool crea un pool con `workers` goroutines. El handler recibe cada Job
// con un contexto cancelable al cerrar el pool.
func NewPool(ctx context.Context, workers int, handler func(context.Context, Job)) *Pool {
	if workers < 1 {
		workers = 1
	}
	ctx, cancel := context.WithCancel(ctx)
	p := &Pool{
		jobs:    make(chan Job, workers*2),
		handler: handler,
		ctx:     ctx,
		cancel:  cancel,
	}
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	return p
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case job, ok := <-p.jobs:
			if !ok {
				return
			}
			p.handler(p.ctx, job)
		}
	}
}

// Enqueue encola un trabajo de forma no bloqueante. Si el pool está lleno
// o cerrado, se descarta con seguridad (el análisis es best-effort).
func (p *Pool) Enqueue(job Job) {
	select {
	case p.jobs <- job:
	default:
	}
}

// Shutdown cancela el contexto y cierra el canal de trabajos.
func (p *Pool) Shutdown() {
	p.once.Do(func() {
		p.cancel()
		close(p.jobs)
	})
}

// Wait bloquea hasta que todas las goroutines terminen. Debe llamarse
// después de Shutdown.
func (p *Pool) Wait() {
	p.wg.Wait()
}
