package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id 哈希参数默认值，兼顾内存开销与爆破抵抗。
const (
	defaultMemory    = 19456 // 内存成本（KiB），约19MiB。
	defaultTime      = 2     // 迭代次数。
	defaultThreads   = 1     // 并行度，单核即满足默认场景。
	defaultKeyLength = 32    // 派生密钥长度（字节）。
	defaultSaltLength = 16   // 随机盐长度（字节）。
)

// Params 保存 Argon2id 哈希使用的内存和时间参数。
type Params struct {
	Memory     uint32 // 内存成本（KiB）。
	Time       uint32 // 迭代次数。
	Threads    uint8  // 并行度。
	KeyLength  uint32 // 派生密钥长度（字节）。
	SaltLength uint32 // 随机盐长度（字节）。
}

// DefaultParams 返回推荐使用的 Argon2id 参数。
// 输入：无。
// 输出：包含默认内存、迭代、并行度与长度参数的 Params。
// 功能：提供可直接用于生成和验证哈希的参数。
// 例如注册时调用返回的 Params 生成密码哈希。
func DefaultParams() Params {
	return Params{
		Memory:     defaultMemory,
		Time:       defaultTime,
		Threads:    defaultThreads,
		KeyLength:  defaultKeyLength,
		SaltLength: defaultSaltLength,
	}
}

// Hash 使用 Argon2id 生成密码哈希。
// 输入：plaintext，待哈希的密码字节；p，哈希参数。
// 输出：PHC 格式的哈希字符串；随机源不可用时返回错误。
// 功能：结合随机盐派生密钥，输出可直接存入数据库的完整哈希。
// 例如对任意密码调用后可得到 $argon2id$v=19$m=19456,t=2,p=1$... 格式字符串。
func Hash(plaintext []byte, p Params) (string, error) {
	salt := make([]byte, p.SaltLength) // 分配随机盐
	if _, err := rand.Read(salt); err != nil { // 读取安全随机盐；失败时返回错误
		return "", err
	}
	key := argon2.IDKey(plaintext, salt, p.Time, p.Memory, p.Threads, p.KeyLength) // 派生密码密钥
	encode := func(value []byte) string { // 无填充 base64url 编码辅助
		return base64.RawStdEncoding.EncodeToString(value)
	}
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Time, p.Threads, encode(salt), encode(key)), nil
}

// Verify 验证明文密码是否与哈希匹配。
// 输入：hash，PHC 格式哈希；plaintext，待验证密码字节。
// 输出：匹配时返回 true；哈希格式非法时返回错误。
// 功能：从哈希中还原参数与盐，重新派生密钥做常量时间比较。
// 例如哈希对应正确密码时返回 true，错误密码返回 false。
func Verify(hash string, plaintext []byte) (bool, error) {
	memory, timeCost, threads, salt, key, err := decodeHash(hash) // 解析哈希组成
	if err != nil {
		return false, err
	}
	recomputed := argon2.IDKey(plaintext, salt, timeCost, memory, threads, uint32(len(key))) // 按原参数重新派生密钥
	return subtle.ConstantTimeCompare(key, recomputed) == 1, nil // 常量时间比较避免时序侧信道
}

// errInvalidHash 表示哈希格式无法解析。
var errInvalidHash = errors.New("invalid argon2id hash")

// decodeHash 解析 PHC 格式的 Argon2id 哈希。
// 输入：hash，待解析哈希字符串。
// 输出：内存、迭代、并行度、盐、密钥；格式非法时返回错误。
// 功能：按 $argon2id$v=...$m=...,t=...,p=...$salt$key 结构拆解哈希。
// 例如合法哈希返回对应参数与字节数据。
func decodeHash(hash string) (uint32, uint32, uint8, []byte, []byte, error) {
	parts := strings.Split(hash, "$") // 按 $ 拆分哈希段
	if len(parts) != 6 || parts[1] != "argon2id" { // 段数或算法标识不合法
		return 0, 0, 0, nil, nil, errInvalidHash
	}
	var memory, timeCost uint64 // 解析参数值
	var threads uint64
	for _, item := range strings.Split(parts[3], ",") { // 逐个解析 m/t/p 参数
		pair := strings.SplitN(item, "=", 2) // 拆分为 名=值
		if len(pair) != 2 {
			return 0, 0, 0, nil, nil, errInvalidHash
		}
		value, err := strconv.ParseUint(pair[1], 10, 32) // 解析数值参数
		if err != nil {
			return 0, 0, 0, nil, nil, errInvalidHash
		}
		switch pair[0] {
		case "m":
			memory = value
		case "t":
			timeCost = value
		case "p":
			threads = value
		default:
			return 0, 0, 0, nil, nil, errInvalidHash
		}
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4]) // 解码盐
	if err != nil {
		return 0, 0, 0, nil, nil, errInvalidHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5]) // 解码派生密钥
	if err != nil {
		return 0, 0, 0, nil, nil, errInvalidHash
	}
	return uint32(memory), uint32(timeCost), uint8(threads), salt, key, nil
}
