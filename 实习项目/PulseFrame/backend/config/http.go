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
// 例如 HTTP_ADDR 为 :9090、HTTP_READ_TIMEOUT 为 "3s" 时，得到9090端口和3秒读取期限。
func loadHTTP(lookup LookupEnv) (HTTP, error) {
	// 先读取服务要监听的地址；没有设置 HTTP_ADDR 或内容为空时，使用 :8080。
	address := readValue(lookup, envHTTPAddress, defaultHTTPAddress)
	// 地址必须包含有效端口，检查通过后才继续读取超时配置。
	if err := validateAddress(address); err != nil {
		return HTTP{}, err
	}

	// 接下来逐项读取请求头、完整请求、响应写入、连接空闲和退出等待的期限。
	// 每一项都先取环境变量或默认值，再把文字换算成时间长度；任意一项有问题就停止加载。
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
// 例如 :8080 检查通过；缺少冒号的 "8080" 或端口为零的 :0 返回错误。
func validateAddress(address string) error {
	// 从地址中分别取出主机和端口，例如 "localhost:8080" 中的端口是 "8080"。
	// 这里主要检查端口，主机部分允许为空，表示监听所有网卡。
	_, portText, err := net.SplitHostPort(address)
	// 无法从地址中分出主机和端口时，说明格式有误，立即报告配置名称和原因。
	if err != nil {
		return fmt.Errorf("%s must use host:port format: %w", envHTTPAddress, err)
	}
	// 取得的端口还是文字，先转换成整数，才能检查它有没有超出允许的范围。
	port, err := strconv.Atoi(portText)
	// 配置端口须为有效的非零数字，不在启动时自动选择端口。
	if err != nil || port < minimumPort || port > maximumPort {
		return fmt.Errorf("%s port must be between %d and %d", envHTTPAddress, minimumPort, maximumPort)
	}
	return nil
}
