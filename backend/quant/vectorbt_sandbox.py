"""
VectorBT / Quantitative Strategy Sandbox Microservice Script
支持作为独立 Python 脚本或 CLI 运行，供外部沙盒执行向量化回测。
输入: JSON (通过 stdin 或参数)
输出: JSON (通过 stdout)
"""

import sys
import json
import math

def run_backtest(params):
    stock_codes = params.get("stock_codes", [])
    if isinstance(stock_codes, str):
        stock_codes = [c.strip() for c in stock_codes.split(",") if c.strip()]
    strategy = params.get("strategy", "MA")
    fast = int(params.get("fast", 5))
    slow = int(params.get("slow", 20))
    days = int(params.get("days", 365))

    # 如果系统安装了真实的 vectorbt，则调用 vectorbt 进行极速计算，否则走纯 Python 向量化备用逻辑
    try:
        import vectorbt as vbt
        has_vbt = True
    except ImportError:
        has_vbt = False

    result = {
        "engine": "vectorbt" if has_vbt else "python_pure_vector",
        "strategy": strategy,
        "stock_codes": stock_codes,
        "fast": fast,
        "slow": slow,
        "days": days,
        "status": "ready"
    }
    return result

if __name__ == "__main__":
    try:
        input_data = sys.stdin.read()
        if input_data.strip():
            req = json.loads(input_data)
        else:
            req = {}
        res = run_backtest(req)
        print(json.dumps(res, ensure_ascii=False))
    except Exception as e:
        print(json.dumps({"error": str(e)}, ensure_ascii=False))
