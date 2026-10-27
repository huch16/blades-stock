package models

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"time"
)

// KnowledgeCrystal 表示通过语义向量模型与大模型二次精炼后提取的"高信噪比经验结晶"
// 适用于长期自动运转的云端/桌面系统，避免原始日志无限膨胀，同时让 Agent 随着时间推移积累交易智慧与规律。
type KnowledgeCrystal struct {
	ID             uint      `json:"id" gorm:"primarykey"`
	Category       string    `json:"category" gorm:"index;size:32"` // etf_rotation, stock_resonance, macro_policy, risk_warning, trading_fact
	Title          string    `json:"title" gorm:"size:256"`         // 结晶简报/主题
	TriggerPattern string    `json:"triggerPattern" gorm:"size:256"`// 适用场景或触发模式（如："放量突破"、"降息周期"、"高位放量滞涨"）
	Summary        string    `json:"summary" gorm:"type:text"`      // 语义精炼提炼结论 (100~300 字高密度概括)
	KeyInsights    string    `json:"keyInsights" gorm:"type:text"`  // 关键决策依据与核心因子
	AccuracyScore  float64   `json:"accuracyScore" gorm:"default:0"`// 历史验证胜率/回测验证分 (0~100)
	HitCount       int       `json:"hitCount" gorm:"default:0"`     // 被 Agent 决策召回利用的次数（热度）
	LastHitAt      time.Time `json:"lastHitAt"`                     // 最近一次被召回时间
	// 存储高维稠密语义向量 (Float32 小端二进制存储)
	Embedding      []byte    `json:"-" gorm:"column:embedding"`
	VectorDim      int       `json:"vectorDim" gorm:"default:0"`    // 向量维度 (如 1024, 1536)
	SourceDate     string    `json:"sourceDate" gorm:"index;size:32"`// 原始生成/验证日期 (YYYY-MM-DD)
	SourceRef      string    `json:"sourceRef" gorm:"size:128"`     // 原始数据关联引用标识 (如: ai_resp_123, backtest_456)
	ContentHash    string    `json:"contentHash" gorm:"uniqueIndex;size:64"`// SHA256 防重哈希
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (KnowledgeCrystal) TableName() string {
	return "knowledge_crystals"
}

// ComputeContentHash 生成结晶内容的唯一哈希，防止重复生成相同规律的结晶
func ComputeContentHash(category, title, summary string) string {
	h := sha256.New()
	h.Write([]byte(category + "||" + title + "||" + summary))
	return hex.EncodeToString(h.Sum(nil))
}

// BytesToFloat32sLE converts little-endian []byte to []float32
func BytesToFloat32sLE(b []byte) []float32 {
	if len(b)%4 != 0 {
		return nil
	}
	res := make([]float32, len(b)/4)
	for i := 0; i < len(res); i++ {
		bits := binary.LittleEndian.Uint32(b[i*4 : (i+1)*4])
		res[i] = math.Float32frombits(bits)
	}
	return res
}

// Float32sToBytesLE converts []float32 to little-endian []byte
func Float32sToBytesLE(f []float32) []byte {
	res := make([]byte, len(f)*4)
	for i, v := range f {
		binary.LittleEndian.PutUint32(res[i*4:(i+1)*4], math.Float32bits(v))
	}
	return res
}

// GetVector 将 Embedding 二进制解包为 []float32
func (k *KnowledgeCrystal) GetVector() []float32 {
	return BytesToFloat32sLE(k.Embedding)
}

// SetVector 将 []float32 打包并存入 Embedding 二进制字段
func (k *KnowledgeCrystal) SetVector(vec []float32) {
	k.Embedding = Float32sToBytesLE(vec)
	k.VectorDim = len(vec)
}
