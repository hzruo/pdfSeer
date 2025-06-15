import { defineUserConfig } from 'vuepress'
import { defaultTheme } from '@vuepress/theme-default'
import { viteBundler } from '@vuepress/bundler-vite'

export default defineUserConfig({
  // 站点配置
  lang: 'zh-CN',
  title: '识文君',
  description: 'PDF智能识别助手 - 让文字识别更智能、更高效',
  
  // 基础路径
  base: '/',
  
  // 头部配置
  head: [
    ['link', { rel: 'icon', href: '/favicon.ico' }],
    ['meta', { name: 'viewport', content: 'width=device-width,initial-scale=1,user-scalable=no' }],
    ['meta', { name: 'keywords', content: 'PDF,OCR,AI,文字识别,智能助手' }]
  ],

  // 主题配置
  theme: defaultTheme({
    // 导航栏
    navbar: [
      {
        text: '首页',
        link: '/'
      },
      {
        text: '指南',
        children: [
          {
            text: '快速开始',
            link: '/guide/getting-started'
          },
          {
            text: '安装说明',
            link: '/guide/installation'
          },
          {
            text: 'AI配置',
            link: '/guide/ai-config'
          }
        ]
      },
      {
        text: '使用教程',
        children: [
          {
            text: '基础使用',
            link: '/tutorial/basic-usage'
          },
          {
            text: '视频教程',
            link: '/tutorial/video-tutorials'
          }
        ]
      },
      {
        text: '常见问题',
        link: '/faq/'
      },
      {
        text: '关于',
        children: [
          {
            text: '关于识文君',
            link: '/about/'
          },
          {
            text: '许可协议',
            link: '/about/license'
          }
        ]
      }
    ],

    // 侧边栏
    sidebar: {
      '/guide/': [
        {
          text: '指南',
          children: [
            '/guide/getting-started.md',
            '/guide/installation.md',
            '/guide/ai-config.md',
            '/guide/system-requirements.md'
          ]
        }
      ],
      '/tutorial/': [
        {
          text: '使用教程',
          children: [
            '/tutorial/basic-usage.md',
            '/tutorial/video-tutorials.md'
          ]
        }
      ],
      '/faq/': [
        {
          text: '常见问题',
          children: [
            '/faq/README.md'
          ]
        }
      ],
      '/about/': [
        {
          text: '关于',
          children: [
            '/about/README.md',
            '/about/license.md'
          ]
        }
      ]
    },

    // 仓库配置
    repo: 'https://github.com/hzruo/pdfSeer',
    repoLabel: 'GitHub',
    
    // 编辑链接
    editLink: false,
    
    // 最后更新时间
    lastUpdated: true,
    lastUpdatedText: '最后更新',

    // 贡献者
    contributors: true,
    contributorsText: '贡献者'
  }),

  // 打包工具
  bundler: viteBundler({
    viteOptions: {},
    vuePluginOptions: {}
  }),

  // 插件
  plugins: []
})
