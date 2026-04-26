from __future__ import annotations

from dataclasses import asdict
from typing import Any

import requests

from quantflow.config import QuantFlowConfig
from quantflow.models import FilingRecord

SUPPORTED_FORMS = {"10-K", "10-Q", "8-K"}

OFFLINE_FILING_FIXTURES: dict[str, list[dict[str, str]]] = {
    "XBI": [
        {
            "form": "8-K",
            "filingDate": "2026-03-11",
            "accessionNumber": "0000000000-26-000001",
            "primaryDocument": "xbi_8k_fda_update.htm",
        },
        {
            "form": "10-Q",
            "filingDate": "2025-11-02",
            "accessionNumber": "0000000000-25-000019",
            "primaryDocument": "xbi_10q_q3.htm",
        },
        {
            "form": "10-K",
            "filingDate": "2025-02-12",
            "accessionNumber": "0000000000-25-000002",
            "primaryDocument": "xbi_10k.htm",
        },
    ],
    "SPY": [
        {
            "form": "10-Q",
            "filingDate": "2026-02-01",
            "accessionNumber": "0000000000-26-000101",
            "primaryDocument": "spy_10q.htm",
        },
        {
            "form": "8-K",
            "filingDate": "2026-01-18",
            "accessionNumber": "0000000000-26-000082",
            "primaryDocument": "spy_8k.htm",
        },
    ],
}


class SecFilingScraperAgent:
    def __init__(self, config: QuantFlowConfig):
        self._config = config
        self._headers = {"User-Agent": config.user_agent, "Accept-Encoding": "gzip, deflate"}

    def collect(
        self,
        ticker: str,
        limit: int = 12,
    ) -> tuple[list[FilingRecord], dict[str, float | int | str]]:
        if self._config.offline_mode:
            records = self._offline_records(ticker, limit)
            return records, self._summarize(records)

        try:
            cik = self._resolve_cik(ticker)
            if not cik:
                records = self._offline_records(ticker, limit)
                return records, self._summarize(records)
            url = f"https://data.sec.gov/submissions/CIK{cik}.json"
            response = requests.get(url, headers=self._headers, timeout=self._config.http_timeout_seconds)
            response.raise_for_status()
            payload = response.json()
            recent = payload.get("filings", {}).get("recent", {})
            forms = recent.get("form", [])
            filing_dates = recent.get("filingDate", [])
            accession_numbers = recent.get("accessionNumber", [])
            primary_documents = recent.get("primaryDocument", [])
            records: list[FilingRecord] = []
            for index, form in enumerate(forms):
                if form not in SUPPORTED_FORMS:
                    continue
                records.append(
                    FilingRecord(
                        ticker=ticker,
                        cik=cik,
                        form=form,
                        filed_at=self._safe_value(filing_dates, index),
                        accession_no=self._safe_value(accession_numbers, index),
                        primary_doc=self._safe_value(primary_documents, index),
                        risk_flag=self._risk_flag(form),
                        extracted_signal=self._extracted_signal(form),
                    )
                )
                if len(records) >= limit:
                    break
            if not records:
                records = self._offline_records(ticker, limit)
            return records, self._summarize(records)
        except requests.RequestException:
            records = self._offline_records(ticker, limit)
            return records, self._summarize(records)

    def _resolve_cik(self, ticker: str) -> str | None:
        url = "https://www.sec.gov/files/company_tickers.json"
        response = requests.get(url, headers=self._headers, timeout=self._config.http_timeout_seconds)
        response.raise_for_status()
        payload: dict[str, Any] = response.json()
        normalized = ticker.upper()
        for item in payload.values():
            if item.get("ticker", "").upper() == normalized:
                cik_value = int(item["cik_str"])
                return f"{cik_value:010d}"
        return None

    def _offline_records(self, ticker: str, limit: int) -> list[FilingRecord]:
        fixtures = OFFLINE_FILING_FIXTURES.get(ticker.upper(), OFFLINE_FILING_FIXTURES["SPY"])
        records: list[FilingRecord] = []
        for item in fixtures[:limit]:
            form = item["form"]
            records.append(
                FilingRecord(
                    ticker=ticker.upper(),
                    cik="0000000000",
                    form=form,
                    filed_at=item["filingDate"],
                    accession_no=item["accessionNumber"],
                    primary_doc=item["primaryDocument"],
                    risk_flag=self._risk_flag(form),
                    extracted_signal=self._extracted_signal(form),
                )
            )
        return records

    @staticmethod
    def _safe_value(values: list[str], index: int) -> str:
        if index < len(values):
            return values[index]
        return ""

    @staticmethod
    def _risk_flag(form: str) -> str:
        if form == "8-K":
            return "event-driven-volatility"
        if form == "10-Q":
            return "quarterly-earnings-shift"
        return "annual-fundamental-reset"

    @staticmethod
    def _extracted_signal(form: str) -> str:
        if form == "8-K":
            return "heightened corporate event frequency"
        if form == "10-Q":
            return "earnings trajectory update"
        return "long-horizon risk posture change"

    def _summarize(self, records: list[FilingRecord]) -> dict[str, float | int | str]:
        if not records:
            return {
                "filing_count": 0,
                "recent_8k_count": 0,
                "earnings_delta_score": 0.0,
                "risk_score": 0.0,
                "latest_filing_date": "",
            }
        recent_8k = sum(1 for record in records if record.form == "8-K")
        periodic_forms = sum(1 for record in records if record.form in {"10-K", "10-Q"})
        risk_score = min(1.0, (recent_8k * 0.22) + (periodic_forms * 0.08))
        earnings_delta_score = min(1.0, periodic_forms / max(1, len(records)))
        latest_filing_date = max(record.filed_at for record in records)
        return {
            "filing_count": len(records),
            "recent_8k_count": recent_8k,
            "earnings_delta_score": round(earnings_delta_score, 4),
            "risk_score": round(risk_score, 4),
            "latest_filing_date": latest_filing_date,
            "filings": [asdict(record) for record in records],
        }
