package tree

import (
	"fmt"
	"math/bits"
)

// SegmentTree 是一个为加权随机抽样优化的、非标准的线段树。
// 它支持高效的单点更新和基于前缀和的随机位置查找。
type SegmentTree struct {
	tree         []float64 // 存储树的节点，大小为 2 * alignedSize
	originalSize int       // 用户请求的原始大小 (N)
	alignedSize  int       // 对齐到2的幂次后的大小
}

// NewSegmentTree 创建一个指定大小的空线段树。
func NewSegmentTree(size int) (*SegmentTree, error) {
	if size <= 0 {
		return nil, fmt.Errorf("树的大小必须为正数")
	}
	alignedSize := 1 << bits.Len(uint(size))
	return &SegmentTree{
		tree:         make([]float64, 2*alignedSize),
		originalSize: size,
		alignedSize:  alignedSize,
	}, nil
}

// Rebuild 从一个给定的权重数组重建树。
// 数组长度必须与树的原始大小匹配。
func (st *SegmentTree) Rebuild(weights []float64) error {
	if len(weights) != st.originalSize {
		return fmt.Errorf("权重数组大小 (%d) 与树的原始大小 (%d) 不匹配", len(weights), st.originalSize)
	}

	// 1. 填充叶子节点
	for i := 0; i < st.originalSize; i++ {
		st.tree[st.alignedSize+i] = weights[i]
	}
	// 将多余的叶子节点清零
	for i := st.originalSize; i < st.alignedSize; i++ {
		st.tree[st.alignedSize+i] = 0
	}

	// 2. 非递归地从下到上构建父节点
	for i := st.alignedSize - 1; i > 0; i-- {
		st.tree[i] = st.tree[2*i] + st.tree[2*i+1]
	}
	return nil
}

// Update 非递归地更新指定索引的权重值。
func (st *SegmentTree) Update(index int, value float64) error {
	if index < 0 || index >= st.originalSize {
		return fmt.Errorf("索引 %d 超出范围 [0, %d)", index, st.originalSize)
	}

	pos := st.alignedSize + index
	st.tree[pos] = value

	// 从下到上更新所有父节点
	for pos > 1 {
		pos /= 2
		st.tree[pos] = st.tree[2*pos] + st.tree[2*pos+1]
	}
	return nil
}

// Query 直接查询指定索引的权重值。
func (st *SegmentTree) Query(index int) (float64, error) {
	if index < 0 || index >= st.originalSize {
		return 0, fmt.Errorf("索引 %d 超出范围 [0, %d)", index, st.originalSize)
	}
	return st.tree[st.alignedSize+index], nil
}

// PrefixSum 查询从索引0到指定索引(包含)的权重总和。
func (st *SegmentTree) PrefixSum(index int) (float64, error) {
	if index < 0 || index >= st.originalSize {
		return 0, fmt.Errorf("索引 %d 超出范围 [0, %d)", index, st.originalSize)
	}

	sum := 0.0
	// l 和 r 是树中节点的索引
	l, r := st.alignedSize, st.alignedSize+index+1
	for l < r {
		if l&1 == 1 { // 如果l是右孩子
			sum += st.tree[l]
			l++
		}
		if r&1 == 1 { // 如果r是右孩子
			r--
			sum += st.tree[r]
		}
		l /= 2
		r /= 2
	}
	return sum, nil
}

// Find 查找第一个其前缀和大于等于给定值的索引。
// 用于加权随机抽样。
func (st *SegmentTree) Find(value float64) (int, error) {
	totalSum := st.tree[1]
	if value < 0 || value > totalSum {
		return -1, fmt.Errorf("查找值 %f 超出总权重范围 [0, %f]", value, totalSum)
	}

	pos := 1
	for pos < st.alignedSize { // 只要还没到叶子层
		leftChild := 2 * pos
		rightChild := 2*pos + 1

		if value <= st.tree[leftChild] {
			// 如果随机值小于等于左子树的总和，则进入左子树
			pos = leftChild
		} else {
			// 否则，减去左子树的权重，然后进入右子树
			value -= st.tree[leftChild]
			pos = rightChild
		}
	}
	return pos - st.alignedSize, nil
}

// TotalSum 返回所有权重的总和。
func (st *SegmentTree) TotalSum() float64 {
	return st.tree[1]
}
