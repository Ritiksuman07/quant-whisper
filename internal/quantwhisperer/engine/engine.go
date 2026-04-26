package engine

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/ritiksuman07/quantflow/internal/quantwhisperer"
	"github.com/ritiksuman07/quantflow/internal/quantwhisperer/broker"
	"github.com/ritiksuman07/quantflow/internal/quantwhisperer/config"
	"github.com/ritiksuman07/quantflow/internal/quantwhisperer/llm"
	"github.com/ritiksuman07/quantflow/internal/quantwhisperer/risk"
	"github.com/ritiksuman07/quantflow/internal/quantwhisperer/store"
)

type Session struct {
	cfg        config.Options
	broker     broker.Client
	marketData MarketDataProvider
	logger     *slog.Logger
	llm        *llm.Client
	guard      *risk.Guard
	store      *store.Store
	strategy   string

	cash       float64
	position   float64
	peakEquity float64
	history    []float64
}

func NewSession(cfg config.Options, brokerClient broker.Client, marketData MarketDataProvider, logger *slog.Logger) (*Session, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if brokerClient == nil {
		return nil, fmt.Errorf("broker client is required")
	}
	if marketData == nil {
		return nil, fmt.Errorf("market data provider is required")
	}
	if logger == nil {
		logger = fallbackLogger("engine")
	}
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}

	return &Session{
		cfg:        cfg,
		broker:     brokerClient,
		marketData: marketData,
		logger:     logger.With(slog.String("component", "engine")),
		llm:        llm.NewClient(cfg, logger.With(slog.String("component", "llm"))),
		guard:      risk.NewGuard(cfg),
		store:      db,
		strategy:   cfg.StrategyText(),
		cash:       cfg.StartingCapital,
		peakEquity: cfg.StartingCapital,
		history:    make([]float64, 0, 64),
	}, nil
}

func (s *Session) Close() error {
	if s.marketData != nil {
		_ = s.marketData.Close()
	}
	if s.store != nil {
		return s.store.Close()
	}
	return nil
}

func (s *Session) Run(ctx context.Context, events chan<- quantwhisperer.Event) error {
	tickCount := 0
	s.emit(events, "status", fmt.Sprintf("connecting broker adapter: %s", s.broker.Name()), nil, nil, nil, time.Now().UTC())
	s.logger.Info("broker_connecting",
		slog.String("symbol", s.cfg.Symbol),
		slog.Int("tick_count", tickCount),
	)
	if err := s.broker.Connect(ctx, s.cfg.Credentials(), s.cfg.Mode); err != nil {
		s.logger.Error("broker_connect_failed",
			slog.String("symbol", s.cfg.Symbol),
			slog.Int("tick_count", tickCount),
			slog.String("error", err.Error()),
		)
		return err
	}
	s.emit(events, "status", "broker connected", nil, nil, nil, time.Now().UTC())
	s.logger.Info("broker_connected",
		slog.String("symbol", s.cfg.Symbol),
		slog.Int("tick_count", tickCount),
	)

	tickCh, err := s.marketData.Subscribe(s.cfg.Symbol)
	if err != nil {
		s.logger.Error("market_data_subscribe_failed",
			slog.String("symbol", s.cfg.Symbol),
			slog.Int("tick_count", tickCount),
			slog.String("error", err.Error()),
		)
		return err
	}
	defer s.marketData.Close()

	go func() {
		<-ctx.Done()
		_ = s.marketData.Close()
	}()

	for tick := range tickCh {
		tickCount++
		s.trackPrice(tick.LastPrice)
		momentum := s.momentum()

		decision, source := s.llm.Decide(ctx, llm.Input{
			Tick:      tick,
			Momentum:  momentum,
			History:   append([]float64(nil), s.history...),
			Strategy:  s.strategy,
			Mode:      s.cfg.Mode,
			Broker:    s.broker.Name(),
			Symbol:    s.cfg.Symbol,
			Timestamp: tick.Timestamp,
			TickCount: tickCount,
		})

		if err := s.store.LogDecision(tick, momentum, decision, source); err != nil {
			s.logger.Error("decision_log_failed",
				slog.String("symbol", tick.Symbol),
				slog.Int("tick_count", tickCount),
				slog.Group("decision",
					slog.String("action", decision.Action),
					slog.Float64("confidence", decision.Confidence),
				),
				slog.String("error", err.Error()),
			)
			s.emit(events, "error", fmt.Sprintf("decision log error: %v", err), &tick, &decision, nil, tick.Timestamp)
		}
		s.logger.Info("decision_generated",
			slog.String("symbol", tick.Symbol),
			slog.Int("tick_count", tickCount),
			slog.Group("decision",
				slog.String("action", decision.Action),
				slog.Float64("confidence", decision.Confidence),
			),
		)

		snapshot := s.snapshot(tick)
		if err := s.guard.CheckSnapshot(snapshot); err != nil {
			s.logger.Warn("risk_wall_triggered",
				slog.String("symbol", tick.Symbol),
				slog.Int("tick_count", tickCount),
				slog.Group("decision",
					slog.String("action", decision.Action),
					slog.Float64("confidence", decision.Confidence),
				),
				slog.String("error", err.Error()),
			)
			s.emit(events, "risk", err.Error(), &tick, &decision, nil, tick.Timestamp)
			if err := s.store.LogSnapshot(s.cfg.Mode, snapshot); err != nil {
				s.emit(events, "error", fmt.Sprintf("snapshot log error: %v", err), &tick, &decision, nil, tick.Timestamp)
			}
			return err
		}

		s.emit(events, "decision", fmt.Sprintf("%s (%.2f via %s)", decision.Action, decision.Confidence, source), &tick, &decision, nil, tick.Timestamp)

		trade, shouldTrade, reason := s.evaluateTrade(decision, snapshot, tick.Timestamp)
		if shouldTrade {
			if s.cfg.Mode == quantwhisperer.ModeLive {
				if err := s.broker.PlaceOrder(ctx, trade); err != nil {
					s.logger.Error("broker_order_failed",
						slog.String("symbol", tick.Symbol),
						slog.Int("tick_count", tickCount),
						slog.Group("decision",
							slog.String("action", decision.Action),
							slog.Float64("confidence", decision.Confidence),
						),
						slog.String("error", err.Error()),
					)
					s.emit(events, "error", fmt.Sprintf("broker order failed: %v", err), &tick, &decision, nil, tick.Timestamp)
					continue
				}
			}
			s.applyTrade(trade)
			s.logger.Info("trade_executed",
				slog.String("symbol", tick.Symbol),
				slog.Int("tick_count", tickCount),
				slog.Group("decision",
					slog.String("action", decision.Action),
					slog.Float64("confidence", decision.Confidence),
				),
			)
			if err := s.store.LogTrade(trade); err != nil {
				s.logger.Error("trade_log_failed",
					slog.String("symbol", tick.Symbol),
					slog.Int("tick_count", tickCount),
					slog.Group("decision",
						slog.String("action", decision.Action),
						slog.Float64("confidence", decision.Confidence),
					),
					slog.String("error", err.Error()),
				)
				s.emit(events, "error", fmt.Sprintf("trade log error: %v", err), &tick, &decision, &trade, tick.Timestamp)
			}
			s.emit(events, "trade", fmt.Sprintf("%s %.0f @ %.2f", trade.Side, trade.Quantity, trade.Price), &tick, &decision, &trade, tick.Timestamp)
		} else if reason != "" {
			s.emit(events, "status", reason, &tick, &decision, nil, tick.Timestamp)
		}

		updated := s.snapshot(tick)
		if err := s.store.LogSnapshot(s.cfg.Mode, updated); err != nil {
			s.logger.Error("snapshot_log_failed",
				slog.String("symbol", tick.Symbol),
				slog.Int("tick_count", tickCount),
				slog.Group("decision",
					slog.String("action", decision.Action),
					slog.Float64("confidence", decision.Confidence),
				),
				slog.String("error", err.Error()),
			)
			s.emit(events, "error", fmt.Sprintf("snapshot log error: %v", err), &tick, &decision, nil, tick.Timestamp)
		}
		s.emit(events, "snapshot", fmt.Sprintf("equity %.2f | dd %.2f%%", updated.Equity, updated.DrawdownPct), &tick, &decision, nil, tick.Timestamp)
	}

	return nil
}

func (s *Session) evaluateTrade(decision quantwhisperer.Decision, snapshot quantwhisperer.Snapshot, now time.Time) (quantwhisperer.Trade, bool, string) {
	action := strings.ToUpper(strings.TrimSpace(decision.Action))
	if action != "BUY" && action != "SELL" {
		return quantwhisperer.Trade{}, false, "decision HOLD: no trade"
	}

	if s.cfg.Mode == quantwhisperer.ModeLive && decision.Confidence < s.cfg.ConfidenceThreshold {
		return quantwhisperer.Trade{}, false, fmt.Sprintf("confidence %.2f below wall %.2f", decision.Confidence, s.cfg.ConfidenceThreshold)
	}

	qty := math.Floor((snapshot.Equity * (s.cfg.PositionSizePct / 100.0)) / snapshot.LastPrice)
	if qty < 1 {
		return quantwhisperer.Trade{}, false, "position size too small for one unit"
	}

	if err := s.guard.AllowTrade(now, snapshot, qty); err != nil {
		return quantwhisperer.Trade{}, false, err.Error()
	}

	trade := quantwhisperer.Trade{
		Mode:       s.cfg.Mode,
		Broker:     s.broker.Name(),
		Symbol:     s.cfg.Symbol,
		Side:       action,
		Quantity:   qty,
		Price:      snapshot.LastPrice,
		Confidence: decision.Confidence,
		Reasoning:  decision.Reasoning,
		Timestamp:  now.UTC(),
	}
	return trade, true, ""
}

func (s *Session) applyTrade(trade quantwhisperer.Trade) {
	value := trade.Quantity * trade.Price
	if trade.Side == "BUY" {
		s.cash -= value
		s.position += trade.Quantity
		return
	}
	s.cash += value
	s.position -= trade.Quantity
}

func (s *Session) trackPrice(price float64) {
	s.history = append(s.history, price)
	if len(s.history) > 32 {
		s.history = s.history[len(s.history)-32:]
	}
}

func (s *Session) momentum() float64 {
	if len(s.history) < 2 {
		return 0
	}
	first := s.history[0]
	last := s.history[len(s.history)-1]
	if first == 0 {
		return 0
	}
	return (last - first) / first
}

func (s *Session) snapshot(tick quantwhisperer.Tick) quantwhisperer.Snapshot {
	equity := s.cash + (s.position * tick.LastPrice)
	if equity > s.peakEquity {
		s.peakEquity = equity
	}
	drawdownPct := 0.0
	if s.peakEquity > 0 {
		drawdownPct = ((s.peakEquity - equity) / s.peakEquity) * 100
	}
	return quantwhisperer.Snapshot{
		Symbol:      tick.Symbol,
		Timestamp:   tick.Timestamp,
		Cash:        s.cash,
		PositionQty: s.position,
		LastPrice:   tick.LastPrice,
		Equity:      equity,
		DrawdownPct: drawdownPct,
	}
}

func (s *Session) emit(events chan<- quantwhisperer.Event, eventType string, message string, tick *quantwhisperer.Tick, decision *quantwhisperer.Decision, trade *quantwhisperer.Trade, ts time.Time) {
	if events == nil {
		return
	}
	snapshot := (*quantwhisperer.Snapshot)(nil)
	if tick != nil {
		s := s.snapshot(*tick)
		snapshot = &s
	}
	events <- quantwhisperer.Event{
		Timestamp: ts,
		Type:      eventType,
		Message:   message,
		Tick:      tick,
		Decision:  decision,
		Trade:     trade,
		Snapshot:  snapshot,
	}
}
