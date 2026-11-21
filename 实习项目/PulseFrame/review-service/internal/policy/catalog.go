package policy

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Table 是文件中的一张具名处置表。
type Table struct {
	Name           string     `json:"name"`                // 切换策略时使用的标识。
	Version        string     `json:"version"`             // 审核记录保存的版本。
	RequiresExpiry bool       `json:"requires_expiry"`     // 启用时是否必须设置到期时间。
	ExpireAt       *time.Time `json:"expire_at,omitempty"` // 默认到期时刻，可由启用请求覆盖。
	Rules          []Rule     `json:"rules"`               // 等级和标签对应的处置。
}

// Catalog 保存启动时读取的规则表集合。
type Catalog struct {
	Default string  `json:"default_table"` // 未指定策略和临时策略到期时使用的表。
	Tables  []Table `json:"tables"`        // 可供选择的规则表。
}

// 输入：path 为 JSON 配置路径。
// 输出：经过校验的策略集合，读取失败或规则无效时返回错误。
// 功能：一次加载全部规则，例如在启动前发现重复的版本号。
func LoadCatalog(path string) (*Catalog, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var c Catalog
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("unexpected content after catalog in %s", path)
	}
	names, versions := map[string]bool{}, map[string]bool{}
	for _, t := range c.Tables {
		if strings.TrimSpace(t.Name) == "" || strings.TrimSpace(t.Version) == "" || names[t.Name] || versions[t.Version] {
			return nil, fmt.Errorf("empty or duplicate table name/version: %q/%q", t.Name, t.Version)
		}
		if t.Name == c.Default && (t.RequiresExpiry || t.ExpireAt != nil) {
			return nil, fmt.Errorf("default table %q must be permanent", t.Name)
		}
		if err := validateRules(t.Rules); err != nil {
			return nil, fmt.Errorf("table %q: %w", t.Name, err)
		}
		names[t.Name], versions[t.Version] = true, true
	}
	if !names[c.Default] {
		return nil, fmt.Errorf("unknown default_table %q", c.Default)
	}
	return &c, nil
}

// 输入：规则列表，每项指定一个等级或标签。
// 输出：配置有效时返回 nil，重复、空规则或未知动作返回错误。
// 功能：校验处置映射，例如同一标签不能同时指定两个动作。
func validateRules(rules []Rule) error {
	if len(rules) == 0 {
		return fmt.Errorf("empty rules")
	}
	seen := map[string]bool{}
	for _, r := range rules {
		if (strings.TrimSpace(r.Label) == "") == (r.VulgarityLevel == "") {
			return fmt.Errorf("rule needs exactly one label or level")
		}
		switch r.VulgarityLevel {
		case "", "L0", "L1", "L2", "L3":
		default:
			return fmt.Errorf("unknown level %q", r.VulgarityLevel)
		}
		key := r.Label + "/" + string(r.VulgarityLevel)
		if seen[key] {
			return fmt.Errorf("duplicate rule %q", key)
		}
		seen[key] = true
		if err := r.Action.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// 输入：name 为表名，expiry 为可选到期时间。
// 输出：指定表的策略快照或配置错误。
// 功能：应用配置的到期规则，例如临时表可从接口接收到期时刻。
func (c *Catalog) Select(name string, expiry time.Time) (*Policy, error) {
	for _, t := range c.Tables {
		if t.Name != name {
			continue
		}
		p := &Policy{Version: t.Version, Rules: t.Rules, IsDefault: name == c.Default, ExpireAt: t.ExpireAt}
		if !expiry.IsZero() && !p.IsDefault {
			p.ExpireAt = &expiry
		}
		if t.RequiresExpiry && p.ExpireAt == nil {
			return nil, fmt.Errorf("table %q requires expire_at", name)
		}
		return p, nil
	}
	return nil, fmt.Errorf("unknown table %q", name)
}
