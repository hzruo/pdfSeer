# 识文君文档

这是识文君 PDF智能识别助手的官方文档，使用 VuePress 构建。

## 📖 文档结构

```
docs/
├── .vuepress/          # VuePress 配置
│   ├── config.ts       # 主配置文件
│   └── public/         # 静态资源目录
│       └── logo.png    # 首页 logo 图片
├── guide/              # 使用指南
│   ├── getting-started.md    # 快速开始
│   ├── installation.md      # 安装说明
│   ├── ai-config.md         # AI配置
│   └── system-requirements.md # 系统要求
├── tutorial/           # 使用教程
│   ├── basic-usage.md       # 基础使用
│   └── video-tutorials.md   # 视频教程
├── faq/               # 常见问题
│   └── README.md           # 常见问题首页
├── about/             # 关于
│   ├── README.md           # 关于识文君
│   └── license.md         # 许可协议
├── README.md          # 文档首页
└── 图片放置说明.md     # 图片配置说明
```

## 🚀 快速开始

### 安装依赖

```bash
cd docs
npm install
```

### 开发模式

```bash
# 启动开发服务器
npm run dev
# 或使用脚本
./dev-docs.sh
```

文档将在 http://localhost:8080 打开，支持热重载。

### 构建文档

```bash
# 构建静态文件
npm run build
# 或使用脚本
./build-docs.sh
```

构建输出在 `.vuepress/dist/` 目录。

### 部署到 GitHub Pages

```bash
# 构建并部署
./build-docs.sh --deploy
```

## 📝 编写文档

### Markdown 语法

VuePress 支持标准 Markdown 语法，以及一些扩展功能：

#### 提示框

```markdown
::: tip 提示
这是一个提示框
:::

::: warning 警告
这是一个警告框
:::

::: danger 危险
这是一个危险提示框
:::
```

#### 代码块

```markdown
```bash
# 这是一个 bash 代码块
npm install
```
```

#### 表格

```markdown
| 列1 | 列2 | 列3 |
|-----|-----|-----|
| 内容1 | 内容2 | 内容3 |
```

#### 链接

```markdown
# 内部链接
[快速开始](/guide/getting-started)

# 外部链接
[GitHub](https://github.com/hzruo/pdfSeer)
```

### Front Matter

每个 Markdown 文件可以包含 Front Matter：

```yaml
---
title: 页面标题
description: 页面描述
---
```

### 导航配置

在 `.vuepress/config.ts` 中配置导航栏和侧边栏：

```typescript
// 导航栏
navbar: [
  {
    text: '指南',
    children: [
      { text: '快速开始', link: '/guide/getting-started' },
      { text: '安装说明', link: '/guide/installation' },
      { text: 'AI配置', link: '/guide/ai-config' }
    ]
  },
  {
    text: '使用教程',
    children: [
      { text: '基础使用', link: '/tutorial/basic-usage' },
      { text: '视频教程', link: '/tutorial/video-tutorials' }
    ]
  },
  {
    text: '常见问题',
    link: '/faq/'
  },
  {
    text: '关于',
    children: [
      { text: '关于识文君', link: '/about/' },
      { text: '许可协议', link: '/about/license' }
    ]
  }
]

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
  '/about/': [
    {
      text: '关于',
      children: [
        '/about/README.md',
        '/about/license.md'
      ]
    }
  ]
}
```

## 🎨 自定义样式

### 主题配置

在 `.vuepress/config.ts` 中可以自定义主题：

```typescript
theme: defaultTheme({
  // 颜色主题
  colorMode: 'auto',
  colorModeSwitch: true,
  
  // 仓库配置
  repo: 'https://github.com/hzruo/pdfSeer',
  
  // 编辑链接
  editLink: false,
  
  // 最后更新时间
  lastUpdated: true
})
```

### 静态资源配置

**图片配置**:
将静态资源放在 `.vuepress/public/` 目录下：

```
.vuepress/public/
├── logo.png          # 首页 logo
├── favicon.ico       # 网站图标
└── images/           # 其他图片
```

在文档中引用：
```markdown
![Logo](/logo.png)
```

**首页配置**:
```yaml
---
heroImage: /logo.png  # 对应 .vuepress/public/logo.png
---
```

### 自定义CSS

创建 `.vuepress/styles/index.scss` 文件添加自定义样式：

```scss
// 自定义颜色
:root {
  --c-brand: #667eea;
  --c-brand-light: #764ba2;
}

// 自定义样式
.custom-class {
  color: var(--c-brand);
}
```

## 📦 部署选项

### GitHub Pages

1. 在 GitHub 仓库设置中启用 Pages
2. 选择 `gh-pages` 分支作为源
3. 运行 `./build-docs.sh --deploy` 部署

### Netlify

1. 连接 GitHub 仓库到 Netlify
2. 设置构建命令: `cd docs && npm run build`
3. 设置发布目录: `docs/.vuepress/dist`

### Vercel

1. 导入 GitHub 仓库到 Vercel
2. 设置根目录: `docs`
3. 构建命令: `npm run build`
4. 输出目录: `.vuepress/dist`

## 🔧 开发工具

### VS Code 扩展

推荐安装以下扩展：

- **Markdown All in One**: Markdown 编辑支持
- **markdownlint**: Markdown 语法检查
- **Prettier**: 代码格式化
- **Vue Language Features**: Vue 语法支持

### 配置文件

创建 `.vscode/settings.json`：

```json
{
  "markdown.preview.breaks": true,
  "markdown.preview.linkify": true,
  "[markdown]": {
    "editor.defaultFormatter": "esbenp.prettier-vscode",
    "editor.wordWrap": "on"
  }
}
```

## 📋 维护清单

### 定期更新

- [ ] 检查依赖包更新 (VuePress、插件等)
- [ ] 更新文档内容 (新功能、变更说明)
- [ ] 检查链接有效性 (内部链接、外部链接)
- [ ] 更新视频教程链接
- [ ] 检查图片资源是否正常显示

### 内容审核

- [ ] 检查拼写和语法
- [ ] 验证配置示例和代码
- [ ] 确保软件功能描述准确
- [ ] 更新版本信息和下载链接
- [ ] 验证依赖安装步骤

## 🤝 贡献指南

### 提交文档更改

1. Fork 仓库
2. 创建功能分支
3. 编写或修改文档
4. 本地测试文档
5. 提交 Pull Request

### 文档特色

- **内容完整**: 涵盖从安装到高级使用的全部内容
- **结构清晰**: 按功能模块组织，便于查找
- **实用导向**: 重点介绍实际操作方法和解决方案
- **视频教程**: 提供直观的视频演示教程
- **用户友好**: 面向普通用户，语言通俗易懂

### 文档规范

- 使用清晰的标题层次
- 提供代码示例和配置说明
- 添加适当的提示框和警告
- 保持内容简洁明了
- 使用统一的术语和格式

## 📞 获取帮助

如果在文档开发过程中遇到问题：

- 📖 查看 [VuePress 官方文档](https://v2.vuepress.vuejs.org/)
- 📧 联系文档维护者: [hzruo@outlook.com](mailto:hzruo@outlook.com)
- 🐛 在 GitHub Issues 中报告问题

---

感谢您为识文君文档做出贡献！
