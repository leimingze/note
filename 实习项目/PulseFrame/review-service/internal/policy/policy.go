package policy

import (
	"fmt"
	"time"

	"pulseframe-review/internal/grading"
)

// Action 处置动作
type Action string

const (
	ActionPass   Action = "pass"   // 放行
	ActionLimit  Action = "limit"  // 限流
	ActionReject Action = "reject" // 拦截
	ActionHuman  Action = "human"  // 人审
)

// Rule 一条规则：类别或等级 → 处置
type Rule struct {
	Label          string                 `json:"label,omitempty"`           // 标签，如"违法犯罪"
	VulgarityLevel grading.VulgarityLevel `json:"vulgarity_level,omitempty"` // 低俗等级
	Action         Action                 `json:"action"`                    // 处置
}

// Policy 一个规则表版本
type Policy struct {
	Version   string     `json:"version"`
	Rules     []Rule     `json:"rules"`
	ExpireAt  *time.Time `json:"expire_at,omitempty"` // 临时规则的到期时间
	IsDefault bool       `json:"is_default"`          // 是否为平时规则
}

// Evaluate 根据观察结果和规则表计算处置
func (p *Policy) Evaluate(level grading.VulgarityLevel, violenceDetected bool, otherLabels []string, needHumanReview bool) Action {
	// 模型判定转人审
	if needHumanReview {
		return ActionHuman
	}

	// 收集所有匹配的规则
	actions := []Action{}

	// 低俗等级
	for _, rule := range p.Rules {
		if rule.VulgarityLevel != "" && rule.VulgarityLevel == level {
			actions = append(actions, rule.Action)
		}
	}

	// 暴力
	if violenceDetected {
		for _, rule := range p.Rules {
			if rule.Label == "暴力" {
				actions = append(actions, rule.Action)
			}
		}
	}

	// 其他标签
	for _, label := range otherLabels {
		for _, rule := range p.Rules {
			if rule.Label == label {
				actions = append(actions, rule.Action)
			}
		}
	}

	// 取最严处置
	return mostSevereAction(actions)
}

// mostSevereAction 返回最严格的处置
func mostSevereAction(actions []Action) Action {
	if len(actions) == 0 {
		return ActionPass
	}

	severity := map[Action]int{
		ActionPass:   0,
		ActionLimit:  1,
		ActionHuman:  2,
		ActionReject: 3,
	}

	maxSeverity := 0
	result := ActionPass

	for _, action := range actions {
		if s := severity[action]; s > maxSeverity {
			maxSeverity = s
			result = action
		}
	}

	return result
}

// IsExpired 判断规则表是否已过期
func (p *Policy) IsExpired(now time.Time) bool {
	if p.ExpireAt == nil {
		return false
	}
	return now.After(*p.ExpireAt)
}

// DefaultPolicy 平时规则表
func DefaultPolicy() *Policy {
	return &Policy{
		Version:   "default-v1",
		IsDefault: true,
		Rules: []Rule{
			{VulgarityLevel: grading.L0, Action: ActionPass},
			{VulgarityLevel: grading.L1, Action: ActionPass},
			{VulgarityLevel: grading.L2, Action: ActionLimit},
			{VulgarityLevel: grading.L3, Action: ActionReject},
			{Label: "暴力", Action: ActionReject},
			{Label: "危险", Action: ActionHuman},
			{Label: "冒犯", Action: ActionHuman},
			{Label: "违法犯罪", Action: ActionReject},
			{Label: "出售违禁品", Action: ActionReject},
			{Label: "危害未成年人身心健康", Action: ActionReject},
			{Label: "未成年不适当行为", Action: ActionReject},
			{Label: "涉政不当言行", Action: ActionReject},
			{Label: "历史虚无主义", Action: ActionReject},
			{Label: "辱骂、引战、宣扬仇恨", Action: ActionHuman},
			{Label: "不实信息", Action: ActionHuman},
			{Label: "夸大商品功效", Action: ActionHuman},
			{Label: "虚假承诺福利", Action: ActionHuman},
			{Label: "炒作卖货", Action: ActionHuman},
			{Label: "引导私下交易", Action: ActionReject},
			{Label: "未添加AI标识", Action: ActionHuman},
			{Label: "盗用抄袭作品", Action: ActionHuman},
		},
	}
}

// StrictPolicy 严查规则表示例
func StrictPolicy(expireAt time.Time) *Policy {
	return &Policy{
		Version:  "strict-v1",
		ExpireAt: &expireAt,
		Rules: []Rule{
			{VulgarityLevel: grading.L0, Action: ActionPass},
			{VulgarityLevel: grading.L1, Action: ActionPass},
			{VulgarityLevel: grading.L2, Action: ActionReject}, // 严查期拦截
			{VulgarityLevel: grading.L3, Action: ActionReject},
			{Label: "暴力", Action: ActionReject},
			{Label: "危险", Action: ActionReject}, // 严查期拦截
			{Label: "冒犯", Action: ActionHuman},
			{Label: "违法犯罪", Action: ActionReject},
			{Label: "出售违禁品", Action: ActionReject},
			{Label: "危害未成年人身心健康", Action: ActionReject},
			{Label: "未成年不适当行为", Action: ActionReject},
			{Label: "涉政不当言行", Action: ActionReject},
			{Label: "历史虚无主义", Action: ActionReject},
			{Label: "辱骂、引战、宣扬仇恨", Action: ActionReject}, // 严查期拦截
			{Label: "不实信息", Action: ActionHuman},
			{Label: "夸大商品功效", Action: ActionHuman},
			{Label: "虚假承诺福利", Action: ActionHuman},
			{Label: "炒作卖货", Action: ActionHuman},
			{Label: "引导私下交易", Action: ActionReject},
			{Label: "未添加AI标识", Action: ActionHuman},
			{Label: "盗用抄袭作品", Action: ActionHuman},
		},
	}
}

// EvaluateWithDefault 评估处置时自动处理临时规则到期
func EvaluateWithDefault(current *Policy, defaultPolicy *Policy, level grading.VulgarityLevel,
	violenceDetected bool, otherLabels []string, needHumanReview bool) (Action, *Policy) {

	activePolicy := current
	if current.IsExpired(time.Now().UTC()) {
		activePolicy = defaultPolicy
	}

	action := activePolicy.Evaluate(level, violenceDetected, otherLabels, needHumanReview)
	return action, activePolicy
}

// ShouldKeepInLibrary 判断违规库条目在临时规则到期后是否保留
func ShouldKeepInLibrary(level grading.VulgarityLevel, labels []string, defaultPolicy *Policy) bool {
	// 按平时规则算一遍
	action := defaultPolicy.Evaluate(level, false, labels, false)
	// 平时规则下也违规则保留
	return action == ActionReject || action == ActionHuman
}

// String 返回处置动作的字符串表示
func (a Action) String() string {
	return string(a)
}

// Validate 验证处置动作是否合法
func (a Action) Validate() error {
	switch a {
	case ActionPass, ActionLimit, ActionReject, ActionHuman:
		return nil
	default:
		return fmt.Errorf("invalid action: %s", a)
	}
}
