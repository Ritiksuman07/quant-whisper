package risk

import (
	"fmt"
	"sync"
	"time"

	"github.com/ritiksuman07/quantflow/internal/quantwhisperer"
	"github.com/ritiksuman07/quantflow/internal/quantwhisperer/config"
)

type Guard struct {
	cfg         config.Options
	mu          sync.Mutex
	tradeWindow []time.Time
	halted      bool
	reason      string
}

func NewGuard(cfg config.Options) *Guard {
	return &Guard{cfg: cfg, tradeWindow: make([]time.Time, 0, cfg.MaxTradesPerMinute)}
}

func (g *Guard) CheckSnapshot(snapshot quantwhisperer.Snapshot) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.halted {
		return fmt.Errorf("trading halted: %s", g.reason)
	}
	if snapshot.DrawdownPct >= g.cfg.MaxDrawdownPct {
		g.halted = true
		g.reason = fmt.Sprintf("max daily drawdown exceeded (%.2f%% >= %.2f%%)", snapshot.DrawdownPct, g.cfg.MaxDrawdownPct)
		return fmt.Errorf("trading halted: %s", g.reason)
	}
	return nil
}

func (g *Guard) AllowTrade(now time.Time, snapshot quantwhisperer.Snapshot, proposedQty float64) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.halted {
		return fmt.Errorf("trading halted: %s", g.reason)
	}
	if snapshot.DrawdownPct >= g.cfg.MaxDrawdownPct {
		g.halted = true
		g.reason = fmt.Sprintf("max daily drawdown exceeded (%.2f%% >= %.2f%%)", snapshot.DrawdownPct, g.cfg.MaxDrawdownPct)
		return fmt.Errorf("trading halted: %s", g.reason)
	}

	oneMinuteAgo := now.Add(-1 * time.Minute)
	kept := g.tradeWindow[:0]
	for _, ts := range g.tradeWindow {
		if ts.After(oneMinuteAgo) {
			kept = append(kept, ts)
		}
	}
	g.tradeWindow = kept
	if len(g.tradeWindow) >= g.cfg.MaxTradesPerMinute {
		g.halted = true
		g.reason = fmt.Sprintf("rate limiter tripped (%d trades in < 1 minute)", len(g.tradeWindow))
		return fmt.Errorf("trading halted: %s", g.reason)
	}

	notional := proposedQty * snapshot.LastPrice
	limit := snapshot.Equity * (g.cfg.PositionSizePct / 100.0)
	if notional > limit {
		g.halted = true
		g.reason = fmt.Sprintf("position sizing exceeded (%.2f > %.2f)", notional, limit)
		return fmt.Errorf("trading halted: %s", g.reason)
	}

	g.tradeWindow = append(g.tradeWindow, now)
	return nil
}

func (g *Guard) HaltedReason() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.reason
}
