import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import App from '../App.vue'

describe('PulseFrame foundation page', () => {
  /**
   * 输入：无。
   * 输出：基础页品牌、技术信息和图片信息缺失时使测试失败。
   * 功能：确认网页只呈现工程基座内容，不再提供账号入口。
   */
  function testFoundationContent(): void {
    const wrapper = mount(App)

    // 基础页需要明确展示项目名称和当前状态。
    expect(wrapper.get('h1').text()).toBe('PulseFrame')
    expect(wrapper.text()).toContain('工程基座')
    expect(wrapper.text()).toContain('基座可用')
    // 视觉素材继续保留，并提供可访问的替代文本。
    expect(wrapper.get('img').attributes('alt')).toBe('创作者在山脊上记录户外景色')
    // 账号表单已经移除，页面不应再含输入控件。
    expect(wrapper.find('form').exists()).toBe(false)
    expect(wrapper.find('input').exists()).toBe(false)
  }

  it('展示无账号功能的工程基座页面', testFoundationContent)
})
