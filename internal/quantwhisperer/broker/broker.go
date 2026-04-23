package broker

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/ritiksuman07/quantflow/internal/quantwhisperer"
)

type Client interface {
	Name() string
	Connect(context.Context, quantwhisperer.Credentials, quantwhisperer.Mode) error
	StreamTicks(context.Context, string, time.Duration, int) (<-chan quantwhisperer.Tick, <-chan error)
	PlaceOrder(context.Context, quantwhisperer.Trade) error
}

type simulatedAdapter struct {
	name   string
	random *rand.Rand
	price  float64
}

type zerodhaRawTick struct {
	Tradable string
	LTP      float64
	Bid      float64
	Ask      float64
	Volume   int64
	Ts       int64
}

type dhanRawTick struct {
	Instrument string
	Last       float64
	BestBid    float64
	BestAsk    float64
	TotalQty   int64
	EpochMs    int64
}

type ibRawTick struct {
	Symbol    string
	Market    float64
	BidPrice  float64
	AskPrice  float64
	Volume    int64
	Timestamp int64
}

func NewClient(name string) (Client, error) {
	kind := strings.ToLower(strings.TrimSpace(name))
	switch kind {
	case "zerodha", "dhan", "ib", "interactivebrokers", "interactive-brokers":
		return &simulatedAdapter{
			name:   normalizeBrokerName(kind),
			random: rand.New(rand.NewSource(time.Now().UnixNano())),
			price:  100 + rand.Float64()*75,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported broker adapter: %s", name)
	}
}

func normalizeBrokerName(kind string) string {
	switch kind {
	case "interactivebrokers", "interactive-brokers", "ib":
		return "interactive-brokers"
	default:
		return kind
	}
}

func (s *simulatedAdapter) Name() string {
	return s.name
}

func (s *simulatedAdapter) Connect(_ context.Context, creds quantwhisperer.Credentials, mode quantwhisperer.Mode) error {
	if mode == quantwhisperer.ModeLive && (creds.APIKey == "" || creds.APISecret == "") {
		return fmt.Errorf("live mode requires %s API credentials in local environment", strings.ToUpper(strings.ReplaceAll(s.name, "-", "_")))
	}
	return nil
}

func (s *simulatedAdapter) StreamTicks(ctx context.Context, symbol string, interval time.Duration, maxTicks int) (<-chan quantwhisperer.Tick, <-chan error) {
	ticks := make(chan quantwhisperer.Tick, 8)
	errs := make(chan error, 1)

	go func() {
		defer close(ticks)
		defer close(errs)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for i := 0; i < maxTicks; i++ {
			select {
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			case <-ticker.C:
				tick := s.nextTick(symbol)
				ticks <- tick
			}
		}
	}()

	return ticks, errs
}

func (s *simulatedAdapter) nextTick(symbol string) quantwhisperer.Tick {
	jump := s.random.NormFloat64() * 0.35
	s.price = math.Max(1, s.price+jump)
	spread := math.Max(0.02, s.price*0.0008)
	volume := int64(100 + s.random.Intn(600))
	now := time.Now().UTC()

	switch s.name {
	case "zerodha":
		raw := zerodhaRawTick{
			Tradable: symbol,
			LTP:      s.price,
			Bid:      s.price - spread,
			Ask:      s.price + spread,
			Volume:   volume,
			Ts:       now.UnixMilli(),
		}
		return normalizeZerodha(raw, s.name)
	case "dhan":
		raw := dhanRawTick{
			Instrument: symbol,
			Last:       s.price,
			BestBid:    s.price - spread,
			BestAsk:    s.price + spread,
			TotalQty:   volume,
			EpochMs:    now.UnixMilli(),
		}
		return normalizeDhan(raw, s.name)
	default:
		raw := ibRawTick{
			Symbol:    symbol,
			Market:    s.price,
			BidPrice:  s.price - spread,
			AskPrice:  s.price + spread,
			Volume:    volume,
			Timestamp: now.UnixMilli(),
		}
		return normalizeIB(raw, s.name)
	}
}

func normalizeZerodha(raw zerodhaRawTick, broker string) quantwhisperer.Tick {
	return quantwhisperer.Tick{
		Broker:    broker,
		Symbol:    raw.Tradable,
		Timestamp: time.UnixMilli(raw.Ts).UTC(),
		LastPrice: raw.LTP,
		Bid:       raw.Bid,
		Ask:       raw.Ask,
		Volume:    raw.Volume,
	}
}

func normalizeDhan(raw dhanRawTick, broker string) quantwhisperer.Tick {
	return quantwhisperer.Tick{
		Broker:    broker,
		Symbol:    raw.Instrument,
		Timestamp: time.UnixMilli(raw.EpochMs).UTC(),
		LastPrice: raw.Last,
		Bid:       raw.BestBid,
		Ask:       raw.BestAsk,
		Volume:    raw.TotalQty,
	}
}

func normalizeIB(raw ibRawTick, broker string) quantwhisperer.Tick {
	return quantwhisperer.Tick{
		Broker:    broker,
		Symbol:    raw.Symbol,
		Timestamp: time.UnixMilli(raw.Timestamp).UTC(),
		LastPrice: raw.Market,
		Bid:       raw.BidPrice,
		Ask:       raw.AskPrice,
		Volume:    raw.Volume,
	}
}

func (s *simulatedAdapter) PlaceOrder(_ context.Context, _ quantwhisperer.Trade) error {
	return nil
}
