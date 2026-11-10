// 与后端一致的输入校验规则，供表单与测试复用。
export const USERNAME_MIN = 3
export const USERNAME_MAX = 24
export const PASSWORD_MIN = 12
export const PASSWORD_MAX = 128

export function validateUsername(username: string): string | null {
  if (username.length < USERNAME_MIN || username.length > USERNAME_MAX) {
    return `用户名长度须为 ${USERNAME_MIN}-${USERNAME_MAX} 个字符`
  }
  if (!/^[A-Za-z0-9_]+$/.test(username)) {
    return '用户名只能包含字母、数字或下划线'
  }
  return null
}

export function validatePassword(password: string): string | null {
  if (password.length < PASSWORD_MIN || password.length > PASSWORD_MAX) {
    return `密码长度须为 ${PASSWORD_MIN}-${PASSWORD_MAX} 个字符`
  }
  return null
}
