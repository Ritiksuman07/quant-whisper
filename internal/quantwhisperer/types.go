package quantwhisperer

import "time"

type Mode string

const (
	ModePaper Mode = "paper"
	ModeLive  Mode = "live"
)

type Credentials struct {
	APIKey      string
	APISecret   string
	AccessToken string
}

type Tick struct {
	Broker    string
	Symbol    string
	Timestamp time.Time
	LastPrice float64
	Bid       float64
	Ask       float64
	Volume    int64
}

type Decision struct {
	Action     string  `json:"action"`
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning"`
	Raw        string  `json:"-"`
}

type Trade struct {
	Mode       Mode
	Broker     string
	Symbol     string
	Side       string
	Quantity   float64
	Price      float64
	Confidence float64
	Reasoning  string
	Timestamp  time.Time
}

type Snapshot struct {
	Symbol      string
	Timestamp   time.Time
	Cash        float64
	PositionQty float64
	LastPrice   float64
	Equity      float64
	DrawdownPct float64
}

type Event struct {
	Timestamp time.Time
	Type      string
	Message   string
	Tick      *Tick
	Decision  *Decision
	Trade     *Trade
	Snapshot  *Snapshot
}
