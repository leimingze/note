package controller

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"pulseframe/dao"
	"pulseframe/response"
	"pulseframe/service"
)

const maxRegistrationBodyBytes = 1024

// Registration 管理注册 HTTP 请求。
type Registration struct {
	service *service.Registration
	logger  *slog.Logger
}

type registrationRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// NewRegistration 构造注册 HTTP 控制器。
// 输入：registration，注册服务；logger，记录内部错误的日志器，均不能为 nil。
// 输出：注册控制器；依赖缺失时返回错误。
// 功能：确保 HTTP 路由接入完整的注册依赖。
func NewRegistration(registration *service.Registration, logger *slog.Logger) (*Registration, error) {
	if registration == nil || logger == nil {
		return nil, errors.New("registration service and logger are required")
	}
	return &Registration{service: registration, logger: logger}, nil
}

// RegisterRoutes 注册账号创建接口。
// 输入：router，Gin 路由，不能为 nil。
// 输出：修改路由表，缺少路由时 panic。
// 功能：向已安装公共中间件的路由注册 POST 接口。
func (registration *Registration) RegisterRoutes(router *gin.Engine) {
	router.POST("/api/v1/auth/register", registration.handle)
}

// handle 处理一次注册请求。
// 输入：c，Gin 请求上下文，包含 JSON 用户名和密码。
// 输出：成功写入 201；非法输入、用户名冲突或内部错误写入统一响应。
// 功能：解析和限制请求体，调用服务完成注册并映射公开错误。
func (registration *Registration) handle(c *gin.Context) {
	var input registrationRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRegistrationBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusBadRequest, Code: response.CodeInvalidArgument, Message: "invalid registration request"})
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusBadRequest, Code: response.CodeInvalidArgument, Message: "invalid registration request"})
		return
	}
	err := registration.service.Register(c.Request.Context(), input.Username, input.Password)
	switch {
	case err == nil:
		c.Status(http.StatusCreated)
	case errors.Is(err, service.ErrInvalidUsername), errors.Is(err, service.ErrInvalidPassword):
		response.WriteError(c, response.ErrorSpec{Status: http.StatusBadRequest, Code: response.CodeInvalidArgument, Message: err.Error()})
	case errors.Is(err, dao.ErrUsernameExists):
		response.WriteError(c, response.ErrorSpec{Status: http.StatusConflict, Code: response.CodeUsernameExists, Message: "username already exists"})
	default:
		registration.logger.Error("registration failed", "error", err, "request_id", c.GetString(response.RequestIDContextKey))
		response.WriteError(c, response.ErrorSpec{Status: http.StatusInternalServerError, Code: response.CodeInternal, Message: "internal server error"})
	}
}
