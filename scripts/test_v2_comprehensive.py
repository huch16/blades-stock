import urllib.request
import urllib.error
import json
import time
import sys

# Windows 控制台输出 UTF-8 编码适配 (避免 🟢 🟡 🔴 等 Emoji 触发 GBK 编码异常)
if hasattr(sys.stdout, 'reconfigure'):
    sys.stdout.reconfigure(encoding='utf-8')
if hasattr(sys.stderr, 'reconfigure'):
    sys.stderr.reconfigure(encoding='utf-8')

BASE_URL = sys.argv[1] if len(sys.argv) > 1 else 'https://8000-agqxrg26lk2za3jpqp2izbt7za.instabox.tech'
TOKEN = '2266762'

def make_req(path, data=None, headers=None, method='POST'):
    url = f'{BASE_URL}{path}'
    req_headers = {
        'User-Agent': 'BladesStockTest/2.0.0'
    }
    if headers:
        req_headers.update(headers)
    if data is not None:
        if isinstance(data, (dict, list)):
            data_bytes = json.dumps(data).encode('utf-8')
            req_headers['Content-Type'] = 'application/json'
        elif isinstance(data, str):
            data_bytes = data.encode('utf-8')
        else:
            data_bytes = data
    else:
        data_bytes = None
    req = urllib.request.Request(url, data=data_bytes, headers=req_headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            resp_headers = resp.headers
            raw_bytes = resp.read()
            if resp_headers.get('Content-Encoding') == 'gzip':
                import gzip
                raw_bytes = gzip.decompress(raw_bytes)
            body = raw_bytes.decode('utf-8')
            return resp.status, resp_headers, body
    except urllib.error.HTTPError as e:
        body = e.read().decode('utf-8')
        return e.code, e.headers, body
    except Exception as e:
        return 500, {}, str(e)

print(f'=== Testing Blades Stock v2.0.0 on {BASE_URL} ===\n')
passed = 0
failed = 0

def check(name, condition, details=''):
    global passed, failed
    if condition:
        print(f'  [PASS] {name}')
        passed += 1
    else:
        print(f'  [FAIL] {name} - {details}')
        failed += 1

# 1. Health & Cloud-Native Probes (/live, /ready, /health)
auth_headers = {'Authorization': f'Bearer {TOKEN}'}

# 1.1 Liveness probe (/live)
status, headers, body = make_req('/live', method='GET')
check('Liveness Probe /live returns 200 OK', status == 200, f'Status {status}')
check('Liveness Probe returns status: ok', '"status":"ok"' in body.replace(' ', ''), f'Got: {body}')

# 1.2 Readiness probe (/ready)
status, headers, body = make_req('/ready', method='GET')
check('Readiness Probe /ready returns 200 OK', status == 200, f'Status {status}')
check('Readiness Probe returns status: ready', '"status":"ready"' in body.replace(' ', ''), f'Got: {body}')

# 1.3 Health check & Sanitization (/health)
status, headers, body = make_req('/health', method='GET')
check('Health Check Endpoint (200 OK)', status == 200, f'Status {status}')
try:
    h_data = json.loads(body)
    check('Health Check reports v2.0.0', h_data.get('version') == '2.0.0', f'Got: {h_data}')
    check('Health Check sanitizes sensitive host cores/commit info', 'cores' not in h_data and 'commit' not in h_data, f'Got: {h_data}')
except Exception as e:
    check('Health Check JSON decode', False, str(e))

status, headers, body = make_req('/api/App/GetVersionInfo', data=[], headers=auth_headers)
try:
    v_data = json.loads(body).get('data', {})
    check('GetVersionInfo returns v2.0.0', v_data.get('version') == '2.0.0' or '2.0.0' in str(v_data), f'Got: {v_data}')
    check('Response Header contains X-Request-ID for distributed tracing', bool(headers.get('X-Request-ID')), f'Got: {headers.get("X-Request-ID")}')
except Exception as e:
    check('GetVersionInfo returns v2.0.0', False, str(e))

# 2. Security Headers (HSTS, CSP without unsafe-eval, X-Frame-Options)
status, headers, body = make_req('/', method='GET')
check('Header X-Frame-Options: DENY', headers.get('X-Frame-Options') == 'DENY', f'Got: {headers.get("X-Frame-Options")}')
check('Header X-Content-Type-Options: nosniff', headers.get('X-Content-Type-Options') == 'nosniff', f'Got: {headers.get("X-Content-Type-Options")}')
check('Header CSP contains default-src', 'default-src' in (headers.get('Content-Security-Policy') or ''), f'Got: {headers.get("Content-Security-Policy")}')
check('Header CSP strips unsafe-eval (hardened anti-XSS)', 'unsafe-eval' not in (headers.get('Content-Security-Policy') or ''), f'Got: {headers.get("Content-Security-Policy")}')
check('Header Strict-Transport-Security (HSTS) enforced', bool(headers.get('Strict-Transport-Security')), f'Got: {headers.get("Strict-Transport-Security")}')
check('Header Referrer-Policy strict-origin', headers.get('Referrer-Policy') == 'strict-origin-when-cross-origin', f'Got: {headers.get("Referrer-Policy")}')

# 2.1 Gzip Compression Negotiation Check (B-003)
status, headers, body = make_req('/live', method='GET', headers={'Accept-Encoding': 'gzip'})
check('Gzip Compression enabled on server (Content-Encoding: gzip)', headers.get('Content-Encoding') == 'gzip', f'Got: {headers.get("Content-Encoding")}')

# 2.2 Protocol Endpoints & Non-existent Route 404 Isolation (P0-2, P1-7)
status, headers, body = make_req('/robots.txt', method='GET')
check('Robots.txt returns 200 text/plain', status == 200 and 'Disallow: /api/' in body, f'Status {status}, Body: {body[:60]}')

status, headers, body = make_req('/sitemap.xml', method='GET')
check('Sitemap.xml returns 200 XML', status == 200 and 'urlset' in body, f'Status {status}, Body: {body[:60]}')

status, headers, body = make_req('/openapi.json', method='GET')
check('OpenAPI specification returns 200 JSON (P0-3, P1-7)', status == 200 and 'openapi' in body and '/api/App/' in body, f'Status {status}')

status, headers, body = make_req('/nonexistent_static_file.txt', method='GET')
check('Nonexistent static file returns real 404 Not Found (P0-2)', status == 404, f'Status: {status}')

status, headers, body = make_req('/api/nonexistent_route', method='GET')
check('Nonexistent /api/ path returns 404 JSON instead of 401 (P0-2)', status == 404, f'Status: {status}')

status, headers, body = make_req('/api/App/NonExistentMethod', method='POST', data=[])
check('Nonexistent RPC method returns 404 JSON instead of 401 (P0-2)', status == 404, f'Status: {status}')

# 3. Auth Login & Rate Limiting Guard
status, headers, body = make_req('/api/auth/login', data={'token': TOKEN})
check('Auth Login with unified token 2266762 succeeds (200)', status == 200, f'Status: {status}, Body: {body}')
set_cookie = headers.get('Set-Cookie') or ''
check('Auth Login returns blades_token cookie', 'blades_token=' in set_cookie, f'Set-Cookie: {set_cookie}')

auth_headers = {'Authorization': f'Bearer {TOKEN}'}

# 4. Secret Masking in GetConfig and ExportConfig
status, headers, body = make_req('/api/App/GetConfig', data=[], headers=auth_headers)
check('GetConfig returns 200', status == 200, f'Status: {status}')
try:
    cfg = json.loads(body).get('data', {})
    keys_str = json.dumps(cfg)
    check('GetConfig masks commercial keys (contains ****)', '****' in keys_str, 'No masked keys found')
except Exception as e:
    check('GetConfig masks commercial keys', False, str(e))

status, headers, body = make_req('/api/App/ExportConfig', data=[], headers=auth_headers)
check('ExportConfig returns 200', status == 200, f'Status: {status}')
try:
    exp_cfg = json.loads(body).get('data', '')
    check('ExportConfig masks commercial keys (contains ****)', '****' in exp_cfg, f'Got: {exp_cfg[:100]}')
except Exception as e:
    check('ExportConfig masks commercial keys', False, str(e))

# 5. Deterministic 3D Quant Diagnosis (250-bar, metadata attribution, trimmed equity)
print('\n--- Testing Deterministic 3D Quant Diagnosis ---')
status, headers, body = make_req('/api/App/DiagnoseStock', data=['600519'], headers=auth_headers)
check('DiagnoseStock(600519) returns 200', status == 200, f'Status: {status}')
try:
    diag = json.loads(body).get('data', {})
    check('Diagnosis WindowBars is standard annual window (240~250 bars)', 240 <= (diag.get('windowBars') or 0) <= 250, f'WindowBars: {diag.get("windowBars")}')
    check('Diagnosis DataWindowDesc is populated', bool(diag.get('dataWindowDesc')), f'Desc: {diag.get("dataWindowDesc")}')
    check('Diagnosis DataSource is attributed (QFQ)', 'QFQ' in (diag.get('dataSource') or ''), f'Source: {diag.get("dataSource")}')
    check('Diagnosis CodeVersion is v2.0.0', diag.get('codeVersion') == 'v2.0.0', f'CodeVersion: {diag.get("codeVersion")}')
    check('Diagnosis ComputedAt timestamp is populated', bool(diag.get('computedAt')), f'ComputedAt: {diag.get("computedAt")}')
    
    quant_dim = diag.get('quantDim', {})
    eq_curve = quant_dim.get('equityCurve', [])
    check('Quant EquityCurve has data (>10 points)', len(eq_curve) > 10, f'Length: {len(eq_curve)}')
    if len(eq_curve) > 2:
        check('EquityCurve initial synthetic flat prefix is trimmed', eq_curve[0] != eq_curve[1] or eq_curve[1] != eq_curve[2] or len(eq_curve) < 200, f'First 5 values: {eq_curve[:5]}')
    
    # Target price behavior
    if diag.get('overallScore', 0) < 50:
        check('Bearish rating suppresses target price (0 or note)', diag.get('targetPrice') == 0 or bool(diag.get('targetPriceNote')), f'Target: {diag.get("targetPrice")}')
    else:
        check('Bullish/neutral rating provides target price (>0)', diag.get('targetPrice', 0) > 0, f'Target: {diag.get("targetPrice")}')
except Exception as e:
    check('Diagnosis reproducibility metadata & equity curve check', False, str(e))

# 5.0.1 510300 3D Quant Diagnosis Remediation Probe (QA Defect 1-5 Checks)
status, headers, body = make_req('/api/App/DiagnoseStock', data=['510300'], headers=auth_headers)
check('DiagnoseStock(510300) returns 200', status == 200, f'Status: {status}')
try:
    d300 = json.loads(body).get('data', {})
    check('510300 逢高减持输出明确反抽减持触发位 (>0，如 MA20=4.21)', d300.get('targetPrice', 0) > 0 and '反抽' in d300.get('targetPriceNote', ''), f'TargetPrice: {d300.get("targetPrice")}, Note: {d300.get("targetPriceNote")}')
    q_dim = d300.get('quantDim', {})
    check('510300 量化回测附带买入持有基准收益曲线 (BenchmarkCurve)', len(q_dim.get('benchmarkCurve', [])) > 10, f'Len: {len(q_dim.get("benchmarkCurve", []))}')
    ht = d300.get('historyTrackRecord', '')
    check('510300 AI历史预测复盘胜率剔除双重乘百Bug (无 5500%)', '5500' not in ht, f'Got: {ht}')
    check('510300 资金维度 HasData 正确识别为 False (无二级资金流)', d300.get('moneyDim', {}).get('hasData') is False, f'HasData: {d300.get("moneyDim", {}).get("hasData")}')
except Exception as e:
    check('510300 3D Quant Diagnosis checks', False, str(e))

# 5.1 Deterministic Pre-Market Holding Risk Control Scan (Zero-Token Rule & QA Remediation)
print('\n--- Testing Deterministic Pre-Market Holding Risk Control Scan ---')
seven_stocks_payload = [
    [
        {'stockCode': '600519', 'stockName': '贵州茅台', 'currentPrice': 1469.27, 'changePercent': -0.72},
        {'stockCode': '512480', 'stockName': '半导体ETF', 'currentPrice': 0.730, 'changePercent': 3.85},
        {'stockCode': '002594', 'stockName': '比亚迪', 'currentPrice': 85.77, 'changePercent': 0.97},
        {'stockCode': '000100', 'stockName': 'TCL科技', 'currentPrice': 4.65, 'changePercent': 2.20},
        {'stockCode': '510300', 'stockName': '沪深300ETF', 'currentPrice': 4.12, 'changePercent': 0.81},
        {'stockCode': '002906', 'stockName': '华阳集团', 'currentPrice': 24.46, 'changePercent': 1.70},
        {'stockCode': '300750', 'stockName': '宁德时代', 'currentPrice': 253.75, 'changePercent': 1.78},
    ]
]

status, headers, body = make_req('/api/App/ScanHoldingRisk', data=seven_stocks_payload, headers=auth_headers)
check('ScanHoldingRisk returns 200', status == 200, f'Status: {status}')
try:
    scan_res = json.loads(body).get('data', {})
    items_list = scan_res.get('items', [])
    check('ScanHoldingRisk returned all 7 target stocks', len(items_list) == 7, f'Got: {len(items_list)}')
    item_map = {it.get('stockCode'): it for it in items_list}

    # 1. 贵州茅台 (600519) 必须为 🟢 多头持有 (反向误判根治)
    maotai = item_map.get('600519', {})
    check('贵州茅台 (600519) 判定为 🟢 GREEN 多头持有 (禁止误判为 🔴 止损)', maotai.get('riskTier') == 'GREEN' and '多头' in maotai.get('tierTitle', ''), f'Tier: {maotai.get("tierTitle")}')
    check('贵州茅台 (600519) 均线为多头排列且站上MA20', maotai.get('alignment') == '多头排列' and maotai.get('distanceToMA20', 0) > 0, f'Alignment: {maotai.get("alignment")}, Dist: {maotai.get("distanceToMA20")}')

    # 2. 半导体ETF (512480) 必须为 🔴 禁止新增仓/破位 (反向误判根治)
    semi = item_map.get('512480', {})
    check('半导体ETF (512480) 判定为 🔴 RED (禁止误判为 🟢 进攻跟踪)', semi.get('riskTier') == 'RED', f'Tier: {semi.get("tierTitle")}')
    check('半导体ETF (512480) 策略严禁包含进攻或依托MA5', '进攻' not in semi.get('actionAdvice', '') and '依托MA5' not in semi.get('actionAdvice', ''), f'Advice: {semi.get("actionAdvice")}')

    # 3. 比亚迪 (002594) 必须为 🟢 GREEN (禁止误判为 🟡 MA20下方)
    byd = item_map.get('002594', {})
    check('比亚迪 (002594) 判定为 🟢 GREEN 多头趋势', byd.get('riskTier') == 'GREEN' and byd.get('distanceToMA20', 0) > 0, f'Tier: {byd.get("tierTitle")}, Dist: {byd.get("distanceToMA20")}')

    # 4. TCL科技 (000100) 必须为 🟡 YELLOW (待放量确认，禁止误判为 🟢 进攻)
    tcl = item_map.get('000100', {})
    check('TCL科技 (000100) 判定为 🟡 YELLOW 待放量确认', tcl.get('riskTier') == 'YELLOW', f'Tier: {tcl.get("tierTitle")}')

    # 5. 沪深300ETF (510300) 防守点位必须实时动态计算 (不可搬运旧数据 4.02)
    etf300 = item_map.get('510300', {})
    check('沪深300ETF (510300) 实时动态计算防守/均线点位 (约 4.15~4.25)', 4.15 <= etf300.get('ma20', 0) <= 4.25, f'MA20: {etf300.get("ma20")}')

    # 6. 华阳集团 (002906) 判定为 🔴 RED (空头禁抄底)
    hy = item_map.get('002906', {})
    check('华阳集团 (002906) 判定为 🔴 RED 空头禁抄底', hy.get('riskTier') == 'RED', f'Tier: {hy.get("tierTitle")}')

    # 7. 宁德时代 (300750) 判定为 🟡 YELLOW (MA20下方)
    catl = item_map.get('300750', {})
    check('宁德时代 (300750) 判定为 🟡 YELLOW MA20下方', catl.get('riskTier') == 'YELLOW', f'Tier: {catl.get("tierTitle")}')

    md_report = scan_res.get('markdownReport', '')
    cjk_count = len([c for c in md_report if '\u4e00' <= c <= '\u9fff'])
    check('Markdown 报告精炼且符合字数约束 (<= 350 CJK字)', cjk_count <= 350, f'CJK Count: {cjk_count}')
    check('Markdown 报告包含完整数据溯源 (日期+前复权+指标口径)', '前复权' in md_report and 'MA(5/10/20)' in md_report, f'Report tail: {md_report[-120:]}')
except Exception as e:
    check('ScanHoldingRisk payload evaluation', False, str(e))

# 6. Buy-Side Stock Thesis Ledger CRUD & Validation
print('\n--- Testing Buy-Side Thesis Ledger ---')
status, headers, body = make_req('/api/App/SaveStockThesis', data=['x', 'Invalid', 'NEW_THESIS', '定论', '核心逻辑', '催化', '风险', 'test'], headers=auth_headers)
check('SaveStockThesis with invalid code "x" is rejected', status != 200 or '无效' in body or '失败' in body, f'Body: {body}')

status, headers, body = make_req('/api/App/SaveStockThesis', data=['600519', '贵州茅台', 'NEW_THESIS', '买入', '白酒行业龙头，高端白酒韧性凸显', '双节批价回暖', '商务消费不及预期', 'Blades-Agent'], headers=auth_headers)
check('SaveStockThesis for 600519 succeeds', status == 200, f'Status: {status}, Body: {body}')

status, headers, body = make_req('/api/App/GetStockThesis', data=['600519'], headers=auth_headers)
try:
    thesis = json.loads(body).get('data', {})
    check('GetStockThesis retrieves saved thesis', thesis.get('stockCode') == '600519' and '白酒行业龙头' in thesis.get('coreThesis', ''), f'Got: {thesis}')
except Exception as e:
    check('GetStockThesis retrieves saved thesis', False, str(e))

status, headers, body = make_req('/api/App/GetStockThesisHistory', data=['600519', 10], headers=auth_headers)
try:
    hist = json.loads(body).get('data', [])
    check('GetStockThesisHistory returns historical list', len(hist) >= 1, f'Count: {len(hist)}')
except Exception as e:
    check('GetStockThesisHistory returns historical list', False, str(e))

status, headers, body = make_req('/api/App/DeleteStockThesis', data=['600519'], headers=auth_headers)
check('DeleteStockThesis API operational', status == 200, f'Status: {status}')

# 7. Registered RPC Methods & Missing Tools Exposure
print('\n--- Testing Registered RPC Methods ---')
status, headers, body = make_req('/api/App/GetEastMoneyKLineWithMA', data=['600519', 'day', 60], headers=auth_headers)
check('RPC GetEastMoneyKLineWithMA returns 200', status == 200 and 'data' in json.loads(body), f'Body: {body[:80]}')

status, headers, body = make_req('/api/App/GetStockFinancialInfo', data=['600519'], headers=auth_headers)
check('RPC GetStockFinancialInfo returns 200', status == 200, f'Status: {status}')

status, headers, body = make_req('/api/App/GetStockLatestFinance', data=['600519'], headers=auth_headers)
check('RPC GetStockLatestFinance returns 200', status == 200, f'Status: {status}')

status, headers, body = make_req('/api/App/GetStockHolderTrend', data=['600519'], headers=auth_headers)
check('RPC GetStockHolderTrend returns 200', status == 200, f'Status: {status}')

status, headers, body = make_req('/api/App/SearchStockByIndicators', data=[{'keyword': '白酒'}], headers=auth_headers)
check('RPC SearchStockByIndicators returns 200', status == 200, f'Status: {status}')

status, headers, body = make_req('/api/App/FilterStocks', data=[{'pageSize': 10}], headers=auth_headers)
check('RPC FilterStocks returns 200', status == 200, f'Status: {status}')

status, headers, body = make_req('/api/App/WebSearch', data=['茅台最新财报', 5], headers=auth_headers)
check('RPC WebSearch returns 200', status == 200, f'Status: {status}')

# 8. Followed Stocks Real-Time Quote Backfill & K-Line Limit
print('\n--- Testing Followed Stocks & K-Line ---')
status, headers, body = make_req('/api/App/GetFollowList', data=[0], headers=auth_headers)
try:
    flist = json.loads(body).get('data', [])
    check('GetFollowList returns core benchmarks', len(flist) >= 5, f'Count: {len(flist)}')
    prices = [s.get('Price') or s.get('price') or s.get('currentPrice') or 0 for s in flist[:3]]
    non_zero = all(p > 0 for p in prices)
    check('Core followed stocks have real-time prices backfilled (>0)', non_zero, f'Prices: {prices}')
except Exception as e:
    check('GetFollowList prices backfill', False, str(e))

status, headers, body = make_req('/api/App/GetStockKLineWithFallback', data=['600519', '贵州茅台', '101', 60], headers=auth_headers)
try:
    k_res = json.loads(body).get('data', {})
    kdata = k_res.get('data', []) if isinstance(k_res, dict) else k_res
    check('GetStockKLineWithFallback strictly respects limit=60', len(kdata) <= 60 and len(kdata) >= 50, f'Length: {len(kdata)}')
except Exception as e:
    check('GetStockKLineWithFallback strictly respects limit', False, str(e))

status, headers, body = make_req('/api/App/GetStockMinutePriceLineData', data=['600519', '贵州茅台'], headers=auth_headers)
try:
    mdata = json.loads(body).get('data', {})
    pts = mdata.get('priceData', []) if isinstance(mdata, dict) else []
    check('GetStockMinutePriceLineData returns intraday minute points', len(pts) > 100, f'Points: {len(pts)}')
except Exception as e:
    check('GetStockMinutePriceLineData intraday points', False, str(e))

print(f'\n======================================================')
print(f'Total Checks: {passed + failed} | Passed: {passed} | Failed: {failed}')
print(f'======================================================')

if failed > 0:
    sys.exit(1)
print('>>> ALL AUDIT & VERIFICATION CHECKS PASSED WITH 100% SUCCESS! <<<')
