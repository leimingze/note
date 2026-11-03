// 脚本功能：检查首页的品牌、状态和图片信息；例如缺少 PulseFrame 标题时测试失败。
// 启动命令：在 frontend 目录运行 npm test。
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import App from '../App.vue'

// 输入：无，由 Vitest 执行测试分组回调。
// 输出：注册首页内容测试，供测试运行器执行。
// 功能：把首页相关检查放在同一组中；例如运行 npm test 时会执行下面的基础页检查。
describe('PulseFrame foundation page', () => {
  /**
   * 输入：无。
   * 输出：基础页品牌、技术信息和图片信息缺失时使测试失败。
   * 功能：检查首页是否显示项目基础信息；例如标题应为 PulseFrame，页面不应含有账号输入表单。
   */
  function testFoundationContent(): void {
    // 将根组件挂载到测试页面中，再从渲染结果读取标题、图片和表单元素。
    const wrapper = mount(App)

    // 基础页需要明确展示项目名称和当前状态。
    expect(wrapper.get('h1').text()).toBe('PulseFrame')
    expect(wrapper.text()).toContain('工程基座')
    expect(wrapper.text()).toContain('基座可用')
    // 图片需要提供文字说明，供无法查看图片的用户了解其内容。
    expect(wrapper.get('img').attributes('alt')).toBe('创作者在山脊上记录户外景色')
    // 当前首页展示工程信息，检查页面未包含表单或输入控件。
    expect(wrapper.find('form').exists()).toBe(false)
    expect(wrapper.find('input').exists()).toBe(false)
  }

  it('展示无账号功能的工程基座页面', testFoundationContent)
})
