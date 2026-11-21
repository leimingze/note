package policy

import (
	"testing"
	"time"

	"pulseframe-review/internal/grading"
)

func TestDefaultPolicy_Evaluate(t *testing.T) {
	p := DefaultPolicy()

	tests := []struct {
		name             string
		level            grading.VulgarityLevel
		violenceDetected bool
		otherLabels      []string
		needHumanReview  bool
		expectedAction   Action
	}{
		{
			name:           "L0 放行",
			level:          grading.L0,
			expectedAction: ActionPass,
		},
		{
			name:           "L1 放行",
			level:          grading.L1,
			expectedAction: ActionPass,
		},
		{
			name:           "L2 限流",
			level:          grading.L2,
			expectedAction: ActionLimit,
		},
		{
			name:           "L3 拦截",
			level:          grading.L3,
			expectedAction: ActionReject,
		},
		{
			name:             "暴力拦截",
			level:            grading.L0,
			violenceDetected: true,
			expectedAction:   ActionReject,
		},
		{
			name:           "违法犯罪拦截",
			level:          grading.L0,
			otherLabels:    []string{"违法犯罪"},
			expectedAction: ActionReject,
		},
		{
			name:            "模型判转人审",
			level:           grading.L0,
			needHumanReview: true,
			expectedAction:  ActionHuman,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := p.Evaluate(tt.level, tt.violenceDetected, tt.otherLabels, tt.needHumanReview)
			if action != tt.expectedAction {
				t.Errorf("expected %s, got %s", tt.expectedAction, action)
			}
		})
	}
}

func TestStrictPolicy_L2拦截(t *testing.T) {
	expireAt := time.Now().UTC().Add(24 * time.Hour)
	p := StrictPolicy(expireAt)

	action := p.Evaluate(grading.L2, false, nil, false)
	if action != ActionReject {
		t.Errorf("严查期 L2 应拦截, got %s", action)
	}
}

func TestMostSevereAction(t *testing.T) {
	tests := []struct {
		name     string
		actions  []Action
		expected Action
	}{
		{
			name:     "空列表返回放行",
			actions:  []Action{},
			expected: ActionPass,
		},
		{
			name:     "取最严",
			actions:  []Action{ActionPass, ActionLimit, ActionReject},
			expected: ActionReject,
		},
		{
			name:     "人审和限流取人审",
			actions:  []Action{ActionLimit, ActionHuman},
			expected: ActionHuman,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mostSevereAction(tt.actions)
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestPolicy_IsExpired(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name     string
		expireAt *time.Time
		now      time.Time
		expected bool
	}{
		{
			name:     "平时规则不过期",
			expireAt: nil,
			now:      now,
			expected: false,
		},
		{
			name:     "未到期",
			expireAt: ptrTime(now.Add(1 * time.Hour)),
			now:      now,
			expected: false,
		},
		{
			name:     "已过期",
			expireAt: ptrTime(now.Add(-1 * time.Hour)),
			now:      now,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Policy{ExpireAt: tt.expireAt}
			got := p.IsExpired(tt.now)
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestEvaluateWithDefault_临时规则到期自动切换(t *testing.T) {
	now := time.Now().UTC()
	defaultP := DefaultPolicy()
	strictP := StrictPolicy(now.Add(-1 * time.Hour)) // 已过期

	action, active := EvaluateWithDefault(strictP, defaultP, grading.L2, false, nil, false)

	if action != ActionLimit {
		t.Errorf("到期后应按平时规则限流, got %s", action)
	}
	if active != defaultP {
		t.Errorf("到期后应使用平时规则")
	}
}

func TestShouldKeepInLibrary(t *testing.T) {
	defaultP := DefaultPolicy()

	tests := []struct {
		name     string
		level    grading.VulgarityLevel
		labels   []string
		expected bool
	}{
		{
			name:     "L3 平时也违规",
			level:    grading.L3,
			expected: true,
		},
		{
			name:     "L2 平时只限流算违规",
			level:    grading.L2,
			expected: false, // limit 不算拦截，所以不保留
		},
		{
			name:     "违法犯罪保留",
			level:    grading.L0,
			labels:   []string{"违法犯罪"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldKeepInLibrary(tt.level, tt.labels, defaultP)
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
