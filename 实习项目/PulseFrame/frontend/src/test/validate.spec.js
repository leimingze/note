import { describe, expect, it } from 'vitest'
import { validUsername, validPassword } from '../validate'

describe('validUsername', () => {
  it('接受合法用户名', () => {
    expect(validUsername('alpha_123')).toBe(true)
    expect(validUsername('abc')).toBe(true)
  })

  it('拒绝长度越界或非法字符', () => {
    expect(validUsername('ab')).toBe(false)
    expect(validUsername('a'.repeat(25))).toBe(false)
    expect(validUsername('with space')).toBe(false)
    expect(validUsername('中文名')).toBe(false)
  })
})

describe('validPassword', () => {
  it('接受满足最少字符数的密码', () => {
    expect(validPassword('x'.repeat(12))).toBe(true)
  })

  it('拒绝过短或过长密码', () => {
    expect(validPassword('x'.repeat(11))).toBe(false)
    expect(validPassword('x'.repeat(129))).toBe(false)
  })
})
