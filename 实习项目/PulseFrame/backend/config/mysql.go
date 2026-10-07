package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

const (
	envMySQLAddress  = "MYSQL_ADDR"
	envMySQLUser     = "MYSQL_USER"
	envMySQLDatabase = "MYSQL_DATABASE"
	envMySQLPassword = "PULSEFRAME_MYSQL_PASSWORD"
	envMySQLTLS      = "MYSQL_TLS"
)

// MySQL 保存连接账号数据库所需的配置。
type MySQL struct {
	Address  string
	User     string
	Database string
	Password string
	TLS      bool
}

// LoadMySQL 读取账号数据库配置。
// 输入：lookup，环境变量读取函数，不能为 nil。
// 输出：完整连接参数；必填项缺失或地址非法时返回错误。
// 功能：启动前校验数据库参数，避免运行时才发现缺少连接信息。
func LoadMySQL(lookup LookupEnv) (MySQL, error) {
	if lookup == nil {
		return MySQL{}, errNilEnvironmentLookup
	}
	values := make([]string, 0, 4)
	for _, key := range []string{envMySQLAddress, envMySQLUser, envMySQLDatabase, envMySQLPassword} {
		value, exists := lookup(key)
		if !exists || strings.TrimSpace(value) == "" {
			return MySQL{}, fmt.Errorf("%s is required", key)
		}
		values = append(values, value)
	}
	host, portText, err := net.SplitHostPort(values[0])
	if err != nil || host == "" {
		return MySQL{}, fmt.Errorf("%s must use host:port format", envMySQLAddress)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < minimumPort || port > maximumPort {
		return MySQL{}, fmt.Errorf("%s has invalid port", envMySQLAddress)
	}
	tls, err := strconv.ParseBool(readValue(lookup, envMySQLTLS, "false"))
	if err != nil {
		return MySQL{}, fmt.Errorf("%s must be true or false: %w", envMySQLTLS, err)
	}
	return MySQL{Address: values[0], User: values[1], Database: values[2], Password: values[3], TLS: tls}, nil
}
