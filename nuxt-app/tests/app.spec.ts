import { mount } from '@vue/test-utils'
import { describe, it, expect } from 'vitest'
import App from '../app.vue'

describe('App', () => {
  it('renders a heading', () => {
    const wrapper = mount(App)
    const heading = wrapper.find('h1')
    expect(heading.exists()).toBe(true)
    expect(heading.text()).toBe('Hello, Nuxt.js!')
  })
})
