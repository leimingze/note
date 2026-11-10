import { describe, expect, it } from 'vitest'
import { validatePassword, validateUsername } from '@/utils/validate'

describe('validateUsername', () => {
  it('接受合法用户名', () => {
    expect(validateUsername('abc_123')).toBeNull()
  })

  it('拒绝长度不足', () => {
    expect(validateUsername('ab')).not.toBeNull()
  })

  it('拒绝非法字符', () => {
    expect(validateUsername('ab cd')).not.toBeNull()
    expect(validateUsername('你好')).not.toBeNull()
  })
})

describe('validatePassword', () => {
  it('接受合法密码', () => {
    expect(validatePassword('123456789012')).toBeNull()
  })

  it('拒绝长度不足', () => {
    expect(validatePassword('short')).not.toBeNull()
  })
})
