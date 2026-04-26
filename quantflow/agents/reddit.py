from __future__ import annotations

from collections import defaultdict
from dataclasses import asdict
from datetime import datetime, timezone
import re

import requests

from quantflow.config import QuantFlowConfig
from quantflow.models import SentimentSnapshot

POSITIVE_WORDS = {
    "beat",
    "bull",
    "bullish",
    "breakout",
    "buy",
    "moon",
    "rally",
    "squeeze",
    "upside",
}

NEGATIVE_WORDS = {
    "bear",
    "bearish",
    "dump",
    "miss",
    "recession",
    "rejected",
    "sell",
    "short",
    "volatility",
}

DEFAULT_SUBREDDITS = ("wallstreetbets", "stocks", "investing")
TICKER_PATTERN = re.compile(r"\$?[A-Z]{1,5}\b")
COMMON_NON_TICKERS = {"A", "I", "YOLO", "DD", "CEO", "USA", "IPO", "ETF", "ATH"}

OFFLINE_POST_FIXTURES: dict[str, list[str]] = {
    "wallstreetbets": [
        "XBI looks weak after another FDA rejected filing, staying short.",
        "Biotech sentiment is deteriorating and volatility keeps rising.",
        "Is SPY finally ready for another rally?",
    ],
    "stocks": [
        "Watching XBI post earnings miss narratives from smaller names.",
        "SPY macro read looks balanced this week.",
        "Any bullish signals for biotech ETFs?",
    ],
    "investing": [
        "Risk management matters if you short event-heavy sectors.",
        "XBI has high mention velocity relative to usual posts.",
    ],
}


class RedditSentimentAgent:
    def __init__(self, config: QuantFlowConfig):
        self._config = config
        self._headers = {
            "User-Agent": config.user_agent,
            "Accept": "application/json",
        }

    def collect(
        self,
        ticker: str,
        limit_per_subreddit: int = 60,
    ) -> tuple[list[SentimentSnapshot], dict[str, float | int | str]]:
        ticker_symbol = ticker.upper()
        snapshots: list[SentimentSnapshot] = []
        total_mentions = 0
        sentiment_weighted_sum = 0.0
        mention_totals = defaultdict(int)

        for subreddit in DEFAULT_SUBREDDITS:
            posts = self._fetch_posts(subreddit, limit_per_subreddit)
            mentions, avg_sentiment = self._score_posts(posts, ticker_symbol)
            mention_totals[subreddit] = mentions
            total_mentions += mentions
            sentiment_weighted_sum += avg_sentiment * max(mentions, 1)
            anomaly = self._anomaly_score(mentions, len(posts))
            snapshots.append(
                SentimentSnapshot(
                    ticker=ticker_symbol,
                    subreddit=subreddit,
                    mentions=mentions,
                    sentiment_score=round(avg_sentiment, 4),
                    anomaly_score=round(anomaly, 4),
                    captured_at=datetime.now(timezone.utc).isoformat(),
                )
            )

        if total_mentions == 0:
            overall_sentiment = 0.0
        else:
            overall_sentiment = sentiment_weighted_sum / total_mentions
        aggregate_anomaly = max((snapshot.anomaly_score for snapshot in snapshots), default=0.0)

        return snapshots, {
            "ticker": ticker_symbol,
            "mention_volume": total_mentions,
            "overall_sentiment": round(overall_sentiment, 4),
            "anomaly_score": round(aggregate_anomaly, 4),
            "by_subreddit": [asdict(snapshot) for snapshot in snapshots],
        }

    def _fetch_posts(self, subreddit: str, limit: int) -> list[str]:
        if self._config.offline_mode:
            return OFFLINE_POST_FIXTURES.get(subreddit, [])
        try:
            url = f"https://www.reddit.com/r/{subreddit}/new.json?limit={limit}"
            response = requests.get(url, headers=self._headers, timeout=self._config.http_timeout_seconds)
            response.raise_for_status()
            children = response.json().get("data", {}).get("children", [])
            posts: list[str] = []
            for child in children:
                data = child.get("data", {})
                title = data.get("title", "")
                selftext = data.get("selftext", "")
                posts.append(f"{title} {selftext}".strip())
            if not posts:
                return OFFLINE_POST_FIXTURES.get(subreddit, [])
            return posts
        except requests.RequestException:
            return OFFLINE_POST_FIXTURES.get(subreddit, [])

    def _score_posts(self, posts: list[str], target_ticker: str) -> tuple[int, float]:
        sentiments: list[float] = []
        mentions = 0
        for post in posts:
            post_upper = post.upper()
            tickers = self._extract_tickers(post_upper)
            if target_ticker in tickers:
                mentions += 1
                sentiments.append(self._sentiment_score(post))
        if not sentiments:
            return mentions, 0.0
        return mentions, sum(sentiments) / len(sentiments)

    @staticmethod
    def _extract_tickers(text: str) -> set[str]:
        matches = TICKER_PATTERN.findall(text)
        normalized: set[str] = set()
        for match in matches:
            symbol = match.lstrip("$").upper()
            if symbol in COMMON_NON_TICKERS:
                continue
            normalized.add(symbol)
        return normalized

    @staticmethod
    def _sentiment_score(text: str) -> float:
        words = re.findall(r"[A-Za-z']+", text.lower())
        if not words:
            return 0.0
        positive_hits = sum(1 for word in words if word in POSITIVE_WORDS)
        negative_hits = sum(1 for word in words if word in NEGATIVE_WORDS)
        return (positive_hits - negative_hits) / max(4, len(words))

    @staticmethod
    def _anomaly_score(mentions: int, post_count: int) -> float:
        baseline = max(2.0, post_count / 15.0)
        return min(5.0, mentions / baseline)
