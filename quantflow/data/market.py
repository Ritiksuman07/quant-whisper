from __future__ import annotations

from datetime import date, timedelta
import hashlib
import random

import requests

from quantflow.config import QuantFlowConfig


class MarketDataProvider:
    def __init__(self, config: QuantFlowConfig):
        self._config = config
        self._headers = {"User-Agent": config.user_agent}

    def get_daily_closes(self, ticker: str, lookback_days: int = 252) -> list[tuple[str, float]]:
        if self._config.offline_mode:
            return self._synthetic_series(ticker=ticker, lookback_days=lookback_days)
        try:
            symbol = f"{ticker.lower()}.us"
            url = f"https://stooq.com/q/d/l/?s={symbol}&i=d"
            response = requests.get(url, timeout=self._config.http_timeout_seconds, headers=self._headers)
            response.raise_for_status()
            lines = [line.strip() for line in response.text.splitlines() if line.strip()]
            if len(lines) <= 1:
                return self._synthetic_series(ticker=ticker, lookback_days=lookback_days)
            rows: list[tuple[str, float]] = []
            for line in lines[1:]:
                parts = line.split(",")
                if len(parts) < 5:
                    continue
                close_price = parts[4]
                if close_price in ("0", "N/D", ""):
                    continue
                rows.append((parts[0], float(close_price)))
            if len(rows) < 40:
                return self._synthetic_series(ticker=ticker, lookback_days=lookback_days)
            return rows[-lookback_days:]
        except requests.RequestException:
            return self._synthetic_series(ticker=ticker, lookback_days=lookback_days)

    @staticmethod
    def _synthetic_series(ticker: str, lookback_days: int) -> list[tuple[str, float]]:
        today = date.today()
        seed = int(hashlib.sha256(ticker.upper().encode("utf-8")).hexdigest()[:8], 16)
        generator = random.Random(seed)
        price = 100.0 + generator.uniform(-3.0, 3.0)
        rows: list[tuple[str, float]] = []
        for day_index in range(lookback_days):
            session_date = today - timedelta(days=lookback_days - day_index)
            drift = generator.uniform(-0.018, 0.02)
            price = max(4.0, price * (1.0 + drift))
            rows.append((session_date.isoformat(), round(price, 4)))
        return rows
