// 脚本功能：提供与后端一致的账号输入校验规则。
// 启动命令：由视图组件调用，无需单独启动。

const USERNAME_PATTERN = /^[A-Za-z0-9_]+$/
export const USERNAME_MIN = 3
export const USERNAME_MAX = 24
export const PASSWORD_MIN = 6
export const PASSWORD_MAX = 128

export function validUsername(username) {
  return (
    typeof username === 'string' &&
    username.length >= USERNAME_MIN &&
    username.length <= USERNAME_MAX &&
    USERNAME_PATTERN.test(username)
  )
}

export function validPassword(password) {
  return (
    typeof password === 'string' &&
    password.length >= PASSWORD_MIN &&
    password.length <= PASSWORD_MAX
  )
}
