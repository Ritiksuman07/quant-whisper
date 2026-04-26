from __future__ import annotations

from pathlib import Path


def render_equity_curve_svg(equity_curve: list[dict[str, float | str]], output_path: Path) -> None:
    width = 1024
    height = 512
    padding = 48
    values = [float(point["equity"]) for point in equity_curve]
    min_value = min(values)
    max_value = max(values)
    span = max(1e-9, max_value - min_value)
    x_span = max(1, len(values) - 1)

    points: list[str] = []
    for index, value in enumerate(values):
        x = padding + ((width - (2 * padding)) * (index / x_span))
        y = padding + ((height - (2 * padding)) * (1 - ((value - min_value) / span)))
        points.append(f"{x:.2f},{y:.2f}")

    polyline = " ".join(points)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(
        f"""<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}" viewBox="0 0 {width} {height}">
  <rect width="100%" height="100%" fill="#0b1020"/>
  <text x="{padding}" y="28" fill="#e2e8f0" font-size="20" font-family="Inter, Arial">QuantFlow Equity Curve</text>
  <line x1="{padding}" y1="{height-padding}" x2="{width-padding}" y2="{height-padding}" stroke="#334155" />
  <line x1="{padding}" y1="{padding}" x2="{padding}" y2="{height-padding}" stroke="#334155" />
  <polyline points="{polyline}" fill="none" stroke="#22d3ee" stroke-width="3" />
  <text x="{padding}" y="{height-padding+26}" fill="#94a3b8" font-size="14" font-family="Inter, Arial">Start</text>
  <text x="{width-padding-36}" y="{height-padding+26}" fill="#94a3b8" font-size="14" font-family="Inter, Arial">End</text>
  <text x="{padding}" y="{padding+16}" fill="#94a3b8" font-size="14" font-family="Inter, Arial">Max: {max_value:.2f}</text>
  <text x="{padding}" y="{padding+36}" fill="#94a3b8" font-size="14" font-family="Inter, Arial">Min: {min_value:.2f}</text>
</svg>
""",
        encoding="utf-8",
    )
