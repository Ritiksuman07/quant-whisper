use serde::{Deserialize, Serialize};
use std::io::{self, Read};

#[derive(Debug, Deserialize)]
struct PricePoint {
    date: String,
    close: f64,
}

#[derive(Debug, Deserialize)]
struct BacktestInput {
    side: String,
    prices: Vec<PricePoint>,
}

#[derive(Debug, Serialize)]
struct EquityPoint {
    date: String,
    equity: f64,
}

#[derive(Debug, Serialize)]
struct BacktestOutput {
    sharpe: f64,
    max_drawdown: f64,
    calmar: f64,
    cagr: f64,
    total_return: f64,
    equity_curve: Vec<EquityPoint>,
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let mut input_buf = String::new();
    io::stdin().read_to_string(&mut input_buf)?;
    let input: BacktestInput = serde_json::from_str(&input_buf)?;

    if input.prices.len() < 3 {
        return Err("Need at least 3 prices".into());
    }

    let direction = if input.side.to_lowercase() == "short" { -1.0 } else { 1.0 };
    let mut equity = 100_000.0_f64;
    let mut peak = equity;
    let mut max_drawdown = 0.0_f64;
    let mut returns: Vec<f64> = Vec::new();
    let mut equity_curve: Vec<EquityPoint> = Vec::new();
    equity_curve.push(EquityPoint {
        date: input.prices[0].date.clone(),
        equity,
    });

    for index in 1..input.prices.len() {
        let prev = input.prices[index - 1].close;
        let curr = input.prices[index].close;
        let daily = direction * ((curr / prev) - 1.0);
        returns.push(daily);
        equity *= 1.0 + daily;
        peak = peak.max(equity);
        let drawdown = (equity / peak) - 1.0;
        if drawdown < max_drawdown {
            max_drawdown = drawdown;
        }
        equity_curve.push(EquityPoint {
            date: input.prices[index].date.clone(),
            equity: (equity * 10_000.0).round() / 10_000.0,
        });
    }

    let mean = returns.iter().sum::<f64>() / returns.len() as f64;
    let variance = if returns.len() > 1 {
        returns
            .iter()
            .map(|value| (value - mean).powi(2))
            .sum::<f64>()
            / (returns.len() as f64 - 1.0)
    } else {
        0.0
    };
    let std_dev = variance.sqrt();
    let sharpe = if std_dev > 0.0 {
        (mean / std_dev) * 252_f64.sqrt()
    } else {
        0.0
    };

    let total_return = (equity / 100_000.0) - 1.0;
    let years = returns.len() as f64 / 252.0;
    let cagr = (1.0 + total_return).powf(1.0 / years.max(1.0 / 252.0)) - 1.0;
    let calmar = if max_drawdown < 0.0 {
        cagr / max_drawdown.abs()
    } else {
        0.0
    };

    let output = BacktestOutput {
        sharpe: (sharpe * 10_000.0).round() / 10_000.0,
        max_drawdown: (max_drawdown * 10_000.0).round() / 10_000.0,
        calmar: (calmar * 10_000.0).round() / 10_000.0,
        cagr: (cagr * 10_000.0).round() / 10_000.0,
        total_return: (total_return * 10_000.0).round() / 10_000.0,
        equity_curve,
    };

    println!("{}", serde_json::to_string(&output)?);
    Ok(())
}
