package grading

import (
	"fmt"
	"sort"
)

// ObservationType 低俗观察项类型
const (
	ObsBodyPartCloseup    = "body_part_closeup"    // 与场景无关的部位特写
	ObsSwimwearOffContext = "swimwear_off_context" // 与活动无关的泳装内衣出镜
	ObsSexualGesture      = "sexual_gesture"       // 性暗示动作
	ObsSexualSound        = "sexual_sound"         // 带性意味的呻吟或喘息声
	ObsSexualAct          = "sexual_act"           // 性行为或模仿性行为
	ObsPrivatePart        = "private_part"         // 无正当用途的私密部位展示
)

// Observation 一个低俗观察项
type Observation struct {
	Type      string  `json:"type"`
	Timestamp float64 `json:"timestamp"` // 秒
}

// Flags 额外标记
type Flags struct {
	IsSwimwearProduct bool `json:"is_swimwear_product"` // 内衣泳装商品展示
	HasSuspectedMinor bool `json:"has_suspected_minor"` // 疑似未成年人
}

// VulgarityLevel 低俗等级
type VulgarityLevel string

const (
	L0 VulgarityLevel = "L0"
	L1 VulgarityLevel = "L1"
	L2 VulgarityLevel = "L2"
	L3 VulgarityLevel = "L3"
)

// ComputeLevel 根据观察项和标记计算低俗等级
func ComputeLevel(observations []Observation, flags Flags) VulgarityLevel {
	if len(observations) == 0 && !flags.HasSuspectedMinor {
		return L0
	}

	// 疑似未成年 + 任何性化表现（部位特写、性暗示动作、性意味声音、性行为、私密部位）→ L3，
	// 标签标准："涉及未成年人的任何性化内容一律 L3"。无关场景的泳装内衣出镜本身不算性化，按常规规则定级。
	if flags.HasSuspectedMinor {
		for _, obs := range observations {
			if obs.Type != ObsSwimwearOffContext {
				return L3
			}
		}
	}

	// 统计各类型出现次数和累计时长
	typeCounts := make(map[string]int)
	typeTimestamps := make(map[string][]float64)

	for _, obs := range observations {
		typeCounts[obs.Type]++
		typeTimestamps[obs.Type] = append(typeTimestamps[obs.Type], obs.Timestamp)
	}

	// L3 判定
	if typeCounts[ObsSexualAct] > 0 || typeCounts[ObsPrivatePart] > 0 {
		return L3
	}

	// 计算特写累计时长（相邻帧间隔估算）
	closeupDuration := estimateDuration(typeTimestamps[ObsBodyPartCloseup])

	// 两种 L2 表现 → L3
	l2Types := 0
	if typeCounts[ObsBodyPartCloseup] >= 2 || closeupDuration >= 3.0 {
		l2Types++
	}
	if typeCounts[ObsSexualGesture] > 0 {
		l2Types++
	}
	if typeCounts[ObsSexualSound] > 0 {
		l2Types++
	}
	if l2Types >= 2 {
		return L3
	}

	// L2 判定
	if typeCounts[ObsBodyPartCloseup] >= 2 || closeupDuration >= 3.0 {
		return L2
	}
	if typeCounts[ObsSexualGesture] > 0 || typeCounts[ObsSexualSound] > 0 {
		return L2
	}

	// 两种 L1 表现 → L2
	l1Types := 0
	if typeCounts[ObsBodyPartCloseup] == 1 {
		l1Types++
	}
	if typeCounts[ObsSwimwearOffContext] > 0 {
		l1Types++
	}
	if l1Types >= 2 {
		return L2
	}

	// L1 判定
	if typeCounts[ObsBodyPartCloseup] > 0 || typeCounts[ObsSwimwearOffContext] > 0 {
		// 内衣泳装商品展示例外
		if flags.IsSwimwearProduct && typeCounts[ObsBodyPartCloseup] == 0 {
			return L0
		}
		return L1
	}

	return L0
}

// estimateDuration 根据时间戳估算累计时长：每个时间点算 2 秒（定时帧间隔），相邻不足 2 秒的只算一次。
func estimateDuration(timestamps []float64) float64 {
	if len(timestamps) == 0 {
		return 0.0
	}
	sort.Float64s(timestamps)

	uniqueDuration := 0.0
	prev := -10.0
	for _, t := range timestamps {
		if t-prev >= 2.0 {
			uniqueDuration += 2.0
			prev = t
		}
	}

	return uniqueDuration
}

// String 返回等级的字符串表示
func (l VulgarityLevel) String() string {
	return string(l)
}

// Validate 验证等级是否合法
func (l VulgarityLevel) Validate() error {
	switch l {
	case L0, L1, L2, L3:
		return nil
	default:
		return fmt.Errorf("invalid vulgarity level: %s", l)
	}
}
