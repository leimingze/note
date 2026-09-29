package secure

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"

	"pulseframe/internal/modules/account/application"
)

const (
	argonVersion           = 19
	argonMemoryKiB         = 19 * 1024
	argonIterations        = 2
	argonParallelism       = 1
	argonSaltBytes         = 16
	argonKeyBytes          = 32
	maximumMemoryKiB       = argonMemoryKiB
	maximumTimeCost        = argonIterations
	maximumThreads         = argonParallelism
	maximumHashConcurrency = 8
)

// Argon2id 保存经过容量基准校准的密码哈希参数。
type Argon2id struct {
	memoryKiB   uint32
	iterations  uint32
	parallelism uint8
	active      chan struct{}
}

// NewArgon2id 创建受并发上限保护的 Argon2id 实现。
// 输入：concurrency，允许同时执行的密码哈希数，范围为 1 至 maximumHashConcurrency。
// 输出：可生成和验证 Argon2id 摘要的实现；参数非法时返回错误。
// 功能：用内存困难型哈希保护已泄漏密码库，并限制在线请求的 CPU 与内存占用。
func NewArgon2id(concurrency int) (*Argon2id, error) {
	if concurrency < 1 || concurrency > maximumHashConcurrency {
		return nil, fmt.Errorf("password hash concurrency must be between 1 and %d", maximumHashConcurrency)
	}
	return &Argon2id{
		memoryKiB: argonMemoryKiB, iterations: argonIterations, parallelism: argonParallelism,
		active: make(chan struct{}, concurrency),
	}, nil
}

// Hash 为原始密码生成随机盐和自描述 Argon2id 摘要。
// 输入：password，未经裁剪或改写的有效 UTF-8 密码。
// 输出：包含算法、参数、盐和摘要的编码；容量耗尽或随机源失败时返回错误。
// 功能：让后续验证按记录参数计算摘要，并将并发资源控制在配置上限内。
func (hasher *Argon2id) Hash(password string) (string, error) {
	if err := hasher.acquire(); err != nil {
		return "", err
	}
	defer hasher.release()
	salt := make([]byte, argonSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	derived := argon2.IDKey([]byte(password), salt, hasher.iterations, hasher.memoryKiB, hasher.parallelism, argonKeyBytes)
	encoded := encodeHash(hasher, salt, derived)
	clear(derived)
	return encoded, nil
}

// Verify 按摘要内参数验证密码并使用常量时间比较。
// 输入：password，原始密码；encoded，存储的 Argon2id PHC 格式摘要。
// 输出：密码匹配返回 nil；摘要损坏、密码不匹配或容量耗尽时返回错误。
// 功能：验证时限制可接受的存储参数，避免损坏记录触发无界资源消耗。
func (hasher *Argon2id) Verify(password string, encoded string) error {
	parameters, salt, expected, err := parseHash(encoded)
	if err != nil {
		return err
	}
	if err := hasher.acquire(); err != nil {
		return err
	}
	defer hasher.release()
	actual := argon2.IDKey([]byte(password), salt, parameters.iterations, parameters.memoryKiB, parameters.parallelism, uint32(len(expected)))
	matched := subtle.ConstantTimeCompare(actual, expected) == 1
	clear(actual)
	if !matched {
		return fmt.Errorf("password does not match")
	}
	return nil
}

// acquire 在不排队的情况下占用一个 Argon2id 计算槽位。
// 输入：hasher，已通过构造器创建的哈希器。
// 输出：有空闲槽位时返回 nil；容量耗尽时返回 application.ErrPasswordHashCapacity。
// 功能：将并发内存成本限制在配置上限内，避免请求堆积形成额外资源压力。
func (hasher *Argon2id) acquire() error {
	select {
	case hasher.active <- struct{}{}:
		return nil
	default:
		return application.ErrPasswordHashCapacity
	}
}

// release 归还已占用的 Argon2id 计算槽位。
// 输入：hasher，当前持有一个活动槽位的哈希器。
// 输出：释放一个槽位，无返回值。
// 功能：保证哈希完成或失败后其他请求可继续使用受限计算容量。
func (hasher *Argon2id) release() {
	<-hasher.active
}

// encodeHash 序列化 Argon2id 参数、盐和摘要。
// 输入：hasher，实际计算参数；salt，随机盐；derived，派生摘要。
// 输出：可持久化的 PHC 风格字符串。
// 功能：保存后续验证与升级所需的算法元数据。
func encodeHash(hasher *Argon2id, salt []byte, derived []byte) string {
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argonVersion, hasher.memoryKiB, hasher.iterations, hasher.parallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(derived))
}

type parameters struct {
	memoryKiB   uint32
	iterations  uint32
	parallelism uint8
}

// parseHash 解析并限制数据库中的 Argon2id 摘要参数。
// 输入：encoded，完整的 PHC 风格摘要。
// 输出：受限参数、盐和预期摘要；格式非法或资源参数过大时返回错误。
// 功能：防止错误或被篡改的数据库内容驱动昂贵的任意哈希参数。
func parseHash(encoded string) (parameters, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return parameters{}, nil, nil, fmt.Errorf("invalid Argon2id hash format")
	}
	parsed, err := parseParameters(parts[3])
	if err != nil {
		return parameters{}, nil, nil, err
	}
	salt, saltErr := base64.RawStdEncoding.DecodeString(parts[4])
	expected, hashErr := base64.RawStdEncoding.DecodeString(parts[5])
	if saltErr != nil || hashErr != nil || len(salt) < argonSaltBytes || len(salt) > 64 ||
		len(expected) < 16 || len(expected) > 64 {
		return parameters{}, nil, nil, fmt.Errorf("invalid Argon2id salt or digest")
	}
	return parsed, salt, expected, nil
}

// parseParameters 解析 Argon2id 参数并执行资源上限校验。
// 输入：raw，m=内存、t=迭代次数、p=并行度格式的参数串。
// 输出：合法的计算参数；未知格式、零值或超限时返回错误。
// 功能：限制密码记录损坏时单次验证可能消耗的 CPU 和内存。
func parseParameters(raw string) (parameters, error) {
	parts := strings.Split(raw, ",")
	if len(parts) != 3 {
		return parameters{}, fmt.Errorf("invalid Argon2id parameters")
	}
	memory, memoryErr := parseUintParameter(parts[0], "m=")
	iterations, iterationsErr := parseUintParameter(parts[1], "t=")
	parallelism, parallelismErr := parseUintParameter(parts[2], "p=")
	if memoryErr != nil || iterationsErr != nil || parallelismErr != nil || memory < argonMemoryKiB ||
		memory > maximumMemoryKiB || iterations < argonIterations || iterations > maximumTimeCost ||
		parallelism == 0 || parallelism > maximumThreads {
		return parameters{}, fmt.Errorf("Argon2id parameters exceed supported limits")
	}
	return parameters{uint32(memory), uint32(iterations), uint8(parallelism)}, nil
}

// parseUintParameter 解析带固定名称前缀的正整数参数。
// 输入：raw，形如 m=19456 的单项参数；prefix，必须匹配的名称。
// 输出：无符号整数；前缀错误、溢出或格式非法时返回错误。
// 功能：拒绝参数重排或额外文本，避免宽松解析产生歧义。
func parseUintParameter(raw string, prefix string) (uint64, error) {
	if !strings.HasPrefix(raw, prefix) {
		return 0, fmt.Errorf("invalid Argon2id parameter")
	}
	value, err := strconv.ParseUint(strings.TrimPrefix(raw, prefix), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid Argon2id parameter: %w", err)
	}
	return value, nil
}
