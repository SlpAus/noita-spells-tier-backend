package spell

import (
	"fmt"
	"math"
	"sort"
)

// --- 算法常量 ---
const (
	gaussianP         = 0.2  // 高斯权重分布中的最低权重惩罚 (P)
	mixtureFactorBase = -0.1 // f(M) = base + rate * M/N
	mixtureFactorRate = 0.01
)

// --- 权重函数 ---

// CalculateWeightForTotal 根据法术的总场次数计算其“冷门优先”选择权重。
func CalculateWeightForTotal(total float64) float64 {
	return 1.0 / (total + 5.0)
}

// gaussianMixtureFactor 根据系统总投票数(M)和法术数(N)计算高斯权重和均匀权重的混合比例 f(M)。
func gaussianMixtureFactor(totalVotes float64, spellCount int) float64 {
	if spellCount == 0 {
		return 0.0 // 避免除以零
	}
	// f(M) = -0.1 + 0.01 * M/N
	factor := mixtureFactorBase + mixtureFactorRate*(totalVotes/float64(spellCount))
	// 将结果限制在 [0, 1] 区间内
	return math.Max(0.0, math.Min(1.0, factor))
}

// --- 高斯匹配器 ---

// gaussianMatcher 预计算并存储用于“实力接近”匹配的权重表。
type gaussianMatcher struct {
	weights    []float64
	prefixSum  []float64
	spellCount int
}

var globalGaussianMatcher *gaussianMatcher

// InitializeGaussianMatcher 在应用启动时创建并初始化全局的高斯匹配器。
func InitializeGaussianMatcher(n int) {
	if n <= 1 {
		globalGaussianMatcher = &gaussianMatcher{spellCount: n}
		return
	}
	maxRankDiff := float64(n - 1)
	weights := make([]float64, 2*(n-1))
	prefixSum := make([]float64, 2*(n-1))
	for i := 1; i < n; i++ {
		d := float64(i)
		weight := math.Pow(gaussianP, math.Pow(d/maxRankDiff, 2))
		weights[n+i-2] = weight // d > 0
		weights[n-i-1] = weight // d < 0
	}
	prefixSum[0] = weights[0]
	for i := 1; i < len(weights); i++ {
		prefixSum[i] = prefixSum[i-1] + weights[i]
	}
	globalGaussianMatcher = &gaussianMatcher{
		weights:    weights,
		prefixSum:  prefixSum,
		spellCount: n,
	}
	fmt.Printf("高斯匹配器初始化成功，法术数量: %d\n", n)
}

// GetMixtureFactor 根据系统总投票数(M)计算高斯权重和均匀权重的混合比例 f(M)。
func (gm *gaussianMatcher) GetMixtureFactor(totalVotes float64) float64 {
	return gaussianMixtureFactor(totalVotes, gm.spellCount)
}

// GetMixedWeight 获取指定排名差距的、混合后的实际权重。
func (gm *gaussianMatcher) GetMixedWeight(rankDiff int, mixtureFactor float64) float64 {
	if rankDiff == 0 || gm.spellCount <= 1 {
		return 0
	}
	// 将排名差距转换为数组索引
	index := 0
	if rankDiff > 0 {
		index = gm.spellCount - 1 + rankDiff - 1
	} else { // rankDiff < 0
		index = gm.spellCount - 1 + rankDiff
	}
	if index < 0 || index >= len(gm.weights) {
		return 1 - mixtureFactor // 越界时返回均匀权重
	}
	// 混合权重 = f(M) * P(d) + (1 - f(M)) * 1
	return mixtureFactor*gm.weights[index] + (1 - mixtureFactor)
}

// GetMixedPrefixSum 获取到指定排名差距（包含）的、混合后的实际权重前缀和。
func (gm *gaussianMatcher) GetMixedPrefixSum(rankDiff int, mixtureFactor float64) float64 {
	if rankDiff == 0 || gm.spellCount <= 1 {
		return 0
	}
	index := 0
	if rankDiff > 0 {
		index = gm.spellCount - 1 + rankDiff - 1
	} else {
		index = gm.spellCount - 1 + rankDiff
	}
	if index < 0 {
		return 0
	}
	if index >= len(gm.prefixSum) {
		index = len(gm.prefixSum) - 1
	}
	// 混合前缀和 = f(M) * PrefixSum(P(d)) + (1 - f(M)) * (d_count)
	return mixtureFactor*gm.prefixSum[index] + (1-mixtureFactor)*float64(index+1)
}

// FindRankOffsetByMixedPrefixSum 使用二分查找，根据一个混合前缀和的值，反向计算出对应的排名差距。
func (gm *gaussianMatcher) FindRankOffsetByMixedPrefixSum(mixedPrefixSum float64, mixtureFactor float64) int {
	if gm.spellCount <= 1 {
		return 0
	}
	// sort.Search 在 [0, n) 区间内进行二分查找，找到第一个满足条件的索引 i
	// 我们要找的是第一个混合前缀和 >= 目标值的索引
	index := sort.Search(len(gm.prefixSum), func(i int) bool {
		currentMixedSum := mixtureFactor*gm.prefixSum[i] + (1-mixtureFactor)*float64(i+1)
		return currentMixedSum >= mixedPrefixSum
	})

	// 将数组索引转换为排名差距 (-N+1 ... -1, 1 ... N-1)
	var rankDiff int
	if index < gm.spellCount-1 {
		// 对应负的排名差距
		rankDiff = index - (gm.spellCount - 1)
	} else {
		// 对应正的排名差距
		rankDiff = index - (gm.spellCount - 1) + 1
	}

	return rankDiff
}
