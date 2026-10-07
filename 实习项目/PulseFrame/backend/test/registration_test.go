package test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"

	"pulseframe/config"
	"pulseframe/controller"
	"pulseframe/dao"
	"pulseframe/response"
	"pulseframe/server"
	"pulseframe/service"
)

const testMySQLDSNEnv = "PULSEFRAME_TEST_MYSQL_DSN"

// registrationDatabase 创建仅供当前测试使用的真实 MySQL 数据库。
// 输入：t，测试上下文；环境变量提供可建库的 MySQL 连接，DSN 不指定数据库。
// 输出：已经创建 users 表的连接池；测试结束时关闭并删除本次创建的数据库。
// 功能：用真实唯一索引与事务行为验证注册，不修改现有业务数据库。
func registrationDatabase(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv(testMySQLDSNEnv)
	if dsn == "" {
		t.Skipf("%s is required for MySQL integration tests", testMySQLDSNEnv)
	}
	settings, err := mysql.ParseDSN(dsn)
	if err != nil || settings.DBName != "" {
		t.Fatalf("%s must be a valid DSN without database name: %v", testMySQLDSNEnv, err)
	}
	admin, err := sql.Open("mysql", settings.FormatDSN())
	if err != nil {
		t.Fatalf("open mysql admin connection: %v", err)
	}
	t.Cleanup(func() { admin.Close() })
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatalf("generate database name: %v", err)
	}
	name := "pulseframe_register_test_" + hex.EncodeToString(random[:])
	if _, err := admin.ExecContext(context.Background(), "CREATE DATABASE `"+name+"`"); err != nil {
		t.Fatalf("create test database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.ExecContext(context.Background(), "DROP DATABASE `"+name+"`"); err != nil {
			t.Errorf("drop test database %s: %v", name, err)
		}
	})
	settings.DBName = name
	db, err := dao.OpenMySQL(context.Background(), config.MySQL{
		Address: settings.Addr, User: settings.User, Database: name, Password: settings.Passwd,
	})
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	schema, err := os.ReadFile("../migrations/001_create_users.sql")
	if err != nil {
		t.Fatalf("read users schema: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), string(schema)); err != nil {
		t.Fatalf("create users table: %v", err)
	}
	return db
}

// registrationRouter 装配接入真实 MySQL 的注册接口。
// 输入：t，测试上下文；db，已经创建 users 表的数据库连接。
// 输出：包含公共中间件与注册路由的 Gin Engine；装配失败时结束测试。
// 功能：通过 HTTP 验证与生产入口相同的注册处理路径。
func registrationRouter(t *testing.T, db *sql.DB) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	router, err := server.NewRouter(server.RouterDependencies{
		Logger: logger, IDGenerator: staticIDGenerator{value: testRequestID}, Health: server.NewHealthState(),
	})
	if err != nil {
		t.Fatalf("create router: %v", err)
	}
	users, err := dao.NewUsers(db)
	if err != nil {
		t.Fatalf("create users: %v", err)
	}
	registration, err := service.NewRegistration(users)
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	handler, err := controller.NewRegistration(registration, logger)
	if err != nil {
		t.Fatalf("create controller: %v", err)
	}
	handler.RegisterRoutes(router)
	return router
}

// registerRequest 发起一条真实 HTTP 注册请求。
// 输入：router，HTTP 路由；body，原始 JSON 请求体。
// 输出：包含状态、响应体和头部的 Recorder。
// 功能：统一验证路由、JSON 解析、服务和数据访问的调用链。
func registerRequest(router http.Handler, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

// registrationJSON 编码注册请求。
// 输入：username，用户名；password，原始密码。
// 输出：符合接口字段约定的 JSON 字符串。
// 功能：让测试经由标准 JSON 编码器构造有效请求。
func registrationJSON(username, password string) string {
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	return string(body)
}

// TestRegistrationPersistsHash 验证注册只写入可用的普通账号。
// 输入：Go 测试上下文和真实 MySQL 测试连接配置。
// 输出：状态、密码哈希、账号默认值或重复响应错误时测试失败。
// 功能：覆盖注册成功、重复用户名、大小写和不自动登录的行为。
func TestRegistrationPersistsHash(t *testing.T) {
	db := registrationDatabase(t)
	router := registrationRouter(t, db)
	recorder := registerRequest(router, registrationJSON("alice", "secret-password"))
	if recorder.Code != http.StatusCreated || recorder.Header().Get("Set-Cookie") != "" {
		t.Fatalf("unexpected registration response: %d %s", recorder.Code, recorder.Body.String())
	}
	var hash string
	var status, role int
	if err := db.QueryRow("SELECT password_hash, status, role FROM users WHERE username = ?", "alice").Scan(&hash, &status, &role); err != nil {
		t.Fatalf("load registered user: %v", err)
	}
	if hash == "secret-password" || bcrypt.CompareHashAndPassword([]byte(hash), []byte("secret-password")) != nil || status != 1 || role != 0 {
		t.Fatalf("invalid stored account: status=%d role=%d", status, role)
	}
	assertErrorResponse(t, registerRequest(router, registrationJSON("alice", "different")), errorExpectation{
		Status: http.StatusConflict, Code: response.CodeUsernameExists, RequireRequestID: true,
	})
	if got := registerRequest(router, registrationJSON("Alice", "different")); got.Code != http.StatusCreated {
		t.Fatalf("case-sensitive username should be allowed: %d", got.Code)
	}
}

// TestRegistrationValidation 验证非法输入在数据库写入前被拒绝。
// 输入：Go 测试上下文和真实 MySQL 测试连接配置。
// 输出：任一非法请求未返回 400 或写入账号时测试失败。
// 功能：覆盖空值、Unicode 字符长度、bcrypt 字节长度及 JSON 边界。
func TestRegistrationValidation(t *testing.T) {
	db := registrationDatabase(t)
	router := registrationRouter(t, db)
	cases := []string{
		registrationJSON("", "password"),
		registrationJSON("   ", "password"),
		registrationJSON(strings.Repeat("用", 25), "password"),
		registrationJSON("alice", ""),
		registrationJSON("alice", strings.Repeat("密", 25)),
		`{"username":"alice",`,
		`{"username":"alice","password":"ok","extra":true}`,
		`{"username":"alice","password":"ok"}{"username":"bob","password":"ok"}`,
		registrationJSON(strings.Repeat("a", 1025), "password"),
	}
	for _, body := range cases {
		assertErrorResponse(t, registerRequest(router, body), errorExpectation{
			Status: http.StatusBadRequest, Code: response.CodeInvalidArgument, RequireRequestID: true,
		})
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid requests wrote users: count=%d err=%v", count, err)
	}
}

// TestRegistrationConcurrentDuplicate 验证唯一索引处理同名并发注册。
// 输入：Go 测试上下文和真实 MySQL 测试连接配置。
// 输出：成功数或冲突数不正确时测试失败。
// 功能：确认多个请求并发时仅一个账号提交成功。
func TestRegistrationConcurrentDuplicate(t *testing.T) {
	db := registrationDatabase(t)
	router := registrationRouter(t, db)
	const requests = 8
	var group sync.WaitGroup
	results := make(chan int, requests)
	for range requests {
		group.Add(1)
		go func() {
			defer group.Done()
			results <- registerRequest(router, registrationJSON("concurrent", "password")).Code
		}()
	}
	group.Wait()
	close(results)
	created, conflicts := 0, 0
	for status := range results {
		switch status {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicts++
		default:
			t.Fatalf("unexpected concurrent registration status: %d", status)
		}
	}
	if created != 1 || conflicts != requests-1 {
		t.Fatalf("unexpected concurrent results: created=%d conflicts=%d", created, conflicts)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", "concurrent").Scan(&count); err != nil || count != 1 {
		t.Fatalf("unexpected stored account count: count=%d err=%v", count, err)
	}
}

// TestRegistrationDatabaseFailure 验证数据库故障不会伪报注册成功。
// 输入：Go 测试上下文和真实 MySQL 测试连接配置。
// 输出：关闭连接后未返回统一 500 时测试失败。
// 功能：确保数据库写入错误与用户名冲突得到不同响应。
func TestRegistrationDatabaseFailure(t *testing.T) {
	db := registrationDatabase(t)
	router := registrationRouter(t, db)
	if err := db.Close(); err != nil {
		t.Fatalf("close test connection: %v", err)
	}
	assertErrorResponse(t, registerRequest(router, registrationJSON("alice", "password")), errorExpectation{
		Status: http.StatusInternalServerError, Code: response.CodeInternal, RequireRequestID: true,
	})
}

// TestLoadMySQLRequiresConnection 验证启动配置拒绝缺失或非法参数。
// 输入：Go 测试上下文。
// 输出：配置未正确校验或读取时测试失败。
// 功能：防止数据库连接密码缺失及地址错误进入启动流程。
func TestLoadMySQLRequiresConnection(t *testing.T) {
	values := envMap{
		"MYSQL_ADDR": "127.0.0.1:3306", "MYSQL_USER": "app", "MYSQL_DATABASE": "pulseframe",
		"PULSEFRAME_MYSQL_PASSWORD": "example-password",
	}
	cfg, err := config.LoadMySQL(values.Lookup)
	if err != nil || cfg.Address != values["MYSQL_ADDR"] || cfg.Password != values["PULSEFRAME_MYSQL_PASSWORD"] {
		t.Fatalf("unexpected mysql config: err=%v", err)
	}
	for _, key := range []string{"MYSQL_ADDR", "MYSQL_USER", "MYSQL_DATABASE", "PULSEFRAME_MYSQL_PASSWORD"} {
		invalid := envMap{}
		for name, value := range values {
			invalid[name] = value
		}
		invalid[key] = ""
		if _, err := config.LoadMySQL(invalid.Lookup); err == nil {
			t.Fatalf("expected empty %s to fail", key)
		}
	}
	values["MYSQL_ADDR"] = "127.0.0.1:0"
	if _, err := config.LoadMySQL(values.Lookup); err == nil {
		t.Fatal("expected invalid MySQL port to fail")
	}
	values["MYSQL_ADDR"] = "127.0.0.1:3306"
	values["MYSQL_TLS"] = "true"
	if cfg, err := config.LoadMySQL(values.Lookup); err != nil || !cfg.TLS {
		t.Fatalf("expected mysql TLS to be enabled: %v", err)
	}
	values["MYSQL_TLS"] = "invalid"
	if _, err := config.LoadMySQL(values.Lookup); err == nil {
		t.Fatal("expected invalid MySQL TLS setting to fail")
	}
}

// TestRegistrationBoundaryPassword 验证 bcrypt 可接受的最大字节长度。
// 输入：Go 测试上下文和真实 MySQL 测试连接配置。
// 输出：72 字节密码被拒绝或存储后无法验证时测试失败。
// 功能：保护密码边界按字节计算的设计要求。
func TestRegistrationBoundaryPassword(t *testing.T) {
	db := registrationDatabase(t)
	router := registrationRouter(t, db)
	password := strings.Repeat("a", 72)
	if recorder := registerRequest(router, registrationJSON("boundary", password)); recorder.Code != http.StatusCreated {
		t.Fatalf("72-byte password rejected: %d %s", recorder.Code, recorder.Body.String())
	}
	var hash string
	if err := db.QueryRow("SELECT password_hash FROM users WHERE username = ?", "boundary").Scan(&hash); err != nil {
		t.Fatalf("load password hash: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		t.Fatalf("stored password hash does not match: %v", err)
	}
}
