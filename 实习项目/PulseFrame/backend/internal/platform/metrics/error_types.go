package metrics

// 服务端错误分类枚举，作为错误类型，仅在日志和指标中使用，不对客户端暴露。
const (
	ErrorConfigInvalid             = "config_invalid"              // 配置加载失败。
	ErrorRequestInvalid            = "request_invalid"             // 请求参数或格式非法。
	ErrorAuthFailed                = "authentication_failed"       // 认证失败。
	ErrorAuthDenied                = "authorization_denied"        // 授权不足。
	ErrorRateLimited               = "rate_limited"                // 触发限流。
	ErrorDependencyUnavailable     = "dependency_unavailable"      // 外部依赖不可用。
	ErrorDependencyTimeout         = "dependency_timeout"          // 外部依赖超时。
	ErrorDependencyResponseInvalid = "dependency_response_invalid" // 外部依赖响应非法。
	ErrorDependencyBusiness        = "dependency_business_error"   // 外部依赖业务错误。
	ErrorInternal                  = "internal_error"              // 非预期内部错误。
	ErrorPanic                     = "panic"                       // 请求链路发生 panic。
	ErrorShutdown                  = "shutdown"                    // 服务退出流程错误。
)
