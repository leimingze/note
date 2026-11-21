package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type HTTP struct {
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

func LoadHTTP() (HTTP, error) {
	port, _ := strconv.Atoi(getEnv("HTTP_PORT", "8081"))
	readTimeout, _ := time.ParseDuration(getEnv("HTTP_READ_TIMEOUT", "30s"))
	writeTimeout, _ := time.ParseDuration(getEnv("HTTP_WRITE_TIMEOUT", "30s"))
	shutdownTimeout, _ := time.ParseDuration(getEnv("HTTP_SHUTDOWN_TIMEOUT", "10s"))

	cfg := HTTP{
		Port:            port,
		ReadTimeout:     readTimeout,
		WriteTimeout:    writeTimeout,
		ShutdownTimeout: shutdownTimeout,
	}

	if cfg.Port <= 0 || cfg.Port > 65535 {
		return cfg, fmt.Errorf("invalid HTTP_PORT: %d", cfg.Port)
	}
	if cfg.ReadTimeout <= 0 || cfg.WriteTimeout <= 0 || cfg.ShutdownTimeout <= 0 {
		return cfg, fmt.Errorf("invalid timeout values")
	}
	return cfg, nil
}

type MySQL struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func LoadMySQL() (MySQL, error) {
	port, _ := strconv.Atoi(getEnv("MYSQL_PORT", "3306"))
	maxOpen, _ := strconv.Atoi(getEnv("MYSQL_MAX_OPEN_CONNS", "25"))
	maxIdle, _ := strconv.Atoi(getEnv("MYSQL_MAX_IDLE_CONNS", "10"))
	lifetime, _ := time.ParseDuration(getEnv("MYSQL_CONN_MAX_LIFETIME", "5m"))

	cfg := MySQL{
		Host:            getEnv("MYSQL_HOST", "127.0.0.1"),
		Port:            port,
		User:            os.Getenv("MYSQL_USER"),
		Password:        os.Getenv("MYSQL_PASSWORD"),
		Database:        os.Getenv("MYSQL_DATABASE"),
		MaxOpenConns:    maxOpen,
		MaxIdleConns:    maxIdle,
		ConnMaxLifetime: lifetime,
	}

	if cfg.User == "" || cfg.Password == "" || cfg.Database == "" {
		return cfg, fmt.Errorf("MYSQL_USER, MYSQL_PASSWORD, MYSQL_DATABASE are required")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return cfg, fmt.Errorf("invalid MYSQL_PORT: %d", cfg.Port)
	}
	if cfg.MaxOpenConns <= 0 || cfg.MaxIdleConns <= 0 || cfg.MaxIdleConns > cfg.MaxOpenConns {
		return cfg, fmt.Errorf("invalid connection pool settings")
	}
	return cfg, nil
}

type ContentModule struct {
	BaseURL    string
	ServiceKey string
	Timeout    time.Duration
}

func LoadContentModule() (ContentModule, error) {
	timeout, _ := time.ParseDuration(getEnv("CONTENT_MODULE_TIMEOUT", "30s"))
	cfg := ContentModule{
		BaseURL:    getEnv("CONTENT_MODULE_BASE_URL", "http://127.0.0.1:8080"),
		ServiceKey: os.Getenv("CONTENT_MODULE_SERVICE_KEY"),
		Timeout:    timeout,
	}
	if cfg.ServiceKey == "" {
		return cfg, fmt.Errorf("CONTENT_MODULE_SERVICE_KEY is required")
	}
	if cfg.Timeout <= 0 {
		return cfg, fmt.Errorf("invalid timeout")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// Review 配置审核队列、基础服务与审核规则。
type Review struct {
	MediaRoot        string        // 原文件根目录，media_key 相对它定位（本地目录代替私有桶）
	WorkDir          string        // 挑帧、转写的临时目录，每个任务一个子目录
	WorkerURL        string        // Python worker 地址
	PromptPath       string        // 观察项提示词文件
	Workers          int           // 同时处理的任务数
	RetryMax         time.Duration // 持续故障时同一任务重试间隔的上限
	RetryBase        time.Duration // 机审重试的首次间隔，之后翻倍
	Lease            time.Duration // 领取任务后的租约，进程崩溃时租约到期可被重新领取
	CallbackBase     time.Duration // 回调失败后的首次重试间隔，之后翻倍
	CallbackMax      time.Duration // 回调重试间隔上限
	PollInterval     time.Duration // 没有任务时的轮询间隔
	PolicyFile       string        // 审核策略 JSON 文件路径。
	PolicyTable      string        // 启动表名，空值使用配置文件中的默认表。
	PolicyExpireAt   time.Time     // 启动表的到期时间，零值使用规则文件设置。
	MaxVideoDuration float64       // 视频时长上限（秒），兜底校验
}

func LoadReview() (Review, error) {
	cfg := Review{
		MediaRoot:        getEnv("MEDIA_ROOT", "/data/media"),
		WorkDir:          getEnv("WORK_DIR", "/tmp/pulseframe-review"),
		WorkerURL:        getEnv("WORKER_URL", "http://127.0.0.1:8090"),
		PromptPath:       getEnv("PROMPT_PATH", "policies/observation/system_prompt.txt"),
		PolicyTable:      os.Getenv("POLICY_TABLE"),
		PolicyFile:       getEnv("POLICY_FILE", "policies/moderation.json"),
		MaxVideoDuration: 20,
	}
	workers, err := strconv.Atoi(getEnv("REVIEW_WORKERS", "2"))
	if err != nil || workers <= 0 {
		return cfg, fmt.Errorf("invalid REVIEW_WORKERS")
	}
	cfg.Workers = workers
	for key, spec := range map[string]struct {
		fallback string
		target   *time.Duration
	}{
		"REVIEW_RETRY_BASE":    {"1m", &cfg.RetryBase},
		"REVIEW_LEASE":         {"10m", &cfg.Lease},
		"CALLBACK_RETRY_BASE":  {"1s", &cfg.CallbackBase},
		"CALLBACK_RETRY_MAX":   {"5m", &cfg.CallbackMax},
		"REVIEW_POLL_INTERVAL": {"1s", &cfg.PollInterval},
	} {
		value, err := time.ParseDuration(getEnv(key, spec.fallback))
		if err != nil || value <= 0 {
			return cfg, fmt.Errorf("invalid %s", key)
		}
		*spec.target = value
	}
	cfg.RetryMax, err = time.ParseDuration(getEnv("REVIEW_RETRY_MAX", max(cfg.RetryBase, 4*time.Minute).String()))
	if err != nil || cfg.RetryMax < cfg.RetryBase {
		return cfg, fmt.Errorf("REVIEW_RETRY_MAX must be at least REVIEW_RETRY_BASE")
	}
	if raw := getEnv("POLICY_EXPIRE_AT", os.Getenv("POLICY_STRICT_EXPIRE_AT")); raw != "" && cfg.PolicyTable != "" {
		expire, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return cfg, fmt.Errorf("invalid policy expiration: expected RFC3339")
		}
		cfg.PolicyExpireAt = expire
	}
	return cfg, nil
}
