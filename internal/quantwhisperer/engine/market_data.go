package engine

import (
	"context"
	"log/slog"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/ritiksuman07/quantflow/internal/quantwhisperer"
	"github.com/ritiksuman07/quantflow/internal/quantwhisperer/broker"
)

type Tick = quantwhisperer.Tick

type MarketDataProvider interface {
	Subscribe(symbol string) (<-chan Tick, error)
	Close() error
}

type SyntheticProvider struct {
	brokerName string
	interval   time.Duration
	maxTicks   int
	rng        *rand.Rand

	mu       sync.Mutex
	cancel   context.CancelFunc
	started  bool
	lastBase float64
	logger   *slog.Logger
}

func NewSyntheticProvider(brokerName string, interval time.Duration, maxTicks int, logger *slog.Logger) *SyntheticProvider {
	if logger == nil {
		logger = fallbackLogger("market_data")
	}
	return &SyntheticProvider{
		brokerName: brokerName,
		interval:   interval,
		maxTicks:   maxTicks,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
		lastBase:   100 + rand.Float64()*75,
		logger:     logger,
	}
}

func (p *SyntheticProvider) Subscribe(symbol string) (<-chan Tick, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		return nil, ErrAlreadySubscribed
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.started = true

	out := make(chan Tick, 8)
	go p.runSynthetic(ctx, symbol, out)
	return out, nil
}

func (p *SyntheticProvider) runSynthetic(ctx context.Context, symbol string, out chan<- Tick) {
	defer close(out)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for i := 0; i < p.maxTicks; i++ {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			out <- p.nextTick(symbol)
		}
	}
}

func (p *SyntheticProvider) nextTick(symbol string) Tick {
	jump := p.rng.NormFloat64() * 0.35
	p.lastBase = math.Max(1, p.lastBase+jump)
	spread := math.Max(0.02, p.lastBase*0.0008)
	volume := int64(100 + p.rng.Intn(600))
	return Tick{
		Broker:    p.brokerName,
		Symbol:    symbol,
		Timestamp: time.Now().UTC(),
		LastPrice: p.lastBase,
		Bid:       p.lastBase - spread,
		Ask:       p.lastBase + spread,
		Volume:    volume,
	}
}

func (p *SyntheticProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	return nil
}

type LiveProvider struct {
	broker   broker.Client
	interval time.Duration
	maxTicks int

	mu      sync.Mutex
	cancel  context.CancelFunc
	started bool
	logger  *slog.Logger
}

func NewLiveProvider(client broker.Client, interval time.Duration, maxTicks int, logger *slog.Logger) *LiveProvider {
	if logger == nil {
		logger = fallbackLogger("market_data")
	}
	return &LiveProvider{
		broker:   client,
		interval: interval,
		maxTicks: maxTicks,
		logger:   logger,
	}
}

func (p *LiveProvider) Subscribe(symbol string) (<-chan Tick, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		return nil, ErrAlreadySubscribed
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.started = true

	in, errs := p.broker.SubscribeWebSocket(ctx, symbol, p.interval, p.maxTicks)
	out := make(chan Tick, 8)
	go p.forward(ctx, in, errs, out)
	return out, nil
}

func (p *LiveProvider) forward(ctx context.Context, in <-chan quantwhisperer.Tick, errs <-chan error, out chan<- Tick) {
	defer close(out)
	for {
		select {
		case <-ctx.Done():
			return
		case tick, ok := <-in:
			if !ok {
				return
			}
			out <- tick
		case err, ok := <-errs:
			if !ok {
				continue
			}
			if err != nil {
				return
			}
		}
	}
}

func (p *LiveProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	return nil
}
