package session

import "github.com/redis/go-redis/v9"

// createScript 原子创建会话与刷新令牌映射。
// 返回 1 表示成功；会话已存在且 key 不同时覆盖，返回 1 仍成功。
var createScript = redis.NewScript(`
local session_key = KEYS[1]
local refresh_key = KEYS[2]
local user_id = ARGV[1]
local auth_version = ARGV[2]
local token_digest = ARGV[3]
local absolute_expires_at = ARGV[4]
redis.call('HSET', session_key, 'userId', user_id, 'authVersion', auth_version, 'currentTokenDigest', token_digest, 'absoluteExpiresAt', absolute_expires_at)
redis.call('EXPIREAT', session_key, absolute_expires_at)
redis.call('SET', refresh_key, string.sub(session_key, 9))
redis.call('EXPIREAT', refresh_key, absolute_expires_at)
return 1
`)

// rotateScript 原子校验并轮换刷新令牌。
// 返回 0 表示会话或映射缺失；1 表示账号版本不一致；2 表示摘要不匹配；3 表示成功。
var rotateScript = redis.NewScript(`
local session_key = KEYS[1]
local old_refresh_key = KEYS[2]
local new_refresh_key = KEYS[3]
local auth_version = ARGV[1]
local absolute_expires_at = ARGV[2]
local new_digest = ARGV[3]
local session_id = string.sub(session_key, 9)
local old_digest = string.sub(old_refresh_key, 9)
if redis.call('EXISTS', session_key) == 0 then
  return 0
end
if redis.call('GET', old_refresh_key) ~= session_id then
  return 0
end
if redis.call('HGET', session_key, 'authVersion') ~= auth_version then
  return 1
end
local current_digest = redis.call('HGET', session_key, 'currentTokenDigest')
if current_digest ~= old_digest then
  return 2
end
redis.call('DEL', old_refresh_key)
redis.call('HSET', session_key, 'currentTokenDigest', new_digest)
redis.call('SET', new_refresh_key, session_id)
redis.call('EXPIREAT', new_refresh_key, absolute_expires_at)
redis.call('EXPIREAT', session_key, absolute_expires_at)
return 3
`)

// deleteScript 原子删除当前刷新令牌映射与会话。
// 返回 1 表示已匹配并删除；0 表示摘要不匹配。
var deleteScript = redis.NewScript(`
local session_key = KEYS[1]
local refresh_key = KEYS[2]
local session_id = string.sub(session_key, 9)
if redis.call('EXISTS', refresh_key) == 0 then
  return 1
end
if redis.call('GET', refresh_key) ~= session_id then
  return 0
end
redis.call('DEL', refresh_key)
redis.call('DEL', session_key)
return 1
`)
