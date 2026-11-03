export namespace agent {
	
	export class DistillationStats {
	    totalCrystals: number;
	    prunedNewsCount: number;
	    prunedMemories: number;
	    prunedReports: number;
	    averageAccuracy: number;
	    totalHits: number;
	    // Go type: time
	    lastRunAt: any;
	    databaseSizeMB: number;
	    estimatedFreedMB: number;
	    compressionRatio: number;
	
	    static createFrom(source: any = {}) {
	        return new DistillationStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalCrystals = source["totalCrystals"];
	        this.prunedNewsCount = source["prunedNewsCount"];
	        this.prunedMemories = source["prunedMemories"];
	        this.prunedReports = source["prunedReports"];
	        this.averageAccuracy = source["averageAccuracy"];
	        this.totalHits = source["totalHits"];
	        this.lastRunAt = this.convertValues(source["lastRunAt"], null);
	        this.databaseSizeMB = source["databaseSizeMB"];
	        this.estimatedFreedMB = source["estimatedFreedMB"];
	        this.compressionRatio = source["compressionRatio"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class KnowledgeCrystal {
	    id: number;
	    category: string;
	    title: string;
	    triggerPattern: string;
	    summary: string;
	    keyInsights: string;
	    accuracyScore: number;
	    hitCount: number;
	    // Go type: time
	    lastHitAt: any;
	    vectorDim: number;
	    sourceDate: string;
	    sourceRef: string;
	    contentHash: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeCrystal(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.category = source["category"];
	        this.title = source["title"];
	        this.triggerPattern = source["triggerPattern"];
	        this.summary = source["summary"];
	        this.keyInsights = source["keyInsights"];
	        this.accuracyScore = source["accuracyScore"];
	        this.hitCount = source["hitCount"];
	        this.lastHitAt = this.convertValues(source["lastHitAt"], null);
	        this.vectorDim = source["vectorDim"];
	        this.sourceDate = source["sourceDate"];
	        this.sourceRef = source["sourceRef"];
	        this.contentHash = source["contentHash"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MarketIntelligenceReport {
	    type: string;
	    title: string;
	    summary: string;
	    content: string;
	    actions?: types.AgentAction[];
	    // Go type: time
	    generatedAt: any;
	    tradingDate: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new MarketIntelligenceReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.title = source["title"];
	        this.summary = source["summary"];
	        this.content = source["content"];
	        this.actions = this.convertValues(source["actions"], types.AgentAction);
	        this.generatedAt = this.convertValues(source["generatedAt"], null);
	        this.tradingDate = source["tradingDate"];
	        this.source = source["source"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MarketLifecycleStatus {
	    phase: string;
	    phaseName: string;
	    isTradingDay: boolean;
	    isTradingTime: boolean;
	    currentTime: string;
	    marketStateDesc: string;
	    nextEvent: string;
	    nextEventTime: string;
	
	    static createFrom(source: any = {}) {
	        return new MarketLifecycleStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.phase = source["phase"];
	        this.phaseName = source["phaseName"];
	        this.isTradingDay = source["isTradingDay"];
	        this.isTradingTime = source["isTradingTime"];
	        this.currentTime = source["currentTime"];
	        this.marketStateDesc = source["marketStateDesc"];
	        this.nextEvent = source["nextEvent"];
	        this.nextEventTime = source["nextEventTime"];
	    }
	}
	export class PortfolioRiskItem {
	    stockCode: string;
	    stockName: string;
	    costPrice: number;
	    currPrice: number;
	    profitRate: number;
	    isWarning: boolean;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new PortfolioRiskItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.costPrice = source["costPrice"];
	        this.currPrice = source["currPrice"];
	        this.profitRate = source["profitRate"];
	        this.isWarning = source["isWarning"];
	        this.reason = source["reason"];
	    }
	}
	export class QuotaStatus {
	    date: string;
	    dailyCalls: number;
	    dailyCostUSD: number;
	
	    static createFrom(source: any = {}) {
	        return new QuotaStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.dailyCalls = source["dailyCalls"];
	        this.dailyCostUSD = source["dailyCostUSD"];
	    }
	}
	export class SystemStateSnapshot {
	    timestamp: string;
	    clockPhase: string;
	    phaseDesc: string;
	    trafficLight: string;
	    trafficDesc: string;
	    upCount: number;
	    downCount: number;
	    limitUpCount: number;
	    limitDownCount: number;
	    holdingsCount: number;
	    profitingCount: number;
	    losingCount: number;
	    riskHoldings: PortfolioRiskItem[];
	    summaryPrompt: string;
	
	    static createFrom(source: any = {}) {
	        return new SystemStateSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.clockPhase = source["clockPhase"];
	        this.phaseDesc = source["phaseDesc"];
	        this.trafficLight = source["trafficLight"];
	        this.trafficDesc = source["trafficDesc"];
	        this.upCount = source["upCount"];
	        this.downCount = source["downCount"];
	        this.limitUpCount = source["limitUpCount"];
	        this.limitDownCount = source["limitDownCount"];
	        this.holdingsCount = source["holdingsCount"];
	        this.profitingCount = source["profitingCount"];
	        this.losingCount = source["losingCount"];
	        this.riskHoldings = this.convertValues(source["riskHoldings"], PortfolioRiskItem);
	        this.summaryPrompt = source["summaryPrompt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace data {
	
	export class AIConfig {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    name: string;
	    baseUrl: string;
	    apiKey: string;
	    modelName: string;
	    maxTokens: number;
	    temperature: number;
	    timeOut: number;
	    httpProxy: string;
	    httpProxyEnabled: boolean;
	    sessionId: string;
	    thinking: boolean;
	    embeddingEnabled: boolean;
	    embeddingModelName: string;
	    embeddingBaseUrl: string;
	    llmConcurrencyLimit: number;
	    subAgentTimeout: number;
	    rateLimitPerSecond: number;
	    burst: number;
	    dailyRequestQuota: number;
	    dailyCostQuota: number;
	
	    static createFrom(source: any = {}) {
	        return new AIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.name = source["name"];
	        this.baseUrl = source["baseUrl"];
	        this.apiKey = source["apiKey"];
	        this.modelName = source["modelName"];
	        this.maxTokens = source["maxTokens"];
	        this.temperature = source["temperature"];
	        this.timeOut = source["timeOut"];
	        this.httpProxy = source["httpProxy"];
	        this.httpProxyEnabled = source["httpProxyEnabled"];
	        this.sessionId = source["sessionId"];
	        this.thinking = source["thinking"];
	        this.embeddingEnabled = source["embeddingEnabled"];
	        this.embeddingModelName = source["embeddingModelName"];
	        this.embeddingBaseUrl = source["embeddingBaseUrl"];
	        this.llmConcurrencyLimit = source["llmConcurrencyLimit"];
	        this.subAgentTimeout = source["subAgentTimeout"];
	        this.rateLimitPerSecond = source["rateLimitPerSecond"];
	        this.burst = source["burst"];
	        this.dailyRequestQuota = source["dailyRequestQuota"];
	        this.dailyCostQuota = source["dailyCostQuota"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AiConfigTemplate {
	    type: string;
	    label: string;
	    description: string;
	    baseUrl: string;
	    modelName: string;
	    apiKeyRequired: boolean;
	    envHint: string;
	
	    static createFrom(source: any = {}) {
	        return new AiConfigTemplate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.label = source["label"];
	        this.description = source["description"];
	        this.baseUrl = source["baseUrl"];
	        this.modelName = source["modelName"];
	        this.apiKeyRequired = source["apiKeyRequired"];
	        this.envHint = source["envHint"];
	    }
	}
	export class AllStockInfoPageData {
	    list: models.AllStockInfo[];
	    total: number;
	    page: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new AllStockInfoPageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], models.AllStockInfo);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AllStockInfoQuery {
	    page: number;
	    pageSize: number;
	    securityCode: string;
	    securityName: string;
	    market: string;
	    industry: string;
	    concept: string;
	    minPrice: string;
	    maxPrice: string;
	    minChange: string;
	    maxChange: string;
	    searchKeyWord: string;
	
	    static createFrom(source: any = {}) {
	        return new AllStockInfoQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.securityCode = source["securityCode"];
	        this.securityName = source["securityName"];
	        this.market = source["market"];
	        this.industry = source["industry"];
	        this.concept = source["concept"];
	        this.minPrice = source["minPrice"];
	        this.maxPrice = source["maxPrice"];
	        this.minChange = source["minChange"];
	        this.maxChange = source["maxChange"];
	        this.searchKeyWord = source["searchKeyWord"];
	    }
	}
	export class BackupFileItem {
	    fileName: string;
	    type: string;
	    typeName: string;
	    size: number;
	    sizeStr: string;
	    // Go type: time
	    createdAt: any;
	    timeStr: string;
	    recordDesc: string;
	
	    static createFrom(source: any = {}) {
	        return new BackupFileItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fileName = source["fileName"];
	        this.type = source["type"];
	        this.typeName = source["typeName"];
	        this.size = source["size"];
	        this.sizeStr = source["sizeStr"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.timeStr = source["timeStr"];
	        this.recordDesc = source["recordDesc"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BackupOverview {
	    aiConfigCount: number;
	    skillCount: number;
	    mcpServerCount: number;
	    chatSessionCount: number;
	    memoryCount: number;
	    aiReportCount: number;
	    aiRecommendCount: number;
	    followedStockCount: number;
	    followedFundCount: number;
	    backtestCount: number;
	    cronTaskCount: number;
	    totalBackupFiles: number;
	    totalBackupSizeStr: string;
	    lastBackupTime: string;
	
	    static createFrom(source: any = {}) {
	        return new BackupOverview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.aiConfigCount = source["aiConfigCount"];
	        this.skillCount = source["skillCount"];
	        this.mcpServerCount = source["mcpServerCount"];
	        this.chatSessionCount = source["chatSessionCount"];
	        this.memoryCount = source["memoryCount"];
	        this.aiReportCount = source["aiReportCount"];
	        this.aiRecommendCount = source["aiRecommendCount"];
	        this.followedStockCount = source["followedStockCount"];
	        this.followedFundCount = source["followedFundCount"];
	        this.backtestCount = source["backtestCount"];
	        this.cronTaskCount = source["cronTaskCount"];
	        this.totalBackupFiles = source["totalBackupFiles"];
	        this.totalBackupSizeStr = source["totalBackupSizeStr"];
	        this.lastBackupTime = source["lastBackupTime"];
	    }
	}
	export class ChangeRankItem {
	    name: string;
	    code?: string;
	    count: number;
	    upCount: number;
	    downCount: number;
	
	    static createFrom(source: any = {}) {
	        return new ChangeRankItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.code = source["code"];
	        this.count = source["count"];
	        this.upCount = source["upCount"];
	        this.downCount = source["downCount"];
	    }
	}
	export class ChangeRankResult {
	    topStocks: ChangeRankItem[];
	    topIndustries: ChangeRankItem[];
	    topConcepts: ChangeRankItem[];
	
	    static createFrom(source: any = {}) {
	        return new ChangeRankResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.topStocks = this.convertValues(source["topStocks"], ChangeRankItem);
	        this.topIndustries = this.convertValues(source["topIndustries"], ChangeRankItem);
	        this.topConcepts = this.convertValues(source["topConcepts"], ChangeRankItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ChangeTypeDailyStats {
	    changeDate: string;
	    typeName: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new ChangeTypeDailyStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.changeDate = source["changeDate"];
	        this.typeName = source["typeName"];
	        this.count = source["count"];
	    }
	}
	export class ChipBin {
	    price: number;
	    vol: number;
	    ratio: number;
	
	    static createFrom(source: any = {}) {
	        return new ChipBin(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.price = source["price"];
	        this.vol = source["vol"];
	        this.ratio = source["ratio"];
	    }
	}
	export class ChipDistributionResult {
	    stockCode: string;
	    days: number;
	    bins: number;
	    current: number;
	    avgCost: number;
	    profitRatio: number;
	    minPrice: number;
	    maxPrice: number;
	    sumVol: number;
	    items: ChipBin[];
	
	    static createFrom(source: any = {}) {
	        return new ChipDistributionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stockCode = source["stockCode"];
	        this.days = source["days"];
	        this.bins = source["bins"];
	        this.current = source["current"];
	        this.avgCost = source["avgCost"];
	        this.profitRatio = source["profitRatio"];
	        this.minPrice = source["minPrice"];
	        this.maxPrice = source["maxPrice"];
	        this.sumVol = source["sumVol"];
	        this.items = this.convertValues(source["items"], ChipBin);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DailyChangeStats {
	    changeDate: string;
	    totalCount: number;
	    upCount: number;
	    downCount: number;
	    limitUp: number;
	    limitDown: number;
	
	    static createFrom(source: any = {}) {
	        return new DailyChangeStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.changeDate = source["changeDate"];
	        this.totalCount = source["totalCount"];
	        this.upCount = source["upCount"];
	        this.downCount = source["downCount"];
	        this.limitUp = source["limitUp"];
	        this.limitDown = source["limitDown"];
	    }
	}
	export class DailyDimensionStats {
	    changeDate: string;
	    upCount: number;
	    downCount: number;
	    totalCount: number;
	
	    static createFrom(source: any = {}) {
	        return new DailyDimensionStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.changeDate = source["changeDate"];
	        this.upCount = source["upCount"];
	        this.downCount = source["downCount"];
	        this.totalCount = source["totalCount"];
	    }
	}
	export class FundBasic {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    code: string;
	    name: string;
	    fullName: string;
	    type: string;
	    establishment: string;
	    scale: string;
	    company: string;
	    manager: string;
	    rating: string;
	    trackingTarget: string;
	    netUnitValue?: number;
	    netUnitValueDate: string;
	    netEstimatedUnit?: number;
	    netEstimatedUnitTime: string;
	    netAccumulated?: number;
	    netGrowth1?: number;
	    netGrowth3?: number;
	    netGrowth6?: number;
	    netGrowth12?: number;
	    netGrowth36?: number;
	    netGrowth60?: number;
	    netGrowthYTD?: number;
	    netGrowthAll?: number;
	
	    static createFrom(source: any = {}) {
	        return new FundBasic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.fullName = source["fullName"];
	        this.type = source["type"];
	        this.establishment = source["establishment"];
	        this.scale = source["scale"];
	        this.company = source["company"];
	        this.manager = source["manager"];
	        this.rating = source["rating"];
	        this.trackingTarget = source["trackingTarget"];
	        this.netUnitValue = source["netUnitValue"];
	        this.netUnitValueDate = source["netUnitValueDate"];
	        this.netEstimatedUnit = source["netEstimatedUnit"];
	        this.netEstimatedUnitTime = source["netEstimatedUnitTime"];
	        this.netAccumulated = source["netAccumulated"];
	        this.netGrowth1 = source["netGrowth1"];
	        this.netGrowth3 = source["netGrowth3"];
	        this.netGrowth6 = source["netGrowth6"];
	        this.netGrowth12 = source["netGrowth12"];
	        this.netGrowth36 = source["netGrowth36"];
	        this.netGrowth60 = source["netGrowth60"];
	        this.netGrowthYTD = source["netGrowthYTD"];
	        this.netGrowthAll = source["netGrowthAll"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FollowedFund {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    code: string;
	    name: string;
	    netUnitValue?: number;
	    netUnitValueDate: string;
	    netEstimatedUnit?: number;
	    netEstimatedUnitTime: string;
	    netAccumulated?: number;
	    netEstimatedRate?: number;
	    netUnitValuePrev?: number;
	    netActualRate?: number;
	    fundBasic: FundBasic;
	
	    static createFrom(source: any = {}) {
	        return new FollowedFund(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.netUnitValue = source["netUnitValue"];
	        this.netUnitValueDate = source["netUnitValueDate"];
	        this.netEstimatedUnit = source["netEstimatedUnit"];
	        this.netEstimatedUnitTime = source["netEstimatedUnitTime"];
	        this.netAccumulated = source["netAccumulated"];
	        this.netEstimatedRate = source["netEstimatedRate"];
	        this.netUnitValuePrev = source["netUnitValuePrev"];
	        this.netActualRate = source["netActualRate"];
	        this.fundBasic = this.convertValues(source["fundBasic"], FundBasic);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FollowedFundPagedResult {
	    items: FollowedFund[];
	    totalCount: number;
	    pageIndex: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new FollowedFundPagedResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], FollowedFund);
	        this.totalCount = source["totalCount"];
	        this.pageIndex = source["pageIndex"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Group {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    name: string;
	    sort: number;
	
	    static createFrom(source: any = {}) {
	        return new Group(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.name = source["name"];
	        this.sort = source["sort"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GroupStock {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    stockCode: string;
	    groupId: number;
	    groupInfo: Group;
	
	    static createFrom(source: any = {}) {
	        return new GroupStock(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.stockCode = source["stockCode"];
	        this.groupId = source["groupId"];
	        this.groupInfo = this.convertValues(source["groupInfo"], Group);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FollowedStock {
	    StockCode: string;
	    Name: string;
	    Volume: number;
	    CostPrice: number;
	    Price: number;
	    PriceChange: number;
	    ChangePercent: number;
	    AlarmChangePercent: number;
	    AlarmPrice: number;
	    // Go type: time
	    Time: any;
	    Sort: number;
	    Cron?: string;
	    IsDel: number;
	    Groups: GroupStock[];
	    AiConfigId: number;
	    EntryPrice: number;
	    TakeProfitPrice: number;
	    StopLossPrice: number;
	
	    static createFrom(source: any = {}) {
	        return new FollowedStock(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.StockCode = source["StockCode"];
	        this.Name = source["Name"];
	        this.Volume = source["Volume"];
	        this.CostPrice = source["CostPrice"];
	        this.Price = source["Price"];
	        this.PriceChange = source["PriceChange"];
	        this.ChangePercent = source["ChangePercent"];
	        this.AlarmChangePercent = source["AlarmChangePercent"];
	        this.AlarmPrice = source["AlarmPrice"];
	        this.Time = this.convertValues(source["Time"], null);
	        this.Sort = source["Sort"];
	        this.Cron = source["Cron"];
	        this.IsDel = source["IsDel"];
	        this.Groups = this.convertValues(source["Groups"], GroupStock);
	        this.AiConfigId = source["AiConfigId"];
	        this.EntryPrice = source["EntryPrice"];
	        this.TakeProfitPrice = source["TakeProfitPrice"];
	        this.StopLossPrice = source["StopLossPrice"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class FundHistoryNetValue {
	    date: string;
	    netValue: number;
	    accumValue: number;
	    dailyGrowth: number;
	    buyStatus: string;
	    sellStatus: string;
	
	    static createFrom(source: any = {}) {
	        return new FundHistoryNetValue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.netValue = source["netValue"];
	        this.accumValue = source["accumValue"];
	        this.dailyGrowth = source["dailyGrowth"];
	        this.buyStatus = source["buyStatus"];
	        this.sellStatus = source["sellStatus"];
	    }
	}
	export class FundHoldingStock {
	    rank: number;
	    stockCode: string;
	    stockName: string;
	    ratio: number;
	    shares: string;
	    marketCap: string;
	    quarter: string;
	    price?: number;
	    changeRate?: number;
	    market: string;
	
	    static createFrom(source: any = {}) {
	        return new FundHoldingStock(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rank = source["rank"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.ratio = source["ratio"];
	        this.shares = source["shares"];
	        this.marketCap = source["marketCap"];
	        this.quarter = source["quarter"];
	        this.price = source["price"];
	        this.changeRate = source["changeRate"];
	        this.market = source["market"];
	    }
	}
	export class FundRankingItem {
	    code: string;
	    name: string;
	    pinyin: string;
	    netValueDate: string;
	    netUnitValue?: number;
	    netAccumulated?: number;
	    dailyGrowth?: number;
	    weekGrowth?: number;
	    monthGrowth?: number;
	    threeMonthGrowth?: number;
	    sixMonthGrowth?: number;
	    yearGrowth?: number;
	    twoYearGrowth?: number;
	    threeYearGrowth?: number;
	    ytdGrowth?: number;
	    sinceInception?: number;
	    establishDate: string;
	    purchasable: boolean;
	    scale?: number;
	    purchaseRate?: number;
	    discountRate?: number;
	    fundTypeDetail: string;
	
	    static createFrom(source: any = {}) {
	        return new FundRankingItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.pinyin = source["pinyin"];
	        this.netValueDate = source["netValueDate"];
	        this.netUnitValue = source["netUnitValue"];
	        this.netAccumulated = source["netAccumulated"];
	        this.dailyGrowth = source["dailyGrowth"];
	        this.weekGrowth = source["weekGrowth"];
	        this.monthGrowth = source["monthGrowth"];
	        this.threeMonthGrowth = source["threeMonthGrowth"];
	        this.sixMonthGrowth = source["sixMonthGrowth"];
	        this.yearGrowth = source["yearGrowth"];
	        this.twoYearGrowth = source["twoYearGrowth"];
	        this.threeYearGrowth = source["threeYearGrowth"];
	        this.ytdGrowth = source["ytdGrowth"];
	        this.sinceInception = source["sinceInception"];
	        this.establishDate = source["establishDate"];
	        this.purchasable = source["purchasable"];
	        this.scale = source["scale"];
	        this.purchaseRate = source["purchaseRate"];
	        this.discountRate = source["discountRate"];
	        this.fundTypeDetail = source["fundTypeDetail"];
	    }
	}
	export class FundRankingResult {
	    items: FundRankingItem[];
	    totalCount: number;
	    pageIndex: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new FundRankingResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], FundRankingItem);
	        this.totalCount = source["totalCount"];
	        this.pageIndex = source["pageIndex"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FundSearchItem {
	    code: string;
	    name: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new FundSearchItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.type = source["type"];
	    }
	}
	
	
	export class KLineData {
	    day: string;
	    open: string;
	    close: string;
	    high: string;
	    low: string;
	    volume: string;
	    amount: string;
	    changePercent: string;
	    changeValue: string;
	    amplitude: string;
	    turnoverRate: string;
	    volumeRatio: string;
	    ma?: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new KLineData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.day = source["day"];
	        this.open = source["open"];
	        this.close = source["close"];
	        this.high = source["high"];
	        this.low = source["low"];
	        this.volume = source["volume"];
	        this.amount = source["amount"];
	        this.changePercent = source["changePercent"];
	        this.changeValue = source["changeValue"];
	        this.amplitude = source["amplitude"];
	        this.turnoverRate = source["turnoverRate"];
	        this.volumeRatio = source["volumeRatio"];
	        this.ma = source["ma"];
	    }
	}
	export class KLineSourceResult {
	    data?: KLineData[];
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new KLineSourceResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.data = this.convertValues(source["data"], KLineData);
	        this.source = source["source"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SettingConfig {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    tushareToken: string;
	    localPushEnable: boolean;
	    dingPushEnable: boolean;
	    dingRobot: string;
	    updateBasicInfoOnStart: boolean;
	    refreshInterval: number;
	    openAiEnable: boolean;
	    aiConfigId: number;
	    prompt: string;
	    checkUpdate: boolean;
	    updateChannel: string;
	    questionTemplate: string;
	    crawlTimeOut: number;
	    kDays: number;
	    browserPath: string;
	    enableNews: boolean;
	    darkTheme: boolean;
	    enableFund: boolean;
	    enablePushNews: boolean;
	    enableOnlyPushRedNews: boolean;
	    httpProxy: string;
	    httpProxyEnabled: boolean;
	    enableAgent: boolean;
	    disableBrowserSearch: boolean;
	    qgqpBId: string;
	    tickFlowAPIKey: string;
	    emApiKey: string;
	    windowWidth: number;
	    windowHeight: number;
	    promptPlazaApiBase: string;
	    backtestAccuracyThreshold: number;
	    backtestVerifyDays: number;
	    backtestEnabled: boolean;
	    akshareServiceUrl: string;
	    aDataServiceUrl: string;
	    defaultDataSource: string;
	    embeddingEnabled: boolean;
	    embeddingModelName: string;
	    embeddingBaseUrl: string;
	    embeddingApiKey: string;
	    mcpTimeout: number;
	    toolRunBudget: number;
	    toolDailyBudget: number;
	    toolDailyCostUSD: number;
	    aiConfigs: AIConfig[];
	
	    static createFrom(source: any = {}) {
	        return new SettingConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.tushareToken = source["tushareToken"];
	        this.localPushEnable = source["localPushEnable"];
	        this.dingPushEnable = source["dingPushEnable"];
	        this.dingRobot = source["dingRobot"];
	        this.updateBasicInfoOnStart = source["updateBasicInfoOnStart"];
	        this.refreshInterval = source["refreshInterval"];
	        this.openAiEnable = source["openAiEnable"];
	        this.aiConfigId = source["aiConfigId"];
	        this.prompt = source["prompt"];
	        this.checkUpdate = source["checkUpdate"];
	        this.updateChannel = source["updateChannel"];
	        this.questionTemplate = source["questionTemplate"];
	        this.crawlTimeOut = source["crawlTimeOut"];
	        this.kDays = source["kDays"];
	        this.browserPath = source["browserPath"];
	        this.enableNews = source["enableNews"];
	        this.darkTheme = source["darkTheme"];
	        this.enableFund = source["enableFund"];
	        this.enablePushNews = source["enablePushNews"];
	        this.enableOnlyPushRedNews = source["enableOnlyPushRedNews"];
	        this.httpProxy = source["httpProxy"];
	        this.httpProxyEnabled = source["httpProxyEnabled"];
	        this.enableAgent = source["enableAgent"];
	        this.disableBrowserSearch = source["disableBrowserSearch"];
	        this.qgqpBId = source["qgqpBId"];
	        this.tickFlowAPIKey = source["tickFlowAPIKey"];
	        this.emApiKey = source["emApiKey"];
	        this.windowWidth = source["windowWidth"];
	        this.windowHeight = source["windowHeight"];
	        this.promptPlazaApiBase = source["promptPlazaApiBase"];
	        this.backtestAccuracyThreshold = source["backtestAccuracyThreshold"];
	        this.backtestVerifyDays = source["backtestVerifyDays"];
	        this.backtestEnabled = source["backtestEnabled"];
	        this.akshareServiceUrl = source["akshareServiceUrl"];
	        this.aDataServiceUrl = source["aDataServiceUrl"];
	        this.defaultDataSource = source["defaultDataSource"];
	        this.embeddingEnabled = source["embeddingEnabled"];
	        this.embeddingModelName = source["embeddingModelName"];
	        this.embeddingBaseUrl = source["embeddingBaseUrl"];
	        this.embeddingApiKey = source["embeddingApiKey"];
	        this.mcpTimeout = source["mcpTimeout"];
	        this.toolRunBudget = source["toolRunBudget"];
	        this.toolDailyBudget = source["toolDailyBudget"];
	        this.toolDailyCostUSD = source["toolDailyCostUSD"];
	        this.aiConfigs = this.convertValues(source["aiConfigs"], AIConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StockBasic {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    ts_code: string;
	    symbol: string;
	    name: string;
	    area: string;
	    industry: string;
	    fullname: string;
	    enname: string;
	    cnspell: string;
	    market: string;
	    exchange: string;
	    curr_type: string;
	    list_status: string;
	    list_date: string;
	    delist_date: string;
	    is_hs: string;
	    act_name: string;
	    act_ent_type: string;
	    bk_name: string;
	    bk_code: string;
	
	    static createFrom(source: any = {}) {
	        return new StockBasic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.ts_code = source["ts_code"];
	        this.symbol = source["symbol"];
	        this.name = source["name"];
	        this.area = source["area"];
	        this.industry = source["industry"];
	        this.fullname = source["fullname"];
	        this.enname = source["enname"];
	        this.cnspell = source["cnspell"];
	        this.market = source["market"];
	        this.exchange = source["exchange"];
	        this.curr_type = source["curr_type"];
	        this.list_status = source["list_status"];
	        this.list_date = source["list_date"];
	        this.delist_date = source["delist_date"];
	        this.is_hs = source["is_hs"];
	        this.act_name = source["act_name"];
	        this.act_ent_type = source["act_ent_type"];
	        this.bk_name = source["bk_name"];
	        this.bk_code = source["bk_code"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StockChangeItem {
	    time: string;
	    code: string;
	    name: string;
	    market: number;
	    changeType: number;
	    typeName: string;
	    volume: number;
	    price: number;
	    changeRate: number;
	    amount: number;
	    industry: string;
	    concept: string;
	
	    static createFrom(source: any = {}) {
	        return new StockChangeItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.code = source["code"];
	        this.name = source["name"];
	        this.market = source["market"];
	        this.changeType = source["changeType"];
	        this.typeName = source["typeName"];
	        this.volume = source["volume"];
	        this.price = source["price"];
	        this.changeRate = source["changeRate"];
	        this.amount = source["amount"];
	        this.industry = source["industry"];
	        this.concept = source["concept"];
	    }
	}
	export class StockChangesResponse {
	    totalCount: number;
	    data: StockChangeItem[];
	
	    static createFrom(source: any = {}) {
	        return new StockChangesResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalCount = source["totalCount"];
	        this.data = this.convertValues(source["data"], StockChangeItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StockInfo {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    "日期": string;
	    "时间": string;
	    "股票代码": string;
	    "股票名称": string;
	    "上次当前价格": number;
	    "当前价格": string;
	    "成交的股票数": string;
	    "成交金额": string;
	    "今日开盘价": string;
	    "昨日收盘价": string;
	    "今日最高价": string;
	    "今日最低价": string;
	    "竞买价": string;
	    "竞卖价": string;
	    "买一报价": string;
	    "买一申报": string;
	    "买二报价": string;
	    "买二申报": string;
	    "买三报价": string;
	    "买三申报": string;
	    "买四报价": string;
	    "买四申报": string;
	    "买五报价": string;
	    "买五申报": string;
	    "卖一报价": string;
	    "卖一申报": string;
	    "卖二报价": string;
	    "卖二申报": string;
	    "卖三报价": string;
	    "卖三申报": string;
	    "卖四报价": string;
	    "卖四申报": string;
	    "卖五报价": string;
	    "卖五申报": string;
	    "市场": string;
	    "盘前盘后": string;
	    "盘前盘后涨跌幅": string;
	    changePercent: number;
	    changePrice: number;
	    highRate: number;
	    lowRate: number;
	    costPrice: number;
	    costVolume: number;
	    profit: number;
	    profitAmount: number;
	    profitAmountToday: number;
	    sort: number;
	    alarmChangePercent: number;
	    alarmPrice: number;
	    Groups: GroupStock[];
	
	    static createFrom(source: any = {}) {
	        return new StockInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this["日期"] = source["日期"];
	        this["时间"] = source["时间"];
	        this["股票代码"] = source["股票代码"];
	        this["股票名称"] = source["股票名称"];
	        this["上次当前价格"] = source["上次当前价格"];
	        this["当前价格"] = source["当前价格"];
	        this["成交的股票数"] = source["成交的股票数"];
	        this["成交金额"] = source["成交金额"];
	        this["今日开盘价"] = source["今日开盘价"];
	        this["昨日收盘价"] = source["昨日收盘价"];
	        this["今日最高价"] = source["今日最高价"];
	        this["今日最低价"] = source["今日最低价"];
	        this["竞买价"] = source["竞买价"];
	        this["竞卖价"] = source["竞卖价"];
	        this["买一报价"] = source["买一报价"];
	        this["买一申报"] = source["买一申报"];
	        this["买二报价"] = source["买二报价"];
	        this["买二申报"] = source["买二申报"];
	        this["买三报价"] = source["买三报价"];
	        this["买三申报"] = source["买三申报"];
	        this["买四报价"] = source["买四报价"];
	        this["买四申报"] = source["买四申报"];
	        this["买五报价"] = source["买五报价"];
	        this["买五申报"] = source["买五申报"];
	        this["卖一报价"] = source["卖一报价"];
	        this["卖一申报"] = source["卖一申报"];
	        this["卖二报价"] = source["卖二报价"];
	        this["卖二申报"] = source["卖二申报"];
	        this["卖三报价"] = source["卖三报价"];
	        this["卖三申报"] = source["卖三申报"];
	        this["卖四报价"] = source["卖四报价"];
	        this["卖四申报"] = source["卖四申报"];
	        this["卖五报价"] = source["卖五报价"];
	        this["卖五申报"] = source["卖五申报"];
	        this["市场"] = source["市场"];
	        this["盘前盘后"] = source["盘前盘后"];
	        this["盘前盘后涨跌幅"] = source["盘前盘后涨跌幅"];
	        this.changePercent = source["changePercent"];
	        this.changePrice = source["changePrice"];
	        this.highRate = source["highRate"];
	        this.lowRate = source["lowRate"];
	        this.costPrice = source["costPrice"];
	        this.costVolume = source["costVolume"];
	        this.profit = source["profit"];
	        this.profitAmount = source["profitAmount"];
	        this.profitAmountToday = source["profitAmountToday"];
	        this.sort = source["sort"];
	        this.alarmChangePercent = source["alarmChangePercent"];
	        this.alarmPrice = source["alarmPrice"];
	        this.Groups = this.convertValues(source["Groups"], GroupStock);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TdxFinanceInfo {
	    market: number;
	    code: string;
	    floatShares: number;
	    totalShares: number;
	    eps: number;
	    totalAssets: number;
	    currentAssets: number;
	    fixedAssets: number;
	    intangibleAssets: number;
	    shareholderCount: number;
	    currentLiabilities: number;
	    longTermLiabilities: number;
	    capitalReserve: number;
	    totalEquity: number;
	    operatingRevenue: number;
	    operatingCost: number;
	    accountsReceivable: number;
	    operatingProfit: number;
	    investmentIncome: number;
	    netCashFlow: number;
	    inventory: number;
	    totalProfit: number;
	    afterTaxProfit: number;
	    netProfit: number;
	    undistributedProfit: number;
	    netAssetsPerShare: number;
	    ipoDate: string;
	    updatedDate: string;
	
	    static createFrom(source: any = {}) {
	        return new TdxFinanceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.market = source["market"];
	        this.code = source["code"];
	        this.floatShares = source["floatShares"];
	        this.totalShares = source["totalShares"];
	        this.eps = source["eps"];
	        this.totalAssets = source["totalAssets"];
	        this.currentAssets = source["currentAssets"];
	        this.fixedAssets = source["fixedAssets"];
	        this.intangibleAssets = source["intangibleAssets"];
	        this.shareholderCount = source["shareholderCount"];
	        this.currentLiabilities = source["currentLiabilities"];
	        this.longTermLiabilities = source["longTermLiabilities"];
	        this.capitalReserve = source["capitalReserve"];
	        this.totalEquity = source["totalEquity"];
	        this.operatingRevenue = source["operatingRevenue"];
	        this.operatingCost = source["operatingCost"];
	        this.accountsReceivable = source["accountsReceivable"];
	        this.operatingProfit = source["operatingProfit"];
	        this.investmentIncome = source["investmentIncome"];
	        this.netCashFlow = source["netCashFlow"];
	        this.inventory = source["inventory"];
	        this.totalProfit = source["totalProfit"];
	        this.afterTaxProfit = source["afterTaxProfit"];
	        this.netProfit = source["netProfit"];
	        this.undistributedProfit = source["undistributedProfit"];
	        this.netAssetsPerShare = source["netAssetsPerShare"];
	        this.ipoDate = source["ipoDate"];
	        this.updatedDate = source["updatedDate"];
	    }
	}
	export class TdxXDXRItem {
	    date: string;
	    category: number;
	    name: string;
	    fenhong?: number;
	    peigujia?: number;
	    songzhuangu?: number;
	    peigu?: number;
	    suogu?: number;
	    preFloatShares?: number;
	    preTotalShares?: number;
	    postFloatShares?: number;
	    postTotalShares?: number;
	
	    static createFrom(source: any = {}) {
	        return new TdxXDXRItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.category = source["category"];
	        this.name = source["name"];
	        this.fenhong = source["fenhong"];
	        this.peigujia = source["peigujia"];
	        this.songzhuangu = source["songzhuangu"];
	        this.peigu = source["peigu"];
	        this.suogu = source["suogu"];
	        this.preFloatShares = source["preFloatShares"];
	        this.preTotalShares = source["preTotalShares"];
	        this.postFloatShares = source["postFloatShares"];
	        this.postTotalShares = source["postTotalShares"];
	    }
	}
	export class TdxCompanyInfoSection {
	    name: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new TdxCompanyInfoSection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.content = source["content"];
	    }
	}
	export class TdxCompanyInfoBundle {
	    sections: TdxCompanyInfoSection[];
	    xdxr: TdxXDXRItem[];
	    finance?: TdxFinanceInfo;
	
	    static createFrom(source: any = {}) {
	        return new TdxCompanyInfoBundle(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sections = this.convertValues(source["sections"], TdxCompanyInfoSection);
	        this.xdxr = this.convertValues(source["xdxr"], TdxXDXRItem);
	        this.finance = this.convertValues(source["finance"], TdxFinanceInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class TradingRecord {
	    ID: number;
	    StockCode: string;
	    StockName: string;
	    Direction: string;
	    Price: number;
	    Volume: number;
	    Amount: number;
	    // Go type: time
	    TradingTime: any;
	    Reason: string;
	    StopLossPrice: number;
	    TakeProfitPrice: number;
	    Fee: number;
	    MarketValue: number;
	    Mindset: string;
	    recordedClosePrice: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new TradingRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.StockCode = source["StockCode"];
	        this.StockName = source["StockName"];
	        this.Direction = source["Direction"];
	        this.Price = source["Price"];
	        this.Volume = source["Volume"];
	        this.Amount = source["Amount"];
	        this.TradingTime = this.convertValues(source["TradingTime"], null);
	        this.Reason = source["Reason"];
	        this.StopLossPrice = source["StopLossPrice"];
	        this.TakeProfitPrice = source["TakeProfitPrice"];
	        this.Fee = source["Fee"];
	        this.MarketValue = source["MarketValue"];
	        this.Mindset = source["Mindset"];
	        this.recordedClosePrice = source["recordedClosePrice"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TradingRecordItem {
	    ID: number;
	    StockCode: string;
	    StockName: string;
	    Direction: string;
	    Price: number;
	    Volume: number;
	    Amount: number;
	    // Go type: time
	    TradingTime: any;
	    Reason: string;
	    StopLossPrice: number;
	    TakeProfitPrice: number;
	    Fee: number;
	    MarketValue: number;
	    Mindset: string;
	    recordedClosePrice: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    closePrice: number;
	    profitAmount: number;
	    profitPercent: number;
	
	    static createFrom(source: any = {}) {
	        return new TradingRecordItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.StockCode = source["StockCode"];
	        this.StockName = source["StockName"];
	        this.Direction = source["Direction"];
	        this.Price = source["Price"];
	        this.Volume = source["Volume"];
	        this.Amount = source["Amount"];
	        this.TradingTime = this.convertValues(source["TradingTime"], null);
	        this.Reason = source["Reason"];
	        this.StopLossPrice = source["StopLossPrice"];
	        this.TakeProfitPrice = source["TakeProfitPrice"];
	        this.Fee = source["Fee"];
	        this.MarketValue = source["MarketValue"];
	        this.Mindset = source["Mindset"];
	        this.recordedClosePrice = source["recordedClosePrice"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.closePrice = source["closePrice"];
	        this.profitAmount = source["profitAmount"];
	        this.profitPercent = source["profitPercent"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TradingRecordListQuery {
	    page: number;
	    pageSize: number;
	    keyword: string;
	    direction: string;
	    startDate: string;
	    endDate: string;
	
	    static createFrom(source: any = {}) {
	        return new TradingRecordListQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.keyword = source["keyword"];
	        this.direction = source["direction"];
	        this.startDate = source["startDate"];
	        this.endDate = source["endDate"];
	    }
	}
	export class TradingRecordPageData {
	    list: TradingRecordItem[];
	    total: number;
	    page: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new TradingRecordPageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], TradingRecordItem);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TradingRecordStatistics {
	    totalBuyAmount: number;
	    totalSellAmount: number;
	    totalProfit: number;
	    profitRate: number;
	    holdingsAmount: number;
	    currentValue: number;
	    stockCount: number;
	
	    static createFrom(source: any = {}) {
	        return new TradingRecordStatistics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalBuyAmount = source["totalBuyAmount"];
	        this.totalSellAmount = source["totalSellAmount"];
	        this.totalProfit = source["totalProfit"];
	        this.profitRate = source["profitRate"];
	        this.holdingsAmount = source["holdingsAmount"];
	        this.currentValue = source["currentValue"];
	        this.stockCount = source["stockCount"];
	    }
	}
	export class TypeCountStats {
	    typeName: string;
	    upCount: number;
	    downCount: number;
	    totalCount: number;
	
	    static createFrom(source: any = {}) {
	        return new TypeCountStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.typeName = source["typeName"];
	        this.upCount = source["upCount"];
	        this.downCount = source["downCount"];
	        this.totalCount = source["totalCount"];
	    }
	}

}

export namespace lo {
	
	export class Tuple2_string_string_ {
	    A: string;
	    B: string;
	
	    static createFrom(source: any = {}) {
	        return new Tuple2_string_string_(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.A = source["A"];
	        this.B = source["B"];
	    }
	}

}

export namespace models {
	
	export class AIResponseResult {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    chatId: string;
	    modelName: string;
	    stockCode: string;
	    stockName: string;
	    question: string;
	    content: string;
	    IsDel: number;
	
	    static createFrom(source: any = {}) {
	        return new AIResponseResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.chatId = source["chatId"];
	        this.modelName = source["modelName"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.question = source["question"];
	        this.content = source["content"];
	        this.IsDel = source["IsDel"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AIResponseResultPageData {
	    list: AIResponseResult[];
	    total: number;
	    page: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new AIResponseResultPageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], AIResponseResult);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AIResponseResultQuery {
	    page: number;
	    pageSize: number;
	    chatId: string;
	    modelName: string;
	    stockCode: string;
	    stockName: string;
	    question: string;
	    startDate: string;
	    endDate: string;
	
	    static createFrom(source: any = {}) {
	        return new AIResponseResultQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.chatId = source["chatId"];
	        this.modelName = source["modelName"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.question = source["question"];
	        this.startDate = source["startDate"];
	        this.endDate = source["endDate"];
	    }
	}
	export class AiAssistantMessage {
	    role: string;
	    content: string;
	    reasoning: string;
	    time: string;
	    modelName?: string;
	    toolCalls?: number[];
	    toolResults?: number[];
	    timeline?: number[];
	
	    static createFrom(source: any = {}) {
	        return new AiAssistantMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.content = source["content"];
	        this.reasoning = source["reasoning"];
	        this.time = source["time"];
	        this.modelName = source["modelName"];
	        this.toolCalls = source["toolCalls"];
	        this.toolResults = source["toolResults"];
	        this.timeline = source["timeline"];
	    }
	}
	export class AiAssistantSessionResp {
	    messages: AiAssistantMessage[];
	    sessionId: string;
	
	    static createFrom(source: any = {}) {
	        return new AiAssistantSessionResp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.messages = this.convertValues(source["messages"], AiAssistantMessage);
	        this.sessionId = source["sessionId"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AiRecommendStocks {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    // Go type: time
	    dataTime?: any;
	    modelName: string;
	    rating: string;
	    stockCode: string;
	    stockName: string;
	    bkCode: string;
	    bkName: string;
	    stockPrice: string;
	    stockCurrentPrice: string;
	    stockCurrentPriceTime: string;
	    stockClosePrice: string;
	    stockPrePrice: string;
	    recommendReason: string;
	    recommendBuyPrice: string;
	    recommendBuyPriceMin: number;
	    recommendBuyPriceMax: number;
	    recommendStopProfitPrice: string;
	    recommendStopProfitPriceMin: number;
	    recommendStopProfitPriceMax: number;
	    recommendStopLossPrice: string;
	    riskRemarks: string;
	    remarks: string;
	    enableAlert: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AiRecommendStocks(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.dataTime = this.convertValues(source["dataTime"], null);
	        this.modelName = source["modelName"];
	        this.rating = source["rating"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.bkCode = source["bkCode"];
	        this.bkName = source["bkName"];
	        this.stockPrice = source["stockPrice"];
	        this.stockCurrentPrice = source["stockCurrentPrice"];
	        this.stockCurrentPriceTime = source["stockCurrentPriceTime"];
	        this.stockClosePrice = source["stockClosePrice"];
	        this.stockPrePrice = source["stockPrePrice"];
	        this.recommendReason = source["recommendReason"];
	        this.recommendBuyPrice = source["recommendBuyPrice"];
	        this.recommendBuyPriceMin = source["recommendBuyPriceMin"];
	        this.recommendBuyPriceMax = source["recommendBuyPriceMax"];
	        this.recommendStopProfitPrice = source["recommendStopProfitPrice"];
	        this.recommendStopProfitPriceMin = source["recommendStopProfitPriceMin"];
	        this.recommendStopProfitPriceMax = source["recommendStopProfitPriceMax"];
	        this.recommendStopLossPrice = source["recommendStopLossPrice"];
	        this.riskRemarks = source["riskRemarks"];
	        this.remarks = source["remarks"];
	        this.enableAlert = source["enableAlert"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AiRecommendStocksPageData {
	    list: AiRecommendStocks[];
	    total: number;
	    page: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new AiRecommendStocksPageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], AiRecommendStocks);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AiRecommendStocksQuery {
	    page: number;
	    pageSize: number;
	    modelName: string;
	    stockCode: string;
	    stockName: string;
	    bkCode: string;
	    bkName: string;
	    startDate: string;
	    endDate: string;
	    enableAlert?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AiRecommendStocksQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.modelName = source["modelName"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.bkCode = source["bkCode"];
	        this.bkName = source["bkName"];
	        this.startDate = source["startDate"];
	        this.endDate = source["endDate"];
	        this.enableAlert = source["enableAlert"];
	    }
	}
	export class AllStockInfo {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    SECUCODE: string;
	    SECURITY_CODE: string;
	    SECURITY_NAME_ABBR: string;
	    NEW_PRICE: string;
	    CHANGE_RATE: string;
	    VOLUME_RATIO: string;
	    HIGH_PRICE: string;
	    LOW_PRICE: string;
	    PRE_CLOSE_PRICE: string;
	    VOLUME: string;
	    DEAL_AMOUNT: string;
	    TURNOVERRATE: string;
	    MARKET: string;
	    CONCEPT: string;
	    INDUSTRY: string;
	    MAX_TRADE_DATE: string;
	
	    static createFrom(source: any = {}) {
	        return new AllStockInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.SECUCODE = source["SECUCODE"];
	        this.SECURITY_CODE = source["SECURITY_CODE"];
	        this.SECURITY_NAME_ABBR = source["SECURITY_NAME_ABBR"];
	        this.NEW_PRICE = source["NEW_PRICE"];
	        this.CHANGE_RATE = source["CHANGE_RATE"];
	        this.VOLUME_RATIO = source["VOLUME_RATIO"];
	        this.HIGH_PRICE = source["HIGH_PRICE"];
	        this.LOW_PRICE = source["LOW_PRICE"];
	        this.PRE_CLOSE_PRICE = source["PRE_CLOSE_PRICE"];
	        this.VOLUME = source["VOLUME"];
	        this.DEAL_AMOUNT = source["DEAL_AMOUNT"];
	        this.TURNOVERRATE = source["TURNOVERRATE"];
	        this.MARKET = source["MARKET"];
	        this.CONCEPT = source["CONCEPT"];
	        this.INDUSTRY = source["INDUSTRY"];
	        this.MAX_TRADE_DATE = source["MAX_TRADE_DATE"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AllStocksResp {
	    version: any;
	    // Go type: struct { Nextpage bool "json:\"nextpage\""; Currentpage int "json:\"currentpage\""; Data []models
	    result: any;
	    success: boolean;
	    message: string;
	    code: number;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new AllStocksResp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.result = this.convertValues(source["result"], Object);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.code = source["code"];
	        this.url = source["url"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AuditLog {
	    id: number;
	    // Go type: time
	    createdAt: any;
	    userId?: string;
	    agentId?: string;
	    skillId?: number;
	    toolName: string;
	    serverName?: string;
	    requestBody: string;
	    response: string;
	    success: boolean;
	    errorMsg?: string;
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new AuditLog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.userId = source["userId"];
	        this.agentId = source["agentId"];
	        this.skillId = source["skillId"];
	        this.toolName = source["toolName"];
	        this.serverName = source["serverName"];
	        this.requestBody = source["requestBody"];
	        this.response = source["response"];
	        this.success = source["success"];
	        this.errorMsg = source["errorMsg"];
	        this.durationMs = source["durationMs"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AuditLogPageData {
	    list: AuditLog[];
	    total: number;
	    page: number;
	    pageSize: number;
	
	    static createFrom(source: any = {}) {
	        return new AuditLogPageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], AuditLog);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AuditLogPageResp {
	    code: number;
	    message: string;
	    data: AuditLogPageData;
	
	    static createFrom(source: any = {}) {
	        return new AuditLogPageResp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.message = source["message"];
	        this.data = this.convertValues(source["data"], AuditLogPageData);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AuditLogQuery {
	    page: number;
	    pageSize: number;
	    userId?: string;
	    agentId?: string;
	    skillId?: number;
	    toolName?: string;
	    serverName?: string;
	    success?: boolean;
	    startDate?: string;
	    endDate?: string;
	    keyword?: string;
	
	    static createFrom(source: any = {}) {
	        return new AuditLogQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.userId = source["userId"];
	        this.agentId = source["agentId"];
	        this.skillId = source["skillId"];
	        this.toolName = source["toolName"];
	        this.serverName = source["serverName"];
	        this.success = source["success"];
	        this.startDate = source["startDate"];
	        this.endDate = source["endDate"];
	        this.keyword = source["keyword"];
	    }
	}
	export class BKFundFlow {
	    id: number;
	    code: string;
	    name: string;
	    netInflow: number;
	    snapTime: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new BKFundFlow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.code = source["code"];
	        this.name = source["name"];
	        this.netInflow = source["netInflow"];
	        this.snapTime = source["snapTime"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BKFundFlowPoint {
	    snapTime: string;
	    netInflow: number;
	
	    static createFrom(source: any = {}) {
	        return new BKFundFlowPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.snapTime = source["snapTime"];
	        this.netInflow = source["netInflow"];
	    }
	}
	export class ConceptFundFlow {
	    id: number;
	    code: string;
	    name: string;
	    netInflow: number;
	    snapTime: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new ConceptFundFlow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.code = source["code"];
	        this.name = source["name"];
	        this.netInflow = source["netInflow"];
	        this.snapTime = source["snapTime"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ConceptFundFlowPoint {
	    snapTime: string;
	    netInflow: number;
	
	    static createFrom(source: any = {}) {
	        return new ConceptFundFlowPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.snapTime = source["snapTime"];
	        this.netInflow = source["netInflow"];
	    }
	}
	export class CronTask {
	    id: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    name: string;
	    cronExpr: string;
	    taskType: string;
	    target: string;
	    params: string;
	    enable: boolean;
	    // Go type: time
	    lastRunAt?: any;
	    // Go type: time
	    nextRunAt?: any;
	    runCount: number;
	    status: string;
	    description: string;
	    lastRunResult: string;
	
	    static createFrom(source: any = {}) {
	        return new CronTask(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.name = source["name"];
	        this.cronExpr = source["cronExpr"];
	        this.taskType = source["taskType"];
	        this.target = source["target"];
	        this.params = source["params"];
	        this.enable = source["enable"];
	        this.lastRunAt = this.convertValues(source["lastRunAt"], null);
	        this.nextRunAt = this.convertValues(source["nextRunAt"], null);
	        this.runCount = source["runCount"];
	        this.status = source["status"];
	        this.description = source["description"];
	        this.lastRunResult = source["lastRunResult"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CronTaskPageResp {
	    total: number;
	    data: CronTask[];
	
	    static createFrom(source: any = {}) {
	        return new CronTaskPageResp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.data = this.convertValues(source["data"], CronTask);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CronTaskQuery {
	    page: number;
	    pageSize: number;
	    name: string;
	    taskType: string;
	    status: string;
	    enable?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CronTaskQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.name = source["name"];
	        this.taskType = source["taskType"];
	        this.status = source["status"];
	        this.enable = source["enable"];
	    }
	}
	export class CustomStrategy {
	    id: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    name: string;
	    query: string;
	    description: string;
	    sortOrder: number;
	
	    static createFrom(source: any = {}) {
	        return new CustomStrategy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.name = source["name"];
	        this.query = source["query"];
	        this.description = source["description"];
	        this.sortOrder = source["sortOrder"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CustomStrategyPageData {
	    list: CustomStrategy[];
	    total: number;
	    page: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new CustomStrategyPageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], CustomStrategy);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CustomStrategyQuery {
	    page: number;
	    pageSize: number;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new CustomStrategyQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.name = source["name"];
	    }
	}
	export class MCPServer {
	    id: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    name: string;
	    description: string;
	    url: string;
	    type: string;
	    headers: string;
	    command: string;
	    args: string;
	    env: string;
	    enable: boolean;
	    status: string;
	    testResult: string;
	
	    static createFrom(source: any = {}) {
	        return new MCPServer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.url = source["url"];
	        this.type = source["type"];
	        this.headers = source["headers"];
	        this.command = source["command"];
	        this.args = source["args"];
	        this.env = source["env"];
	        this.enable = source["enable"];
	        this.status = source["status"];
	        this.testResult = source["testResult"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MCPServerPageResp {
	    total: number;
	    data: MCPServer[];
	
	    static createFrom(source: any = {}) {
	        return new MCPServerPageResp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.data = this.convertValues(source["data"], MCPServer);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MCPServerQuery {
	    page: number;
	    pageSize: number;
	    name: string;
	    status: string;
	    enable?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MCPServerQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.enable = source["enable"];
	    }
	}
	export class MCPServerTool {
	    id: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    mcpServerId: number;
	    toolName: string;
	    description: string;
	    paramsSchema: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MCPServerTool(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.mcpServerId = source["mcpServerId"];
	        this.toolName = source["toolName"];
	        this.description = source["description"];
	        this.paramsSchema = source["paramsSchema"];
	        this.enabled = source["enabled"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MarketStatistic {
	    id: number;
	    dataDate: string;
	    dataTime: string;
	    upCount: number;
	    downCount: number;
	    upRatio: number;
	    upDownRatio: number;
	    sentimentDesc: string;
	    limitUp: number;
	    limitDown: number;
	    limitRatio: number;
	    shUpCount: number;
	    shDownCount: number;
	    szUpCount: number;
	    szDownCount: number;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new MarketStatistic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.dataDate = source["dataDate"];
	        this.dataTime = source["dataTime"];
	        this.upCount = source["upCount"];
	        this.downCount = source["downCount"];
	        this.upRatio = source["upRatio"];
	        this.upDownRatio = source["upDownRatio"];
	        this.sentimentDesc = source["sentimentDesc"];
	        this.limitUp = source["limitUp"];
	        this.limitDown = source["limitDown"];
	        this.limitRatio = source["limitRatio"];
	        this.shUpCount = source["shUpCount"];
	        this.shDownCount = source["shDownCount"];
	        this.szUpCount = source["szUpCount"];
	        this.szDownCount = source["szDownCount"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SentimentResult {
	    Score: number;
	    Category: number;
	    PositiveCount: number;
	    NegativeCount: number;
	    Description: string;
	
	    static createFrom(source: any = {}) {
	        return new SentimentResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Score = source["Score"];
	        this.Category = source["Category"];
	        this.PositiveCount = source["PositiveCount"];
	        this.NegativeCount = source["NegativeCount"];
	        this.Description = source["Description"];
	    }
	}
	export class Skill {
	    id: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    name: string;
	    description: string;
	    category: string;
	    systemPrompt: string;
	    directive: string;
	    examples: string;
	    triggerKeywords: string;
	    synonyms: string;
	    toolGroups: string;
	    mcpServerIds: string;
	    enable: boolean;
	    sortOrder: number;
	    sandbox: boolean;
	    sandbox_percentage: number;
	    version: string;
	    status: string;
	    inputSchema: string;
	    outputSchema: string;
	
	    static createFrom(source: any = {}) {
	        return new Skill(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.category = source["category"];
	        this.systemPrompt = source["systemPrompt"];
	        this.directive = source["directive"];
	        this.examples = source["examples"];
	        this.triggerKeywords = source["triggerKeywords"];
	        this.synonyms = source["synonyms"];
	        this.toolGroups = source["toolGroups"];
	        this.mcpServerIds = source["mcpServerIds"];
	        this.enable = source["enable"];
	        this.sortOrder = source["sortOrder"];
	        this.sandbox = source["sandbox"];
	        this.sandbox_percentage = source["sandbox_percentage"];
	        this.version = source["version"];
	        this.status = source["status"];
	        this.inputSchema = source["inputSchema"];
	        this.outputSchema = source["outputSchema"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SkillPageResp {
	    total: number;
	    data: Skill[];
	
	    static createFrom(source: any = {}) {
	        return new SkillPageResp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.data = this.convertValues(source["data"], Skill);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SkillQuery {
	    page: number;
	    pageSize: number;
	    name: string;
	    category: string;
	    enable?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SkillQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.enable = source["enable"];
	    }
	}
	export class StockChangeHistory {
	    id: number;
	    changeTime: string;
	    changeDate: string;
	    stockCode: string;
	    stockName: string;
	    market: number;
	    changeType: number;
	    typeName: string;
	    volume: number;
	    price: number;
	    changeRate: number;
	    amount: number;
	    industry: string;
	    concept: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new StockChangeHistory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.changeTime = source["changeTime"];
	        this.changeDate = source["changeDate"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.market = source["market"];
	        this.changeType = source["changeType"];
	        this.typeName = source["typeName"];
	        this.volume = source["volume"];
	        this.price = source["price"];
	        this.changeRate = source["changeRate"];
	        this.amount = source["amount"];
	        this.industry = source["industry"];
	        this.concept = source["concept"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StockChangeHistoryPageData {
	    list: StockChangeHistory[];
	    total: number;
	    page: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new StockChangeHistoryPageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], StockChangeHistory);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StockChangeHistoryQuery {
	    stockCode: string;
	    stockName: string;
	    changeType: number;
	    changeTypes: number[];
	    typeName: string;
	    startDate: string;
	    endDate: string;
	    startTime: string;
	    endTime: string;
	    minVolume: number;
	    minAmount: number;
	    minChangeRate: number;
	    maxChangeRate: number;
	    industry: string;
	    concept: string;
	    page: number;
	    pageSize: number;
	
	    static createFrom(source: any = {}) {
	        return new StockChangeHistoryQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.changeType = source["changeType"];
	        this.changeTypes = source["changeTypes"];
	        this.typeName = source["typeName"];
	        this.startDate = source["startDate"];
	        this.endDate = source["endDate"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	        this.minVolume = source["minVolume"];
	        this.minAmount = source["minAmount"];
	        this.minChangeRate = source["minChangeRate"];
	        this.maxChangeRate = source["maxChangeRate"];
	        this.industry = source["industry"];
	        this.concept = source["concept"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	    }
	}
	export class StockInfo {
	    SECUCODE: string;
	    SECURITY_CODE: string;
	    SECURITY_NAME_ABBR: string;
	    NEW_PRICE: any;
	    CHANGE_RATE: any;
	    VOLUME_RATIO: any;
	    HIGH_PRICE: any;
	    LOW_PRICE: any;
	    PRE_CLOSE_PRICE: any;
	    VOLUME: any;
	    DEAL_AMOUNT: any;
	    TURNOVERRATE: any;
	    MARKET: string;
	    CONCEPT: any;
	    INDUSTRY: string;
	    MAX_TRADE_DATE: string;
	
	    static createFrom(source: any = {}) {
	        return new StockInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.SECUCODE = source["SECUCODE"];
	        this.SECURITY_CODE = source["SECURITY_CODE"];
	        this.SECURITY_NAME_ABBR = source["SECURITY_NAME_ABBR"];
	        this.NEW_PRICE = source["NEW_PRICE"];
	        this.CHANGE_RATE = source["CHANGE_RATE"];
	        this.VOLUME_RATIO = source["VOLUME_RATIO"];
	        this.HIGH_PRICE = source["HIGH_PRICE"];
	        this.LOW_PRICE = source["LOW_PRICE"];
	        this.PRE_CLOSE_PRICE = source["PRE_CLOSE_PRICE"];
	        this.VOLUME = source["VOLUME"];
	        this.DEAL_AMOUNT = source["DEAL_AMOUNT"];
	        this.TURNOVERRATE = source["TURNOVERRATE"];
	        this.MARKET = source["MARKET"];
	        this.CONCEPT = source["CONCEPT"];
	        this.INDUSTRY = source["INDUSTRY"];
	        this.MAX_TRADE_DATE = source["MAX_TRADE_DATE"];
	    }
	}
	export class TechnicalIndicators {
	    MACD_GOLDEN_FORK: boolean;
	    KDJ_GOLDEN_FORK: boolean;
	    BREAK_THROUGH: boolean;
	    LOW_FUNDS_INFLOW: boolean;
	    HIGH_FUNDS_OUTFLOW: boolean;
	    BREAKUP_MA_5DAYS: boolean;
	    LONG_AVG_ARRAY: boolean;
	    SHORT_AVG_ARRAY: boolean;
	    UPPER_LARGE_VOLUME: boolean;
	    DOWN_NARROW_VOLUME: boolean;
	    ONE_DAYANG_LINE: boolean;
	    TWO_DAYANG_LINES: boolean;
	    RISE_SUN: boolean;
	    POWER_FULGUN: boolean;
	    RESTORE_JUSTICE: boolean;
	    DOWN_7DAYS: boolean;
	    UPPER_8DAYS: boolean;
	    UPPER_9DAYS: boolean;
	    UPPER_4DAYS: boolean;
	    HEAVEN_RULE: boolean;
	    UPSIDE_VOLUME: boolean;
	    BEARISH_ENGULFING: boolean;
	    REVERSING_HAMMER: boolean;
	    SHOOTING_STAR: boolean;
	    EVENING_STAR: boolean;
	    FIRST_DAWN: boolean;
	    PREGNANT: boolean;
	    BLACK_CLOUD_TOPS: boolean;
	    MORNING_STAR: boolean;
	    NARROW_FINISH: boolean;
	    UPP_DAYS: number;
	    CONCERN_RANK_7DAYS: number;
	    UPNDAY: number;
	    DOWNNDAY: number;
	
	    static createFrom(source: any = {}) {
	        return new TechnicalIndicators(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.MACD_GOLDEN_FORK = source["MACD_GOLDEN_FORK"];
	        this.KDJ_GOLDEN_FORK = source["KDJ_GOLDEN_FORK"];
	        this.BREAK_THROUGH = source["BREAK_THROUGH"];
	        this.LOW_FUNDS_INFLOW = source["LOW_FUNDS_INFLOW"];
	        this.HIGH_FUNDS_OUTFLOW = source["HIGH_FUNDS_OUTFLOW"];
	        this.BREAKUP_MA_5DAYS = source["BREAKUP_MA_5DAYS"];
	        this.LONG_AVG_ARRAY = source["LONG_AVG_ARRAY"];
	        this.SHORT_AVG_ARRAY = source["SHORT_AVG_ARRAY"];
	        this.UPPER_LARGE_VOLUME = source["UPPER_LARGE_VOLUME"];
	        this.DOWN_NARROW_VOLUME = source["DOWN_NARROW_VOLUME"];
	        this.ONE_DAYANG_LINE = source["ONE_DAYANG_LINE"];
	        this.TWO_DAYANG_LINES = source["TWO_DAYANG_LINES"];
	        this.RISE_SUN = source["RISE_SUN"];
	        this.POWER_FULGUN = source["POWER_FULGUN"];
	        this.RESTORE_JUSTICE = source["RESTORE_JUSTICE"];
	        this.DOWN_7DAYS = source["DOWN_7DAYS"];
	        this.UPPER_8DAYS = source["UPPER_8DAYS"];
	        this.UPPER_9DAYS = source["UPPER_9DAYS"];
	        this.UPPER_4DAYS = source["UPPER_4DAYS"];
	        this.HEAVEN_RULE = source["HEAVEN_RULE"];
	        this.UPSIDE_VOLUME = source["UPSIDE_VOLUME"];
	        this.BEARISH_ENGULFING = source["BEARISH_ENGULFING"];
	        this.REVERSING_HAMMER = source["REVERSING_HAMMER"];
	        this.SHOOTING_STAR = source["SHOOTING_STAR"];
	        this.EVENING_STAR = source["EVENING_STAR"];
	        this.FIRST_DAWN = source["FIRST_DAWN"];
	        this.PREGNANT = source["PREGNANT"];
	        this.BLACK_CLOUD_TOPS = source["BLACK_CLOUD_TOPS"];
	        this.MORNING_STAR = source["MORNING_STAR"];
	        this.NARROW_FINISH = source["NARROW_FINISH"];
	        this.UPP_DAYS = source["UPP_DAYS"];
	        this.CONCERN_RANK_7DAYS = source["CONCERN_RANK_7DAYS"];
	        this.UPNDAY = source["UPNDAY"];
	        this.DOWNNDAY = source["DOWNNDAY"];
	    }
	}
	export class VersionInfo {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    version: string;
	    content: string;
	    icon: string;
	    alipay: string;
	    wxpay: string;
	    buildTimeStamp: number;
	    officialStatement: string;
	    IsDel: number;
	
	    static createFrom(source: any = {}) {
	        return new VersionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.version = source["version"];
	        this.content = source["content"];
	        this.icon = source["icon"];
	        this.alipay = source["alipay"];
	        this.wxpay = source["wxpay"];
	        this.buildTimeStamp = source["buildTimeStamp"];
	        this.officialStatement = source["officialStatement"];
	        this.IsDel = source["IsDel"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace quant {
	
	export class DirStats3 {
	    Total: number;
	    Accurate: number;
	    Rate: number;
	
	    static createFrom(source: any = {}) {
	        return new DirStats3(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Total = source["Total"];
	        this.Accurate = source["Accurate"];
	        this.Rate = source["Rate"];
	    }
	}
	export class AIReportBacktestResult {
	    Total: number;
	    Processed: number;
	    Skipped: number;
	    Accurate: number;
	    AccuracyRate: number;
	    DirectionStats: Record<string, DirStats3>;
	    Duration: number;
	    ModelName: string;
	    SkillUsed: string;
	
	    static createFrom(source: any = {}) {
	        return new AIReportBacktestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Total = source["Total"];
	        this.Processed = source["Processed"];
	        this.Skipped = source["Skipped"];
	        this.Accurate = source["Accurate"];
	        this.AccuracyRate = source["AccuracyRate"];
	        this.DirectionStats = this.convertValues(source["DirectionStats"], DirStats3, true);
	        this.Duration = source["Duration"];
	        this.ModelName = source["ModelName"];
	        this.SkillUsed = source["SkillUsed"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class MoneyDimensionResult {
	    score: number;
	    mainNetInflow5D: number;
	    superLargeNetPct: number;
	    largeNetPct: number;
	    recentInflowTrend: string;
	    chipConcentration: string;
	    evaluation: string;
	
	    static createFrom(source: any = {}) {
	        return new MoneyDimensionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.score = source["score"];
	        this.mainNetInflow5D = source["mainNetInflow5D"];
	        this.superLargeNetPct = source["superLargeNetPct"];
	        this.largeNetPct = source["largeNetPct"];
	        this.recentInflowTrend = source["recentInflowTrend"];
	        this.chipConcentration = source["chipConcentration"];
	        this.evaluation = source["evaluation"];
	    }
	}
	export class QuantDimensionResult {
	    score: number;
	    strategyName: string;
	    winRatePct: number;
	    totalReturnPct: number;
	    sharpeRatio: number;
	    maxDrawdownPct: number;
	    profitFactor: number;
	    totalTrades: number;
	    equityCurve: number[];
	    evaluation: string;
	
	    static createFrom(source: any = {}) {
	        return new QuantDimensionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.score = source["score"];
	        this.strategyName = source["strategyName"];
	        this.winRatePct = source["winRatePct"];
	        this.totalReturnPct = source["totalReturnPct"];
	        this.sharpeRatio = source["sharpeRatio"];
	        this.maxDrawdownPct = source["maxDrawdownPct"];
	        this.profitFactor = source["profitFactor"];
	        this.totalTrades = source["totalTrades"];
	        this.equityCurve = source["equityCurve"];
	        this.evaluation = source["evaluation"];
	    }
	}
	export class SkillABTestResult {
	    SkillID1: number;
	    SkillID2: number;
	    Accuracy1: number;
	    Accuracy2: number;
	    TotalTested: number;
	    Details: Record<number, number>;
	
	    static createFrom(source: any = {}) {
	        return new SkillABTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.SkillID1 = source["SkillID1"];
	        this.SkillID2 = source["SkillID2"];
	        this.Accuracy1 = source["Accuracy1"];
	        this.Accuracy2 = source["Accuracy2"];
	        this.TotalTested = source["TotalTested"];
	        this.Details = source["Details"];
	    }
	}
	export class TechnicalDimensionResult {
	    score: number;
	    trend: string;
	    ma5: number;
	    ma20: number;
	    ma60: number;
	    atr: number;
	    volumeRatio: number;
	    priceDistanceMA20: number;
	    evaluation: string;
	
	    static createFrom(source: any = {}) {
	        return new TechnicalDimensionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.score = source["score"];
	        this.trend = source["trend"];
	        this.ma5 = source["ma5"];
	        this.ma20 = source["ma20"];
	        this.ma60 = source["ma60"];
	        this.atr = source["atr"];
	        this.volumeRatio = source["volumeRatio"];
	        this.priceDistanceMA20 = source["priceDistanceMA20"];
	        this.evaluation = source["evaluation"];
	    }
	}
	export class StockDiagnosisResult {
	    stockCode: string;
	    stockName: string;
	    overallScore: number;
	    suggestion: string;
	    confidence: number;
	    currentPrice: number;
	    targetPrice: number;
	    stopLossPrice: number;
	    riskRewardRatio: number;
	    quantDim: QuantDimensionResult;
	    moneyDim: MoneyDimensionResult;
	    technicalDim: TechnicalDimensionResult;
	    summary: string;
	    risks: string[];
	    historyTrackRecord: string;
	    lastUpdated: string;
	
	    static createFrom(source: any = {}) {
	        return new StockDiagnosisResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.overallScore = source["overallScore"];
	        this.suggestion = source["suggestion"];
	        this.confidence = source["confidence"];
	        this.currentPrice = source["currentPrice"];
	        this.targetPrice = source["targetPrice"];
	        this.stopLossPrice = source["stopLossPrice"];
	        this.riskRewardRatio = source["riskRewardRatio"];
	        this.quantDim = this.convertValues(source["quantDim"], QuantDimensionResult);
	        this.moneyDim = this.convertValues(source["moneyDim"], MoneyDimensionResult);
	        this.technicalDim = this.convertValues(source["technicalDim"], TechnicalDimensionResult);
	        this.summary = source["summary"];
	        this.risks = source["risks"];
	        this.historyTrackRecord = source["historyTrackRecord"];
	        this.lastUpdated = source["lastUpdated"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace services {
	
	export class AiModelInfo {
	    modelName: string;
	    maxTokens: number;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new AiModelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modelName = source["modelName"];
	        this.maxTokens = source["maxTokens"];
	        this.source = source["source"];
	    }
	}
	export class PromptTemplateItem {
	    id: number;
	    name: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new PromptTemplateItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	    }
	}

}

export namespace types {
	
	export class AgentAction {
	    type: string;
	    payload: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new AgentAction(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.payload = source["payload"];
	    }
	}

}

