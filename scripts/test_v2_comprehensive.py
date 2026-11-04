import urllib.request
import urllib.error
import json
import time
import sys

BASE_URL = 'https://8000-agqxrg26lk2za3jpqp2izbt7za.instabox.tech'
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
            body = resp.read().decode('utf-8')
            return resp.status, resp.headers, body
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

# 2. Security Headers
status, headers, body = make_req('/', method='GET')
check('Header X-Frame-Options: DENY', headers.get('X-Frame-Options') == 'DENY', f'Got: {headers.get("X-Frame-Options")}')
check('Header X-Content-Type-Options: nosniff', headers.get('X-Content-Type-Options') == 'nosniff', f'Got: {headers.get("X-Content-Type-Options")}')
check('Header CSP contains default-src', 'default-src' in (headers.get('Content-Security-Policy') or ''), f'Got: {headers.get("Content-Security-Policy")}')
check('Header Referrer-Policy strict-origin', headers.get('Referrer-Policy') == 'strict-origin-when-cross-origin', f'Got: {headers.get("Referrer-Policy")}')

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
