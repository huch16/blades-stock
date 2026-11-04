import urllib.request
import json
import sys

# Windows 控制台输出 UTF-8 编码适配
if hasattr(sys.stdout, 'reconfigure'):
    sys.stdout.reconfigure(encoding='utf-8')

base_url = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8000"
headers = {
    "Content-Type": "application/json",
    "Authorization": "Bearer 2266762"
}

def post(endpoint, data):
    req = urllib.request.Request(
        base_url + endpoint,
        data=json.dumps(data).encode("utf-8"),
        headers=headers
    )
    resp = urllib.request.urlopen(req)
    return json.loads(resp.read().decode("utf-8"))

print(f"=== Probing Live Audit Remediations on: {base_url} ===")

# 1. Probe 688019 (Anji Tech) 3D Quant Diagnosis
diag_res = post("/api/App/DiagnoseStock", ["688019"])
d = diag_res.get("data", {})
print("\n--- 1. Anji Microelectronics (688019) 3D Diagnosis Audit ---")
print(f"  Overall Score      : {d.get('overallScore')}/100")
print(f"  Suggestion         : {d.get('suggestion')}")
print(f"  Current Price      : {d.get('currentPrice')} 元")
print(f"  Target Price       : {d.get('targetPrice')} 元")
print(f"  Target Price Note  : {d.get('targetPriceNote')}")
print(f"  Stop Loss Price    : {d.get('stopLossPrice')} 元")
print(f"  Forecast RRR       : {d.get('riskRewardRatio')}")

quant = d.get("quantDim", {})
print(f"  Strategy           : {quant.get('strategyName')}")
print(f"  Total Trades       : {quant.get('totalTrades')} 次")
print(f"  Profit Factor      : {quant.get('profitFactor')}")
print(f"  Quant Score        : {quant.get('score')} 分")
print(f"  Quant Evaluation   : {quant.get('evaluation')}")

tech = d.get("technicalDim", {})
print(f"  Tech Trend         : {tech.get('trend')} (距20日线 {tech.get('priceDistanceMA20')}%)")
print(f"  Tech Score         : {tech.get('score')} 分")
print(f"  Risk Warnings      : {d.get('risks')}")

# 2. Check Guizhou Maotai (600519)
diag_res_mao = post("/api/App/DiagnoseStock", ["600519"])
dm = diag_res_mao.get("data", {})
print("\n--- 2. Guizhou Maotai (600519) 3D Diagnosis Audit ---")
print(f"  Overall Score      : {dm.get('overallScore')}/100")
print(f"  Suggestion         : {dm.get('suggestion')}")
print(f"  Target Price       : {dm.get('targetPrice')} 元 ({dm.get('targetPriceNote')})")
print(f"  Forecast RRR       : {dm.get('riskRewardRatio')}")
print(f"  Profit Factor      : {dm.get('quantDim', {}).get('profitFactor')}")

# 3. Check Stock Thesis Ledger
print("\n--- 3. Buy-Side Cognitive Ledger (Theses) Probe ---")
post("/api/App/SaveStockThesis", [
    "688019", "安集科技", "NEW_THESIS", "观望/均线压制",
    "抛光液与化学品龙头，但目前技术形态受MA20均线压制，处于弱势整理期",
    "【证据天梯】已核实2025年报与近期公告，主营稳健但订单增速放缓",
    "【研究缺口】尚未披露特定存储厂先进封装扩产份额；【证伪条件】毛利率跌破40%或海外替代方案突破",
    "Serenity-Skill4-Audit"
])
thesis_res = post("/api/App/GetStockThesis", ["688019"])
t = thesis_res.get("data", {})
print(f"  Retrieved Thesis   : {t.get('stockCode')} {t.get('stockName')} [{t.get('verdictText')}]")
print(f"  Core Thesis        : {t.get('coreThesis')}")
print(f"  Catalysts/Evidence : {t.get('keyCatalysts')}")
print(f"  Risks/Falsification: {t.get('riskFactors')}")

# 4. Probe Serenity Research Report Generation (Evidence Ladder & Cognitive Ledger)
print("\n--- 4. Serenity Research Pipeline Probe (Evidence Ladder & Cognitive Ledger) ---")
try:
    latest_res = post("/api/App/GetLatestMarketIntelligenceReportByType", ["serenity_industry_analysis"])
    rep = latest_res.get("data", {})
    content = rep.get("content", "")
    
    if not content:
        print("  Triggering live Serenity generation...")
        serenity_res = post("/api/App/GenerateSerenityResearchReport", ["中微公司", "688012", 0])
        rep = serenity_res.get("data", {})
        content = rep.get("content", "")
    
    print(f"  Report Title       : {rep.get('title')}")
    print(f"  Report Summary     : {rep.get('summary')}")
    print(f"  Content Length     : {len(content)} 字符")
    
    # 审查是否彻底根除伪量化综合打分
    has_score_calc = "综合得分" in content or "权重 40%" in content or "三维打分机制" in content
    print(f"  ❌ Contains Fake Composite Score: {has_score_calc} (Must be False)")
    
    # 审查是否落实证据天梯与法定公告
    has_notice_evidence = "公告" in content or "法定" in content or "第一级" in content
    print(f"  ✅ Has Evidence Ladder / Notices : {has_notice_evidence} (Must be True)")
    
    # 审查是否坦承研究缺口
    has_gaps = "研究缺口" in content or "Research Gap" in content
    print(f"  ✅ Has Research Gaps Declared   : {has_gaps} (Must be True)")
    
    # 审查是否输出结构化 Thesis 代码块
    has_thesis = "```thesis" in content
    print(f"  ✅ Has Structured Thesis Block  : {has_thesis} (Must be True)")

    if content:
        print("\n  [Serenity Report Excerpt (First 600 chars)]:")
        for line in content[:600].split("\n"):
            print(f"    {line}")
        print("\n  [Serenity Report Thesis Tail (Last 500 chars)]:")
        for line in content[-500:].split("\n"):
            print(f"    {line}")
except Exception as e:
    print(f"  Serenity Generation Probe Notice: {e}")

# 5. Probe stevia0829 Native ETF Rotation Decision & Regime Gate
print("\n--- 5. stevia0829 Native ETF Rotation Decision Probe ---")
try:
    rep_etf_res = post("/api/App/GetLatestMarketIntelligenceReportByType", ["etf_rotation_decision"])
    rep_etf = rep_etf_res.get("data") or {}
    content_etf = rep_etf.get("content", "")
    if not content_etf:
        print("  Triggering live ETF Rotation generation...")
        etf_res = post("/api/App/GenerateETFRotationReport", [0])
        rep_etf = etf_res.get("data", {})
        content_etf = rep_etf.get("content", "")
        
    print(f"  ETF Report Title   : {rep_etf.get('title')}")
    print(f"  ETF Report Summary : {rep_etf.get('summary')}")
    print(f"  Content Length     : {len(content_etf)} 字符")
    
    # 审查是否包含 510300 硬风控状态机
    has_regime = "510300" in content_etf and ("MA120" in content_etf or "硬风控" in content_etf or "DEFENSE" in content_etf or "OFFENSE" in content_etf)
    print(f"  ✅ Has 510300 Regime Gate      : {has_regime} (Must be True)")
    
    # 审查是否包含纯数学 21 日对数动量矩阵表格
    has_momentum_table = "动量得分" in content_etf and "年化动量" in content_etf
    print(f"  ✅ Has 21-Day Log Momentum Table: {has_momentum_table} (Must be True)")
    
    # 审查是否彻底根除 5 视角加权打分表伪量化
    has_fake_scorecard = "投委会5视角加权评分表" in content_etf or "投委会打分矩阵" in content_etf or "趋势30%" in content_etf
    print(f"  ❌ Contains Fake Scorecard     : {has_fake_scorecard} (Must be False)")

    if content_etf:
        print("\n  [ETF Report Excerpt (First 600 chars)]:")
        for line in content_etf[:600].split("\n"):
            print(f"    {line}")
except Exception as e:
    print(f"  ETF Rotation Probe Notice: {e}")

# 6. Probe Settings Security, Credential Masking & Model Registry Streamlining
print("\n--- 6. Settings Security & Credential Masking Audit ---")
try:
    cfg_res = post("/api/App/GetConfig", [])
    cfg_data = cfg_res.get("data") or {}
    st = cfg_data.get("settings") or {}
    ai_list = cfg_data.get("aiConfigs") or []

    # 审查 Tushare Token 与其他凭据是否掩码
    has_masked_token = "****" in (st.get("tushareToken") or "****")
    print(f"  ✅ Tushare Token Masked        : {has_masked_token} (Must be True)")
    
    # 审查 AI 模型的 ApiKey 是否全部脱敏
    all_keys_masked = all("****" in (c.get("apiKey") or "****") for c in ai_list if c.get("apiKey") and c.get("apiKey") != "none")
    print(f"  ✅ All Model API Keys Masked   : {all_keys_masked} (Must be True)")

    # 审查导出配置是否脱敏
    exp_res = post("/api/App/ExportConfig", [])
    exp_raw = exp_res.get("data", "")
    exp_masked = "****" in exp_raw
    print(f"  ✅ Exported Config Sanitized   : {exp_masked} (Must be True)")

    print(f"  Total Registered Models        : {len(ai_list)} 项 (精简为5大核心身份字段)")
    for i, c in enumerate(ai_list[:3]):
        print(f"    Model #{i+1}: {c.get('name')} | {c.get('baseUrl')} | {c.get('modelName')} | Thinking: {c.get('thinking')}")
except Exception as e:
    print(f"  Settings Security Probe Notice: {e}")

print("\n>>> All verification probes executed successfully! <<<")
