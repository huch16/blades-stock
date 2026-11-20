#!/usr/bin/env python3
import datetime
import json
import math
import time
import traceback
from typing import Any, Optional

import akshare as ak
import pandas as pd
from fastapi import FastAPI, HTTPException, Query
from fastapi.responses import JSONResponse

app = FastAPI(title="AKShare Service", version="1.0.0")


def _df_to_json(df: pd.DataFrame) -> list[dict[str, Any]]:
    if df is None or df.empty:
        return []
    df = df.replace({pd.NA: None})
    df = df.where(pd.notna(df), None)
    for col in df.select_dtypes(include=["datetime64"]).columns:
        df[col] = df[col].astype(str)
    result = df.to_dict(orient="records")
    for row in result:
        for k, v in row.items():
            if isinstance(v, (datetime.date, datetime.datetime)):
                row[k] = v.isoformat()
            elif isinstance(v, float) and (math.isnan(v) or math.isinf(v)):
                row[k] = None
    return result


def _ok(data: Any, meta: Optional[dict] = None) -> JSONResponse:
    body: dict[str, Any] = {"code": 0, "data": data, "msg": "ok"}
    if meta:
        body["meta"] = meta
    return JSONResponse(content=body)


def _err(msg: str) -> JSONResponse:
    return JSONResponse({"code": -1, "data": None, "msg": msg})


@app.get("/health")
async def health():
    return {"status": "ok", "service": "akshare", "version": ak.__version__}


@app.get("/api/v1/futures/spot")
async def futures_spot(market: str = Query("中金所")):
    try:
        df = ak.futures_spot_price()
        return _ok(_df_to_json(df), {"market": market, "count": len(df)})
    except Exception as e:
        return _err(f"futures_spot error: {e}")


@app.get("/api/v1/futures/main_ship")
async def futures_main_ship():
    try:
        df = ak.match_main_contract()
        return _ok(_df_to_json(df))
    except Exception as e:
        return _err(f"futures_main_ship error: {e}")


@app.get("/api/v1/etf/spot")
async def etf_spot():
    try:
        df = ak.fund_etf_spot_em()
        return _ok(_df_to_json(df), {"columns": list(df.columns)})
    except Exception as e:
        return _err(f"etf_spot error: {e}")


@app.get("/api/v1/etf/historical")
async def etf_historical(
    symbol: str = Query(...),
    period: str = Query("daily"),
    start_date: str = Query(""),
    end_date: str = Query(""),
    adjust: str = Query("qfq"),
):
    try:
        df = ak.fund_etf_hist_em(symbol=symbol, period=period, start_date=start_date, end_date=end_date, adjust=adjust)
        return _ok(_df_to_json(df))
    except Exception as e:
        return _err(f"etf_historical error: {e}")


@app.get("/api/v1/fund/open/net_value")
async def open_fund_net_value(symbol: str = Query(...), start_date: str = Query(""), end_date: str = Query("")):
    try:
        df = ak.fund_open_fund_info_em(symbol=symbol, indicator="单位净值走势")
        if start_date:
            df = df[df["净值日期"] >= start_date]
        if end_date:
            df = df[df["净值日期"] <= end_date]
        return _ok(_df_to_json(df))
    except Exception as e:
        return _err(f"open_fund_net_value error: {e}")


@app.get("/api/v1/fund/open/rank")
async def open_fund_rank():
    try:
        df = ak.fund_open_fund_rank_em()
        return _ok(_df_to_json(df), {"columns": list(df.columns)})
    except Exception as e:
        return _err(f"open_fund_rank error: {e}")


@app.get("/api/v1/board/concept")
async def board_concept():
    try:
        df = ak.stock_board_concept_name_em()
        return _ok(_df_to_json(df), {"columns": list(df.columns)})
    except Exception as e:
        return _err(f"board_concept error: {e}")


@app.get("/api/v1/board/industry")
async def board_industry():
    try:
        df = ak.stock_board_industry_name_em()
        return _ok(_df_to_json(df), {"columns": list(df.columns)})
    except Exception as e:
        return _err(f"board_industry error: {e}")


@app.get("/api/v1/macro/cpi")
async def macro_cpi():
    try:
        df = ak.macro_china_cpi_yearly()
        return _ok(_df_to_json(df))
    except Exception as e:
        return _err(f"macro_cpi error: {e}")


@app.get("/api/v1/macro/gdp")
async def macro_gdp():
    try:
        df = ak.macro_china_gdp()
        return _ok(_df_to_json(df))
    except Exception as e:
        return _err(f"macro_gdp error: {e}")


@app.get("/api/v1/macro/pmi")
async def macro_pmi():
    try:
        df = ak.macro_china_pmi()
        return _ok(_df_to_json(df))
    except Exception as e:
        return _err(f"macro_pmi error: {e}")


@app.get("/api/v1/macro/money_supply")
async def macro_money_supply():
    try:
        df = ak.macro_china_money_supply()
        return _ok(_df_to_json(df))
    except Exception as e:
        return _err(f"macro_money_supply error: {e}")


@app.get("/api/v1/longhu/today")
async def longhu_today():
    try:
        df = ak.stock_lhb_yybph_em()
        return _ok(_df_to_json(df))
    except Exception as e:
        return _err(f"longhu_today error: {e}")


@app.get("/api/v1/convertible/bond_spot")
async def convertible_bond_spot():
    try:
        df = ak.bond_zh_cov()
        return _ok(_df_to_json(df), {"columns": list(df.columns)})
    except Exception as e:
        return _err(f"convertible_bond_spot error: {e}")


@app.post("/api/v1/call")
async def generic_call(function: str = Query(...), kwargs: Optional[dict[str, Any]] = None):
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


@app.on_event("startup")
async def startup():
    print(f"[AKShare Service] Started — AKShare v{ak.__version__}")


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8089)
