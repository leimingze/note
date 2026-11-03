package test

import (
	"strings"
	"testing"
	"time"

	"pulseframe/config"
)

// 输入：无。
// 输出：仅含数据库必填项的独立环境映射，不访问数据库。
// 功能：为配置解析测试提供有效输入，防止必填错误遮盖其他校验。
// 例如返回的映射包含 MYSQL_USER 和 MYSQL_DATABASE，并显式把 MYSQL_PASSWORD 设置为空字符串。
func mysqlEnvironment() envMap {
	return envMap{"MYSQL_USER": "config-test", "MYSQL_PASSWORD": "", "MYSQL_DATABASE": "config-test"}
}

// 输入：t，测试上下文。
// 输出：默认连接上限和期限不符合预期时测试失败。
// 功能：验证可选参数具有有界的默认配置，并允许零个空闲连接。
// 例如未设置连接数量时检查总上限为20、空闲上限为5；把空闲上限设置为0也应通过。
func TestMySQLDefaults(t *testing.T) {
	values := mysqlEnvironment()
	cfg, err := config.Load(values.Lookup)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MySQL.MaxOpenConns != 20 || cfg.MySQL.MaxIdleConns != 5 {
		t.Fatal("unexpected default pool capacity")
	}
	if cfg.MySQL.ConnectTimeout != 3*time.Second || cfg.MySQL.StartupTimeout != 5*time.Second {
		t.Fatal("unexpected default connection deadlines")
	}
	if cfg.MySQL.ReadTimeout != 5*time.Second || cfg.MySQL.WriteTimeout != 5*time.Second {
		t.Fatal("unexpected default I/O deadlines")
	}
	if cfg.MySQL.ConnMaxLifetime != 30*time.Minute || cfg.MySQL.ConnMaxIdleTime != 5*time.Minute {
		t.Fatal("unexpected connection recycling deadlines")
	}
	values["MYSQL_MAX_IDLE_CONNS"] = "0"
	cfg, err = config.Load(values.Lookup)
	if err != nil || cfg.MySQL.MaxIdleConns != 0 {
		t.Fatalf("zero idle connections rejected: %v", err)
	}
}

// 输入：t，测试上下文。
// 输出：原始认证信息被改写或合法参数未生效时测试失败。
// 功能：验证密码空白和 DSN 特殊字符保留，以及 IPv6 和 TLS 配置。
// 例如地址设置为 [::1]:3307、启动期限设置为 "250ms" 后，检查两项配置按原意生效。
func TestMySQLOverrides(t *testing.T) {
	values := mysqlEnvironment()
	values["MYSQL_PASSWORD"] = " \t:/@?#&=\n "
	values["MYSQL_ADDR"] = "[::1]:3307"
	values["MYSQL_TLS_MODE"] = "true"
	values["MYSQL_MAX_OPEN_CONNS"] = "3"
	values["MYSQL_MAX_IDLE_CONNS"] = "1"
	values["MYSQL_STARTUP_TIMEOUT"] = "250ms"
	cfg, err := config.Load(values.Lookup)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MySQL.Password != values["MYSQL_PASSWORD"] {
		t.Fatal("password changed during configuration loading")
	}
	if cfg.MySQL.Address != "[::1]:3307" || cfg.MySQL.TLSMode != "true" {
		t.Fatal("address or TLS override ignored")
	}
	if cfg.MySQL.MaxOpenConns != 3 || cfg.MySQL.MaxIdleConns != 1 || cfg.MySQL.StartupTimeout != 250*time.Millisecond {
		t.Fatal("pool or timeout override ignored")
	}
}

// 输入：t，测试上下文。
// 输出：缺失必填变量未被拒绝时测试失败。
// 功能：确保数据库必填项不会采用隐含账号或数据库。
// 例如移除 MYSQL_USER 后，加载错误中应包含 MYSQL_USER，指出缺少的配置。
func TestMySQLRequiredVariables(t *testing.T) {
	// 每轮只移除一个必填项，确认错误能够指出对应变量。
	for _, key := range []string{"MYSQL_USER", "MYSQL_PASSWORD", "MYSQL_DATABASE"} {
		values := mysqlEnvironment()
		delete(values, key)
		_, err := config.Load(values.Lookup)
		if err == nil || !strings.Contains(err.Error(), key) {
			t.Fatalf("missing %s: %v", key, err)
		}
	}
}

// 输入：t，测试上下文。
// 输出：非法连接参数未被拒绝或错误未定位配置项时测试失败。
// 功能：验证连接数量、地址、认证信息和 TLS 模式的边界。
// 例如总上限保持默认20而空闲上限设置为21时，应报告 MYSQL_MAX_IDLE_CONNS 错误。
func TestMySQLRejectsInvalidValues(t *testing.T) {
	cases := []struct{ key, value string }{
		{"MYSQL_ADDR", "localhost"}, {"MYSQL_ADDR", ":3306"},
		{"MYSQL_ADDR", "127.0.0.1:0"}, {"MYSQL_ADDR", "127.0.0.1:65536"},
		{"MYSQL_ADDR", "127.0.0.1:abc"}, {"MYSQL_USER", " "},
		{"MYSQL_DATABASE", " "}, {"MYSQL_TLS_MODE", "skip-verify"},
		{"MYSQL_MAX_OPEN_CONNS", "0"}, {"MYSQL_MAX_OPEN_CONNS", "-1"},
		{"MYSQL_MAX_OPEN_CONNS", "abc"}, {"MYSQL_MAX_OPEN_CONNS", "999999999999999999999"},
		{"MYSQL_MAX_IDLE_CONNS", "-1"}, {"MYSQL_MAX_IDLE_CONNS", "21"},
	}
	// 每轮从有效配置开始，只覆盖当前待验证的非法值，保持各场景相互独立。
	for _, item := range cases {
		values := mysqlEnvironment()
		values[item.key] = item.value
		_, err := config.Load(values.Lookup)
		if err == nil || !strings.Contains(err.Error(), item.key) {
			t.Fatalf("invalid %s: %v", item.key, err)
		}
	}
}

// 输入：t，测试上下文。
// 输出：任一无效期限被接受时测试失败。
// 功能：防止零值、负值和格式错误关闭超时及连接回收。
// 例如 MYSQL_CONNECT_TIMEOUT 分别设置为 "0s"、"-1s"、"invalid" 时，都应报错。
func TestMySQLRejectsInvalidDurations(t *testing.T) {
	keys := []string{
		"MYSQL_CONNECT_TIMEOUT", "MYSQL_READ_TIMEOUT", "MYSQL_WRITE_TIMEOUT",
		"MYSQL_STARTUP_TIMEOUT", "MYSQL_CONN_MAX_LIFETIME", "MYSQL_CONN_MAX_IDLE_TIME",
	}
	// 逐项验证三类错误，防止某个期限遗漏正数和格式校验。
	for _, key := range keys {
		for _, raw := range []string{"0s", "-1s", "invalid"} {
			values := mysqlEnvironment()
			values[key] = raw
			_, err := config.Load(values.Lookup)
			if err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("invalid %s: %v", key, err)
			}
		}
	}
}
