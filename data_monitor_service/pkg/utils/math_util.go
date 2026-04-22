package utils

import (
	"math"
)

// LogNormalize 对数归一化 (适用于幂律分布的绝对值指标，如播放量、粉丝量)
// 将长尾数据平滑映射到 0-100 区间
func LogNormalize(value float32, benchmark float32) float32 {
	if value <= 0 {
		return 0
	}
	// 加 1 防止 Log(0) 异常
	score := (math.Log10(float64(value)+1) / math.Log10(float64(benchmark)+1)) * 100
	if score > 100 {
		return 100 // 封顶 100 分
	}
	// 保留一位小数
	return float32(math.Round(score*10) / 10)
}

// LinearNormalize 线性归一化 (适用于本身已经是比率的相对指标，如互动率、完播率)
func LinearNormalize(value float32, benchmark float32) float32 {
	if value <= 0 {
		return 0
	}
	score := (float64(value) / float64(benchmark)) * 100
	if score > 100 {
		return 100
	}
	return float32(math.Round(score*10) / 10)
}
