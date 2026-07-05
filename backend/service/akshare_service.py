#!/usr/bin/env python3
"""
AKShare HTTP 服务 — 为 blades-stock 提供 AKShare 数据接口。

部署在 VPS（115.190.72.22），blades-stock 通过 HTTP 调用。

启动：
  cd /root/akshare-app
  source venv/bin/activate
  uvicorn akshare_service:app --host 0.0.0.0 --port 8089 --workers 2

Supervisor 托管（推荐）：
  [program:akshare]
  command=/root/akshare-app/venv/bin/uvicorn akshare_service:app --host 0.0.0.0 --port 8089 --workers 2
  directory=/root/akshare-app
  user=root
  autostart=true
  autorestart=true
  stderr_logfile=/var/log/akshare.err.log
  stdout_logfile=/var/log/akshare.out.log
"""

import asyncio
import time
import traceback
from typing import Any, Optional

import akshare as ak
import pandas as pd
from fastapi import FastAPI, HTTPException, Query
from fastapi.responses import JSONResponse

app = FastAPI(title="AKShare Service", version="1.0.0")

# ──────────────────────────────────────────────
# Helper: convert DataFrame to JSON-serializable
# ──────────────────────────────────────────────

def _df_to_json(df: pd.DataFrame) -> list[dict[str, Any]]:
    """Convert DataFrame → list[dict], NaN → None, Timestamp → str."""
    if df is None or df.empty:
        return []
    df = df.replace({pd.NA: None})
    df = df.where(pd.notna(df), None)
    # Convert Timestamp columns to ISO strings
    for col in df.select_dtypes(include=["datetime64[ns, UTC]", "datetime64[ns]", "datetime64"]).columns:
        df[col] = df[col].astype(str)
    return df.to_dict(orient="records")


def _ok(data: Any, meta: Optional[dict] = None) -> JSONResponse:
    body: dict[str, Any] = {"code": 0, "data": data, "msg": "ok"}
    if meta:
        body["meta"] = meta
    return JSONResponse(body)


def _err(msg: str) -> JSONResponse:
    return JSONResponse({"code": -1, "data": None, "msg": msg})


# ──────────────────────────────────────────────
# Health
# ──────────────────────────────────────────────

@app.get("/health")
async def health():
    return {"status": "ok", "service": "akshare", "version": ak.__version__}


# ──────────────────────────────────────────────
# 1. 商品期货 — 实时行情
# ──────────────────────────────────────────────

@app.get("/api/v1/futures/spot")
async def futures_spot(market: str = Query("中金所", description="中金所/上期所/大商所/郑商所/广期所/上海国际能源交易中心")):
    """商品期货实时行情"""
    try:
        df = ak.futures_zh_spot_sina(market=market)
        return _ok(_df_to_json(df), {"market": market, "count": len(df)})
    except Exception as e:
        return _err(f"futures_spot error: {e}")


@app.get("/api/v1/futures/main_ship")
async def futures_main_ship():
    """期货主力合约列表"""
    try:
        df = ak.match_main_contract()
        return _ok(_df_to_json(df))
    except Exception as e:
        return _err(f"futures_main_ship error: {e}")


# ──────────────────────────────────────────────
# 2. ETF — 实时行情 / 折溢价
# ──────────────────────────────────────────────

@app.get("/api/v1/etf/spot")
async def etf_spot():
    """ETF 实时行情（含最新价、涨跌幅、折溢价率）"""
    try:
        df = ak.fund_etf_spot_em()
        return _ok(
            _df_to_json(df),
            {"columns": list(df.columns)},
        )
    except Exception as e:
        return _err(f"etf_spot error: {e}")


@app.get("/api/v1/etf/historical")
async def etf_historical(
    symbol: str = Query(..., description="ETF 代码，如 510050"),
    period: str = Query("daily", description="daily/weekly/monthly"),
    start_date: str = Query("", description="起始日期 YYYYMMDD"),
    end_date: str = Query("", description="结束日期 YYYYMMDD"),
    adjust: str = Query("qfq", description="复权 qfq/hfq"),
):
    """ETF 历史行情"""
    try:
        df = ak.fund_etf_hist_em(
            symbol=symbol,
            period=period,
            start_date=start_date,
            end_date=end_date,
            adjust=adjust,
        )
        return _ok(_df_to_json(df))
    except Exception as e:
        return _err(f"etf_historical error: {e}")


# ──────────────────────────────────────────────
# 3. 开放式基金 — 净值
# ──────────────────────────────────────────────

@app.get("/api/v1/fund/open/net_value")
async def open_fund_net_value(
    symbol: str = Query(..., description="基金代码"),
    start_date: str = Query("", description="起始日期"),
    end_date: str = Query("", description="结束日期"),
):
    """开放式基金历史净值"""
    try:
        df = ak.fund_open_fund_info_em(
            symbol=symbol,
            indicator="单位净值走势",
        )
        if start_date:
            df = df[df["净值日期"] >= start_date]
        if end_date:
            df = df[df["净值日期"] <= end_date]
        return _ok(_df_to_json(df))
    except Exception as e:
        return _err(f"open_fund_net_value error: {e}")


@app.get("/api/v1/fund/open/rank")
async def open_fund_rank():
    """开放式基金排行榜"""
    try:
        df = ak.fund_open_fund_rank_em()
        return _ok(
            _df_to_json(df),
            {"columns": list(df.columns)},
        )
    except Exception as e:
        return _err(f"open_fund_rank error: {e}")


# ──────────────────────────────────────────────
# 4. 板块 / 概念
# ──────────────────────────────────────────────

@app.get("/api/v1/board/concept")
async def board_concept():
    """概念板块列表"""
    try:
        df = ak.stock_board_concept_name_em()
        return _ok(
            _df_to_json(df),
            {"columns": list(df.columns)},
        )
    except Exception as e:
        return _err(f"board_concept error: {e}")


@app.get("/api/v1/board/industry")
async def board_industry():
    """行业板块列表"""
    try:
        df = ak.stock_board_industry_name_em()
        return _ok(
            _df_to_json(df),
            {"columns": list(df.columns)},
        )
    except Exception as e:
        return _err(f"board_industry error: {e}")


# ──────────────────────────────────────────────
# 5. 宏观经济
# ──────────────────────────────────────────────

@app.get("/api/v1/macro/cpi")
async def macro_cpi():
    """消费者物价指数 CPI（年度同比）"""
    try:
        df = ak.macro_china_cpi_yearly()
        return _ok(_df_to_json(df))
    except Exception as e:
        return _err(f"macro_cpi error: {e}")


@app.get("/api/v1/macro/gdp")
async def macro_gdp():
    """GDP 数据"""
    try:
        df = ak.macro_china_gdp()
        return _ok(_df_to_json(df))
    except Exception as e:
        return _err(f"macro_gdp error: {e}")


@app.get("/api/v1/macro/pmi")
async def macro_pmi():
    """采购经理人指数 PMI"""
    try:
        df = ak.macro_china_pmi()
        return _ok(_df_to_json(df))
    except Exception as e:
        return _err(f"macro_pmi error: {e}")


@app.get("/api/v1/macro/money_supply")
async def macro_money_supply():
    """货币供应量 M2/M1/M0"""
    try:
        df = ak.macro_china_money_supply()
        return _ok(_df_to_json(df))
    except Exception as e:
        return _err(f"macro_money_supply error: {e}")


# ──────────────────────────────────────────────
# 6. 龙虎榜
# ──────────────────────────────────────────────

@app.get("/api/v1/longhu/today")
async def longhu_today():
    """当日龙虎榜"""
    try:
        df = ak.stock_lhb_yyb_most()
        return _ok(_df_to_json(df))
    except Exception as e:
        return _err(f"longhu_today error: {e}")


# ──────────────────────────────────────────────
# 7. 可转债
# ──────────────────────────────────────────────

@app.get("/api/v1/convertible/bond_spot")
async def convertible_bond_spot():
    """可转债实时行情"""
    try:
        df = ak.bond_zh_cov()
        return _ok(
            _df_to_json(df),
            {"columns": list(df.columns)},
        )
    except Exception as e:
        return _err(f"convertible_bond_spot error: {e}")


# ──────────────────────────────────────────────
# 8. 泛函调用 — 通用代理（高风险，按需开放）
# ──────────────────────────────────────────────

@app.post("/api/v1/call")
async def generic_call(
    function: str = Query(..., description="AKShare 函数名，如 stock_board_concept_name_em"),
    kwargs: Optional[dict[str, Any]] = None,
):
    """通用 AKShare 函数调用（自由模式，参数以 JSON body 传入）"""
    if kwargs is None:
        kwargs = {}
    try:
        fn = getattr(ak, function, None)
        if fn is None:
            return _err(f"AKShare 函数 '{function}' 不存在")
        result = fn(**kwargs)
        if isinstance(result, pd.DataFrame):
            return _ok(_df_to_json(result), {"columns": list(result.columns)})
        elif isinstance(result, (list, dict, str, int, float, bool)):
            return _ok(result)
        return _ok(str(result))
    except Exception as e:
        return _err(f"generic_call error: {e}")


# ──────────────────────────────────────────────
# Startup info
# ──────────────────────────────────────────────

@app.on_event("startup")
async def startup():
    print(f"[AKShare Service] Started — AKShare v{ak.__version__}")
    print(f"[AKShare Service] Endpoints: /api/v1/futures/spot, /api/v1/etf/spot, ...")


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8089)
