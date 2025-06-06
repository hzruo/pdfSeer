import { marked } from 'marked'

interface MarkdownOptions {
  breaks?: boolean
  gfm?: boolean
  headerIds?: boolean
  mangle?: boolean
  sanitize?: boolean
  smartLists?: boolean
  smartypants?: boolean
  xhtml?: boolean
}

/**
 * 配置并渲染Markdown文本
 * @param text 要渲染的Markdown文本
 * @param options 可选的配置选项
 * @returns 渲染后的HTML字符串
 */
export function renderMarkdown(text: string, options: MarkdownOptions = {}): string {
  if (!text) {
    return ''
  }

  try {
    // 设置默认配置
    const defaultOptions: MarkdownOptions = {
      breaks: true,        // 启用换行转换，保持文本格式
      gfm: true,           // 启用GitHub风格的Markdown
      headerIds: false,    // 禁用标题ID生成
      mangle: false,       // 禁用邮箱地址混淆
      sanitize: false,     // 禁用HTML清理（我们信任输入）
      smartLists: true,    // 启用智能列表
      smartypants: false,  // 禁用智能标点符号转换
      xhtml: false         // 使用HTML5而不是XHTML
    }

    // 合并用户配置
    const finalOptions = { ...defaultOptions, ...options }

    // 配置marked选项
    marked.setOptions(finalOptions)

    // 预处理文本：优化换行处理
    let processedText = text

    // 更温和的换行处理：只处理明显的段落分隔
    processedText = processedText
      .replace(/\n{3,}/g, '\n\n')             // 多个换行符合并为双换行

    // 使用marked.parse来确保同步解析
    const result = marked.parse(processedText)
    return result as string
  } catch (error) {
    console.error('Markdown渲染失败:', error)
    return text
  }
}

/**
 * 预处理文本，优化换行和格式
 * @param text 原始文本
 * @returns 处理后的文本
 */
export function preprocessText(text: string): string {
  if (!text) {
    return ''
  }

  return text
    // 统一换行符
    .replace(/\r\n/g, '\n')
    .replace(/\r/g, '\n')
    // 清理多余的空白字符
    .replace(/[ \t]+$/gm, '')  // 移除行尾空白
    .replace(/^\s*\n/gm, '\n') // 移除只有空白的行
    // 优化段落间距
    .replace(/\n{4,}/g, '\n\n\n') // 限制最多3个连续换行
    .trim()
}

/**
 * 检测文本是否包含Markdown格式
 * @param text 要检测的文本
 * @returns 是否包含Markdown格式
 */
export function hasMarkdownSyntax(text: string): boolean {
  if (!text) {
    return false
  }

  const markdownPatterns = [
    /^#{1,6}\s+/m,           // 标题
    /\*\*.*?\*\*/,           // 粗体
    /\*.*?\*/,               // 斜体
    /`.*?`/,                 // 行内代码
    /```[\s\S]*?```/,        // 代码块
    /^\s*[-*+]\s+/m,         // 无序列表
    /^\s*\d+\.\s+/m,         // 有序列表
    /^\s*>\s+/m,             // 引用
    /\[.*?\]\(.*?\)/,        // 链接
    /!\[.*?\]\(.*?\)/,       // 图片
    /^\s*\|.*\|.*\|/m,       // 表格
    /^---+$/m,               // 分隔线
  ]

  return markdownPatterns.some(pattern => pattern.test(text))
}

/**
 * 转义HTML特殊字符
 * @param text 要转义的文本
 * @returns 转义后的文本
 */
export function escapeHtml(text: string): string {
  if (!text) {
    return ''
  }

  const htmlEscapes: Record<string, string> = {
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;'
  }

  return text.replace(/[&<>"']/g, (match) => htmlEscapes[match])
}

/**
 * 智能渲染文本：如果包含Markdown语法则渲染，否则保持原样
 * @param text 要渲染的文本
 * @param forceMarkdown 是否强制使用Markdown渲染
 * @returns 渲染后的HTML字符串或原始文本
 */
export function smartRender(text: string, forceMarkdown = false): string {
  if (!text) {
    return ''
  }

  // 预处理文本
  const processedText = preprocessText(text)

  // 如果强制使用Markdown或检测到Markdown语法，则渲染
  if (forceMarkdown || hasMarkdownSyntax(processedText)) {
    try {
      return renderMarkdown(processedText)
    } catch (error) {
      console.error('智能渲染失败，回退到纯文本:', error)
      return escapeHtml(processedText).replace(/\n/g, '<br>')
    }
  }

  // 否则返回转义后的纯文本（保持换行）
  return escapeHtml(processedText).replace(/\n/g, '<br>')
}
