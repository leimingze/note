package grading

import (
	"testing"
)

func TestComputeLevel_L0(t *testing.T) {
	tests := []struct {
		name  string
		obs   []Observation
		flags Flags
	}{
		{
			name:  "无观察项",
			obs:   []Observation{},
			flags: Flags{},
		},
		{
			name:  "内衣泳装商品展示无特写",
			obs:   []Observation{{Type: ObsSwimwearOffContext, Timestamp: 1.0}},
			flags: Flags{IsSwimwearProduct: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := ComputeLevel(tt.obs, tt.flags)
			if level != L0 {
				t.Errorf("expected L0, got %s", level)
			}
		})
	}
}

func TestComputeLevel_L1(t *testing.T) {
	tests := []struct {
		name  string
		obs   []Observation
		flags Flags
	}{
		{
			name:  "部位特写 1 次",
			obs:   []Observation{{Type: ObsBodyPartCloseup, Timestamp: 3.0}},
			flags: Flags{},
		},
		{
			name:  "无关场景泳装",
			obs:   []Observation{{Type: ObsSwimwearOffContext, Timestamp: 5.0}},
			flags: Flags{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := ComputeLevel(tt.obs, tt.flags)
			if level != L1 {
				t.Errorf("expected L1, got %s", level)
			}
		})
	}
}

func TestComputeLevel_L2(t *testing.T) {
	tests := []struct {
		name  string
		obs   []Observation
		flags Flags
	}{
		{
			name: "部位特写 2 次",
			obs: []Observation{
				{Type: ObsBodyPartCloseup, Timestamp: 1.0},
				{Type: ObsBodyPartCloseup, Timestamp: 5.0},
			},
			flags: Flags{},
		},
		{
			name: "部位特写累计超过 3 秒",
			obs: []Observation{
				{Type: ObsBodyPartCloseup, Timestamp: 1.0},
				{Type: ObsBodyPartCloseup, Timestamp: 3.0},
			},
			flags: Flags{},
		},
		{
			name:  "性暗示动作",
			obs:   []Observation{{Type: ObsSexualGesture, Timestamp: 4.0}},
			flags: Flags{},
		},
		{
			name:  "带性意味声音",
			obs:   []Observation{{Type: ObsSexualSound, Timestamp: 6.0}},
			flags: Flags{},
		},
		{
			name: "两种 L1 表现",
			obs: []Observation{
				{Type: ObsBodyPartCloseup, Timestamp: 2.0},
				{Type: ObsSwimwearOffContext, Timestamp: 4.0},
			},
			flags: Flags{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := ComputeLevel(tt.obs, tt.flags)
			if level != L2 {
				t.Errorf("expected L2, got %s", level)
			}
		})
	}
}

func TestComputeLevel_L2_叠加L1仍为L2(t *testing.T) {
	obs := []Observation{
		{Type: ObsBodyPartCloseup, Timestamp: 1.0},
		{Type: ObsBodyPartCloseup, Timestamp: 5.0},
		{Type: ObsSwimwearOffContext, Timestamp: 7.0},
	}
	level := ComputeLevel(obs, Flags{})
	if level != L2 {
		t.Errorf("L2 叠加 L1 应仍为 L2, got %s", level)
	}
}

func TestComputeLevel_L3(t *testing.T) {
	tests := []struct {
		name  string
		obs   []Observation
		flags Flags
	}{
		{
			name:  "性行为",
			obs:   []Observation{{Type: ObsSexualAct, Timestamp: 3.0}},
			flags: Flags{},
		},
		{
			name:  "私密部位展示",
			obs:   []Observation{{Type: ObsPrivatePart, Timestamp: 5.0}},
			flags: Flags{},
		},
		{
			name: "两种 L2 表现",
			obs: []Observation{
				{Type: ObsBodyPartCloseup, Timestamp: 1.0},
				{Type: ObsBodyPartCloseup, Timestamp: 5.0},
				{Type: ObsSexualGesture, Timestamp: 8.0},
			},
			flags: Flags{},
		},
		{
			name:  "疑似未成年 + 性暗示动作",
			obs:   []Observation{{Type: ObsSexualGesture, Timestamp: 2.0}},
			flags: Flags{HasSuspectedMinor: true},
		},
		{
			name:  "疑似未成年 + 性意味声音",
			obs:   []Observation{{Type: ObsSexualSound, Timestamp: 4.0}},
			flags: Flags{HasSuspectedMinor: true},
		},
		{
			name:  "疑似未成年 + 部位特写 1 次（成年人只算 L1）",
			obs:   []Observation{{Type: ObsBodyPartCloseup, Timestamp: 4.0}},
			flags: Flags{HasSuspectedMinor: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := ComputeLevel(tt.obs, tt.flags)
			if level != L3 {
				t.Errorf("expected L3, got %s", level)
			}
		})
	}
}

func TestEstimateDuration(t *testing.T) {
	tests := []struct {
		name       string
		timestamps []float64
		expected   float64
	}{
		{
			name:       "空列表",
			timestamps: []float64{},
			expected:   0.0,
		},
		{
			name:       "单个时间点",
			timestamps: []float64{1.0},
			expected:   2.0,
		},
		{
			name:       "相邻时间点小于2秒",
			timestamps: []float64{1.0, 1.5},
			expected:   2.0,
		},
		{
			name:       "相邻时间点大于2秒",
			timestamps: []float64{1.0, 5.0},
			expected:   4.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateDuration(tt.timestamps)
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}
