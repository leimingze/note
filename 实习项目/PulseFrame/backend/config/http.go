package config

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

const (
	envHTTPAddress       = "HTTP_ADDR"                // HTTP 监听地址的环境变量名。
	envReadHeaderTimeout = "HTTP_READ_HEADER_TIMEOUT" // 请求头读取期限的环境变量名。
	envReadTimeout       = "HTTP_READ_TIMEOUT"        // 完整请求读取期限的环境变量名。
	envWriteTimeout      = "HTTP_WRITE_TIMEOUT"       // 响应写入期限的环境变量名。
	envIdleTimeout       = "HTTP_IDLE_TIMEOUT"        // HTTP 连接空闲期限的环境变量名。
	envShutdownTimeout   = "HTTP_SHUTDOWN_TIMEOUT"    // 优雅退出期限的环境变量名。

	defaultHTTPAddress    = ":8080"          // 默认监听本机所有网卡的8080端口。
	defaultReadHeaderTime = 5 * time.Second  // 默认最多用5秒读取请求头。
	defaultReadTime       = 15 * time.Second // 默认最多用15秒读取完整请求，包含请求体。
	defaultWriteTime      = 15 * time.Second // 默认响应写入期限为15秒。
	defaultIdleTime       = 60 * time.Second // 默认复用连接最多空闲60秒。
	defaultShutdownTime   = 10 * time.Second // 优雅退出时默认最多等待现有请求10秒。
)

// HTTP 保存 HTTP 服务生命周期配置。
type HTTP struct {
	Address           string        // HTTP 监听地址，使用 host:port；主机为空表示监听所有网卡。
	ReadHeaderTimeout time.Duration // 读取请求头的期限。
	ReadTimeout       time.Duration // 读取完整请求的期限，包含请求体。
	WriteTimeout      time.Duration // 向客户端写入响应的期限。
	IdleTimeout       time.Duration // 复用连接等待下一次请求的最长空闲时间。
	ShutdownTimeout   time.Duration // 服务退出时等待正在处理的请求完成的期限。
}

// 输入：lookup，已确认非 nil 的环境变量查询函数。
// 输出：HTTP 配置；地址或超时非法时返回错误。
// 功能：集中加载 HTTP 配置，在创建服务前确认监听参数及各阶段期限有效。
func loadHTTP(lookup LookupEnv) (HTTP, error) {
	// readValue 通过 lookup 读取 HTTP_ADDR，缺失或为空时返回默认地址 :8080。
	address := readValue(lookup, envHTTPAddress, defaultHTTPAddress)
	// validateAddress 检查 host:port 格式及端口范围，合法时返回 nil，否则返回错误。
	if err := validateAddress(address); err != nil {
		return HTTP{}, err
	}

	// 以下 parsePositiveDuration 调用均传入读取函数、变量名和默认时长，得到正数 time.Duration。
	// 第一项读取请求头超时，缺失时采用5秒；后续各项使用各自的变量名和默认值。
	readHeader, err := parsePositiveDuration(lookup, envReadHeaderTimeout, defaultReadHeaderTime)
	if err != nil {
		return HTTP{}, err
	}
	read, err := parsePositiveDuration(lookup, envReadTimeout, defaultReadTime)
	if err != nil {
		return HTTP{}, err
	}
	write, err := parsePositiveDuration(lookup, envWriteTimeout, defaultWriteTime)
	if err != nil {
		return HTTP{}, err
	}
	idle, err := parsePositiveDuration(lookup, envIdleTimeout, defaultIdleTime)
	if err != nil {
		return HTTP{}, err
	}
	shutdown, err := parsePositiveDuration(lookup, envShutdownTimeout, defaultShutdownTime)
	if err != nil {
		return HTTP{}, err
	}

	return HTTP{
		Address:           address,
		ReadHeaderTimeout: readHeader,
		ReadTimeout:       read,
		WriteTimeout:      write,
		IdleTimeout:       idle,
		ShutdownTimeout:   shutdown,
	}, nil
}

// 输入：address，符合 host:port 形式的监听地址。
// 输出：地址合法时返回 nil，否则返回包含配置名称的错误。
// 功能：在监听端口前暴露地址格式和端口范围问题。
func validateAddress(address string) error {
	// SplitHostPort 返回主机、端口文本和错误；这里只需要端口，用 _ 忽略主机返回值。
	_, portText, err := net.SplitHostPort(address)
	// 监听地址允许空主机表示所有网卡，同时支持带方括号的 IPv6 地址。
	if err != nil {
		return fmt.Errorf("%s must use host:port format: %w", envHTTPAddress, err)
	}
	// Atoi 将端口文本转换为整数，例如 "8080" 转为 8080，供下一步检查数值范围。
	port, err := strconv.Atoi(portText)
	// 配置端口须为有效的非零数字，不在启动时自动选择端口。
	if err != nil || port < minimumPort || port > maximumPort {
		return fmt.Errorf("%s port must be between %d and %d", envHTTPAddress, minimumPort, maximumPort)
	}
	return nil
}
