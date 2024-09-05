import markDownPlugin from 'vitepress-demo-editor/markdownPlugin'

import { UserConfig } from 'vitepress'
import sidebar from './sidebar.ts'

const nav = [
  { text: '组件文档', link: '/quick-start/', target: '_self' },
  // { text: 'playground', link: '/playground/' },
  {
    text: 'Github',
    link: 'https://github.com/qor5/x/tree/master/ui/vuetifyx/vuetifyxjs',
    target: '_blank',
    rel: ''
  }
]

const config: UserConfig = {
  themeConfig: {
    sidebar,
    nav,
    search: true,
  },

  title: 'VuetifyX UI',
  lang: 'zh-CN',
  description: '一个基于vuetify的企业级组件库',
  markdown: {
    config: (md) => {
      md.use(markDownPlugin, {})
    }
  }
}

export default config
