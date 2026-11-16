package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"pulseframe/internal/model"
	"pulseframe/internal/password"
	"pulseframe/internal/platform/config"
	"pulseframe/internal/platform/response"
	"pulseframe/internal/ratelimit"
	"pulseframe/internal/session"
	"pulseframe/internal/store"
	"pulseframe/internal/token"
)

// AuthHandler 处理登录、注册、刷新与退出等认证接口。
type AuthHandler struct {
	db           *gorm.DB           // 数据库实例。
	cfg          config.Auth        // 认证配置。
	signer       *token.Signer      // 访问令牌签发器。
	verifier     *token.Verifier    // 访问令牌验证器。
	sessions     *session.Manager   // 会话管理器。
	loginAccount *ratelimit.Limiter // 账号登录限流器。
	loginIP      *ratelimit.Limiter // IP 登录限流器。
	registerIP   *ratelimit.Limiter // IP 注册限流器。
}

// NewAuthHandler 创建认证处理器。
// 输入：db，数据库实例；redis，Redis 客户端；cfg，认证配置。
// 输出：认证处理器；JWT 密钥加载失败时返回错误。
// 功能：加载令牌签名/验证密钥，并创建会话管理器与限流器。
// 例如配置有效时返回可注册到路由的认证处理器。
func NewAuthHandler(db *gorm.DB, redis *redis.Client, cfg config.Auth) (*AuthHandler, error) {
	signer, err := token.NewSigner(cfg.JWTPrivateKeyPath, "pulseframe-key", cfg.JWTIssuer, cfg.JWTAudience) // 加载私钥
	if err != nil {
		return nil, err
	}
	verifier, err := token.NewVerifier(cfg.JWTPublicKeyPath, cfg.JWTIssuer, cfg.JWTAudience) // 加载公钥
	if err != nil {
		return nil, err
	}
	return &AuthHandler{
		db:           db,
		cfg:          cfg,
		signer:       signer,
		verifier:     verifier,
		sessions:     session.NewManager(redis),
		loginAccount: ratelimit.NewLimiter(redis, cfg.LoginAccountWindow, cfg.LoginAccountLimit, true),
		loginIP:      ratelimit.NewLimiter(redis, cfg.LoginIPWindow, cfg.LoginIPLimit, true),
		registerIP:   ratelimit.NewLimiter(redis, cfg.RegisterIPWindow, cfg.RegisterIPLimit, true),
	}, nil
}

// Register 注册账号。
// 输入：c，当前 Gin 请求上下文。
// 输出：成功写入201并返回无内容；输入非法、限流或冲突时写入对应错误。
// 功能：校验用户名密码、检查注册限流，并在事务中创建用户与凭据。
// 例如用户名可用时返回201，重复用户名返回409。
func (h *AuthHandler) Register(c *gin.Context) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} // 注册请求体
	if err := c.ShouldBindJSON(&body); err != nil { // 解析请求体
		response.WriteError(c, response.ErrorSpec{Status: http.StatusBadRequest, Code: response.CodeInvalidRequest, Message: "invalid request"})
		return
	}
	if !validUsername(body.Username) || !validPassword(body.Password) { // 校验格式
		response.WriteError(c, response.ErrorSpec{Status: http.StatusBadRequest, Code: response.CodeInvalidRequest, Message: "invalid username or password"})
		return
	}
	ip := normalizeIP(c.ClientIP()) // 规范化 IP
	if ip == "" {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusBadRequest, Code: response.CodeInvalidRequest, Message: "invalid client ip"})
		return
	}
	allowed, err := h.registerIP.Allow(c.Request.Context(), "rate:register:ip:"+ip) // 检查注册限流
	if err != nil {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusServiceUnavailable, Code: response.CodeDependencyUnavailable, Message: "rate limit unavailable"})
		return
	}
	if !allowed { // 超额
		response.WriteError(c, response.ErrorSpec{Status: http.StatusTooManyRequests, Code: response.CodeTooManyRequests, Message: "rate limited"})
		return
	}
	hash, err := password.Hash([]byte(body.Password), password.DefaultParams()) // 生成密码哈希
	if err != nil {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusInternalServerError, Code: response.CodeInternal, Message: "internal server error"})
		return
	}
	userID, err := newRandomID() // 生成用户标识
	if err != nil {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusInternalServerError, Code: response.CodeInternal, Message: "internal server error"})
		return
	}
	now := time.Now().UTC() // 当前时间
	err = store.CreateUserWithCredential(c.Request.Context(), h.db,
		&model.User{ID: userID, Status: "active", AuthVersion: 1, CreatedAt: now, UpdatedAt: now},
		&model.PasswordCredential{UserID: userID, Username: body.Username, PasswordHash: hash, CreatedAt: now, UpdatedAt: now},
	) // 事务创建用户与凭据
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 { // 唯一约束冲突
			response.WriteError(c, response.ErrorSpec{Status: http.StatusConflict, Code: response.CodeConflict, Message: "username already exists"})
			return
		}
		response.WriteError(c, response.ErrorSpec{Status: http.StatusServiceUnavailable, Code: response.CodeDependencyUnavailable, Message: "database unavailable"})
		return
	}
	c.Status(http.StatusCreated) // 返回201
}

// Login 密码登录。
// 输入：c，当前 Gin 请求上下文。
// 输出：成功写入200并返回访问令牌、下发刷新 Cookie；限流、凭证错误或账号禁用时写入对应错误。
// 功能：检查账号与 IP 限流，验证密码与账号状态，创建会话并返回令牌。
// 例如凭证正确返回200，密码错误返回401。
func (h *AuthHandler) Login(c *gin.Context) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} // 登录请求体
	if err := c.ShouldBindJSON(&body); err != nil { // 解析请求体
		response.WriteError(c, response.ErrorSpec{Status: http.StatusBadRequest, Code: response.CodeInvalidRequest, Message: "invalid request"})
		return
	}
	ip := normalizeIP(c.ClientIP()) // 规范化 IP
	if ip == "" {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusBadRequest, Code: response.CodeInvalidRequest, Message: "invalid client ip"})
		return
	}
	ctx := c.Request.Context()                                                     // 请求上下文
	allowed, err := h.loginAccount.Allow(ctx, "rate:login:account:"+body.Username) // 检查账号限流
	if err != nil {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusServiceUnavailable, Code: response.CodeDependencyUnavailable, Message: "rate limit unavailable"})
		return
	}
	if !allowed { // 账号超额
		response.WriteError(c, response.ErrorSpec{Status: http.StatusTooManyRequests, Code: response.CodeTooManyRequests, Message: "rate limited"})
		return
	}
	allowed, err = h.loginIP.Allow(ctx, "rate:login:ip:"+ip) // 检查 IP 限流
	if err != nil {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusServiceUnavailable, Code: response.CodeDependencyUnavailable, Message: "rate limit unavailable"})
		return
	}
	if !allowed { // IP 超额
		response.WriteError(c, response.ErrorSpec{Status: http.StatusTooManyRequests, Code: response.CodeTooManyRequests, Message: "rate limited"})
		return
	}
	credential, err := store.GetCredentialByUsername(ctx, h.db, body.Username) // 查询凭据
	if err != nil {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusUnauthorized, Code: "INVALID_CREDENTIALS", Message: "invalid credentials"})
		return
	}
	ok, err := password.Verify(credential.PasswordHash, []byte(body.Password)) // 验证密码
	if err != nil || !ok {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusUnauthorized, Code: "INVALID_CREDENTIALS", Message: "invalid credentials"})
		return
	}
	user, err := store.GetUserByID(ctx, h.db, credential.UserID) // 查询用户
	if err != nil {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusServiceUnavailable, Code: response.CodeDependencyUnavailable, Message: "database unavailable"})
		return
	}
	if user.Status != "active" { // 账号禁用
		response.WriteError(c, response.ErrorSpec{Status: http.StatusForbidden, Code: "ACCOUNT_DISABLED", Message: "account disabled"})
		return
	}
	h.issueSession(c, user) // 签发会话并返回令牌
}

// issueSession 创建会话并返回令牌、下发刷新 Cookie。
// 输入：c，当前 Gin 请求上下文；user，已验证用户。
// 输出：写入200并返回访问令牌；Redis 写入失败时写入503。
// 功能：生成会话与刷新令牌，写入 Redis 并签发访问令牌。
// 例如会话创建成功返回访问令牌与刷新 Cookie。
func (h *AuthHandler) issueSession(c *gin.Context, user *model.User) {
	ctx := c.Request.Context()      // 请求上下文
	sessionID, err := newRandomID() // 生成会话标识
	if err != nil {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusInternalServerError, Code: response.CodeInternal, Message: "internal server error"})
		return
	}
	refreshToken, digest, err := newRefreshToken() // 生成刷新令牌与摘要
	if err != nil {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusInternalServerError, Code: response.CodeInternal, Message: "internal server error"})
		return
	}
	now := time.Now().Unix()                                                                                        // 当前秒数
	absoluteExpiresAt := now + int64(h.cfg.SessionTTL.Seconds())                                                    // 绝对到期时间
	if err := h.sessions.Create(ctx, sessionID, user.ID, user.AuthVersion, digest, absoluteExpiresAt); err != nil { // 写会话
		response.WriteError(c, response.ErrorSpec{Status: http.StatusServiceUnavailable, Code: response.CodeDependencyUnavailable, Message: "session unavailable"})
		return
	}
	exp := min(now+int64(h.cfg.AccessTokenTTL.Seconds()), absoluteExpiresAt)                       // 访问令牌到期时间
	accessToken, err := h.signer.Sign(user.ID, sessionID, now, time.Duration(exp-now)*time.Second) // 签发令牌
	if err != nil {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusInternalServerError, Code: response.CodeInternal, Message: "internal server error"})
		return
	}
	setRefreshCookie(c, refreshToken, int(absoluteExpiresAt-now))                    // 下发刷新 Cookie
	c.JSON(http.StatusOK, gin.H{"accessToken": accessToken, "expiresIn": exp - now}) // 返回访问令牌
}

// Refresh 刷新访问令牌。
// 输入：c，当前 Gin 请求上下文。
// 输出：成功写入200并返回新访问令牌；会话失效、账号禁用或 Redis 故障时写入对应错误。
// 功能：校验 Origin 与刷新 Cookie，读取会话、检查账号状态与版本，并原子轮换令牌。
// 例如会话有效返回新令牌，会话过期返回401。
func (h *AuthHandler) Refresh(c *gin.Context) {
	if !h.originAllowed(c) { // 校验 Origin
		response.WriteError(c, response.ErrorSpec{Status: http.StatusForbidden, Code: response.CodeForbidden, Message: "origin not allowed"})
		return
	}
	refreshToken, err := c.Cookie("refreshToken") // 读取刷新 Cookie
	if err != nil || refreshToken == "" {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusUnauthorized, Code: "LOGIN_EXPIRED", Message: "login expired"})
		return
	}
	ctx := c.Request.Context()                 // 请求上下文
	digest := refreshTokenDigest(refreshToken) // 计算拓扑
	sess, err := h.sessions.Get(ctx, digest)   // 读取会话
	if errors.Is(err, session.ErrNotFound) {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusUnauthorized, Code: "LOGIN_EXPIRED", Message: "login expired"})
		return
	}
	if err != nil {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusServiceUnavailable, Code: response.CodeDependencyUnavailable, Message: "session unavailable"})
		return
	}
	user, err := store.GetUserByID(ctx, h.db, sess.UserID) // 查询用户
	if err != nil {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusServiceUnavailable, Code: response.CodeDependencyUnavailable, Message: "database unavailable"})
		return
	}
	if user.Status != "active" { // 账号禁用
		_ = h.sessions.Delete(ctx, digest, sess.SessionID)
		response.WriteError(c, response.ErrorSpec{Status: http.StatusForbidden, Code: "ACCOUNT_DISABLED", Message: "account disabled"})
		return
	}
	if user.AuthVersion != sess.AuthVersion { // 版本不一致
		_ = h.sessions.Delete(ctx, digest, sess.SessionID)
		response.WriteError(c, response.ErrorSpec{Status: http.StatusUnauthorized, Code: "LOGIN_EXPIRED", Message: "login expired"})
		return
	}
	newToken, newDigest, err := newRefreshToken() // 生成新刷新令牌
	if err != nil {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusInternalServerError, Code: response.CodeInternal, Message: "internal server error"})
		return
	}
	err = h.sessions.Rotate(ctx, digest, sess.SessionID, user.AuthVersion, newDigest, sess.AbsoluteExpiresAt) // 轮换令牌
	if errors.Is(err, session.ErrTokenMismatch) || errors.Is(err, session.ErrNotFound) {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusUnauthorized, Code: "LOGIN_EXPIRED", Message: "login expired"})
		return
	}
	if err != nil {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusServiceUnavailable, Code: response.CodeDependencyUnavailable, Message: "session unavailable"})
		return
	}
	now := time.Now().Unix()                                                                            // 当前秒数
	exp := min(now+int64(h.cfg.AccessTokenTTL.Seconds()), sess.AbsoluteExpiresAt)                       // 访问令牌到期时间
	accessToken, err := h.signer.Sign(user.ID, sess.SessionID, now, time.Duration(exp-now)*time.Second) // 签发令牌
	if err != nil {
		response.WriteError(c, response.ErrorSpec{Status: http.StatusInternalServerError, Code: response.CodeInternal, Message: "internal server error"})
		return
	}
	setRefreshCookie(c, newToken, int(sess.AbsoluteExpiresAt-now))                   // 下发新刷新 Cookie
	c.JSON(http.StatusOK, gin.H{"accessToken": accessToken, "expiresIn": exp - now}) // 返回新访问令牌
}

// Logout 退出登录。
// 输入：c，当前 Gin 请求上下文。
// 输出：成功写入204并清除 Cookie；校验失败或 Redis 故障时写入对应错误。
// 功能：校验 Origin，读取刷新 Cookie，删除会话并清除刷新 Cookie。
// 例如正常退出返回204。
func (h *AuthHandler) Logout(c *gin.Context) {
	if !h.originAllowed(c) { // 校验 Origin
		response.WriteError(c, response.ErrorSpec{Status: http.StatusForbidden, Code: response.CodeForbidden, Message: "origin not allowed"})
		return
	}
	refreshToken, err := c.Cookie("refreshToken") // 读取刷新 Cookie
	if err != nil || refreshToken == "" {
		c.Status(http.StatusNoContent) // 无会话按已退出处理
		return
	}
	ctx := c.Request.Context()                 // 请求上下文
	digest := refreshTokenDigest(refreshToken) // 计算摘要
	sess, err := h.sessions.Get(ctx, digest)   // 读取会话
	if err == nil {
		_ = h.sessions.Delete(ctx, digest, sess.SessionID) // 删除会话
	}
	clearRefreshCookie(c) // 清除 Cookie
	c.Status(http.StatusNoContent)
}
