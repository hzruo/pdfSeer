# 系统要求

本页面详细说明识文君的系统要求和依赖配置。

## 💻 硬件要求

### 最低配置

- **CPU**: 双核 2.0GHz 或更高
- **内存**: 4GB RAM
- **存储**: 500MB 可用空间
- **网络**: 互联网连接（用于AI服务）

### 推荐配置

- **CPU**: 四核 2.5GHz 或更高
- **内存**: 8GB RAM 或更高
- **存储**: 1GB 可用空间（SSD推荐）
- **网络**: 稳定的宽带连接

## 🖥️ 操作系统支持

### macOS

- **版本**: macOS 10.15 (Catalina) 或更高
- **架构**: Intel x64 和 Apple Silicon (M1/M2) 
- **依赖**: Xcode Command Line Tools

### Windows

- **版本**: Windows 10 (1903) 或更高
- **架构**: x64 (64位)
- **依赖**: Visual C++ Redistributable

### Linux

- **发行版**: Ubuntu 18.04+, CentOS 7+, Fedora 30+
- **架构**: x64 (64位)
- **桌面环境**: GNOME, KDE, XFCE 等

## 📦 核心依赖

### libvips (必需)

libvips 是识文君的核心依赖，用于PDF页面渲染和图像处理。

**macOS 安装**:
```bash
# 使用 Homebrew
brew install vips

# 使用 MacPorts
sudo port install vips
```

**Windows 安装**:

**下载地址**:
- GitHub官方: [vips-dev-w64-all-8.12.2.zip](https://github.com/libvips/build-win64-mxe/releases/download/v8.12.2/vips-dev-w64-all-8.12.2.zip)
- 网盘下载: [TeraCloud网盘](https://zeze.teracloud.jp/share/1271ba0bfb2d3b46) (免登录下载)

**安装步骤**:
1. 下载 `vips-dev-w64-all-8.12.2.zip` 文件
2. 解压到D盘根目录，得到文件夹 `D:\vips-dev-w64-all-8.12.2`
3. 配置系统环境变量:
   - 右键"此电脑" → "属性" → "高级系统设置"
   - 点击"环境变量"按钮
   - 在"系统变量"中找到"Path"，点击"编辑"
   - 点击"新建"，添加路径: `D:\vips-dev-w64-all-8.12.2\bin`
   - 点击"确定"保存所有设置
4. 重启命令提示符或重启电脑
5. 验证安装: 打开命令提示符，输入 `vips --version`

::: warning 重要提醒
必须先配置好 libvips 依赖再运行识文君软件，否则软件无法正常启动。
:::

**Linux 安装**:
```bash
# Ubuntu/Debian
sudo apt-get install libvips-dev

# CentOS/RHEL
sudo yum install vips-devel

# Fedora
sudo dnf install vips-devel

# Arch Linux
sudo pacman -S libvips
```

### WebKit (Linux)

Linux 版本需要 WebKit2GTK 支持：

```bash
# Ubuntu/Debian
sudo apt-get install libwebkit2gtk-4.0-dev

# CentOS/RHEL
sudo yum install webkit2gtk3-devel

# Fedora
sudo dnf install webkit2gtk3-devel
```

## 🌐 网络要求

### AI服务连接

- **OpenAI**: 需要访问 `api.openai.com`
- **Google Gemini**: 需要访问 `generativelanguage.googleapis.com`
- **Pollinations**: 需要访问 `text.pollinations.ai`

### 防火墙配置

确保以下端口可以访问：
- **HTTPS (443)**: AI服务API调用
- **HTTP (80)**: 某些服务的重定向

### 代理支持

识文君支持系统代理设置，如果您在企业网络环境中：
1. 配置系统代理
2. 确保代理支持HTTPS
3. 检查代理是否允许AI服务域名

## 🔧 开发环境要求

如果您需要从源码构建识文君：

### Go 环境

- **版本**: Go 1.23 或更高
- **模块**: 启用 Go Modules

### Node.js 环境

- **版本**: Node.js 18 或更高
- **包管理**: npm 或 yarn

### Wails 框架

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### 构建工具

**macOS**:
- Xcode Command Line Tools
- Homebrew (推荐)

**Windows**:
- Visual Studio Build Tools
- NSIS (用于安装包)

**Linux**:
- GCC 编译器
- pkg-config
- 相关开发库

## 📊 性能考虑

### 内存使用

- **基础运行**: 约 100-200MB
- **处理PDF**: 额外 50-100MB 每页
- **AI处理**: 额外 100-300MB

### 存储空间

- **应用本体**: 约 50-100MB
- **依赖库**: 约 100-200MB
- **缓存数据**: 约 10-50MB

### 网络带宽

- **OCR识别**: 约 1-5MB 每页（图像上传）
- **文本处理**: 约 1-10KB 每次请求
- **模型下载**: 某些本地模型可能需要GB级下载

## 🔍 兼容性测试

### 已测试平台

| 平台 | 版本 | 状态 | 备注 |
|------|------|------|------|
| macOS Intel | 10.15-13.x | ✅ 支持 | 完全兼容 |
| macOS Apple Silicon | 11.0-13.x | ✅ 支持 | 原生支持 |
| Windows 10 | 1903+ | ✅ 支持 | 完全兼容 |
| Windows 11 | 所有版本 | ✅ 支持 | 完全兼容 |
| Ubuntu | 18.04-22.04 | ✅ 支持 | 完全兼容 |
| CentOS | 7-8 | ✅ 支持 | 完全兼容 |
| Fedora | 30+ | ✅ 支持 | 完全兼容 |

### 已知问题

- **Ubuntu 16.04**: WebKit版本过旧，不支持
- **Windows 7**: 系统版本过旧，不支持
- **32位系统**: 不支持，仅支持64位

## 🚨 故障排除

### 依赖检测失败

如果应用提示依赖缺失：

1. **重新安装依赖**
   ```bash
   # macOS
   brew reinstall vips
   
   # Linux
   sudo apt-get install --reinstall libvips-dev
   ```

2. **检查环境变量**
   ```bash
   # 检查 libvips 是否在 PATH 中
   which vips
   
   # 检查 pkg-config 是否能找到 vips
   pkg-config --exists vips && echo "Found" || echo "Not found"
   ```

3. **手动配置路径**
   
   如果自动检测失败，可能需要手动配置库路径。

### 性能问题

如果应用运行缓慢：

1. **检查系统资源**
   - 确保有足够的可用内存
   - 检查CPU使用率
   - 确认存储空间充足

2. **优化设置**
   - 减少并发处理数量
   - 选择更快的AI服务
   - 使用较小的PDF文件测试

3. **网络优化**
   - 检查网络连接稳定性
   - 考虑使用本地AI服务
   - 配置合适的超时设置

## 📞 获取帮助

如果您的系统不满足要求或遇到兼容性问题：

- 📧 联系技术支持: [hzruo@outlook.com](mailto:hzruo@outlook.com)
- 🐛 报告问题: [GitHub Issues](https://github.com/hzruo/pdfSeer/issues)
- 📖 查看 [故障排除指南](/faq)

---

确保您的系统满足以上要求后，就可以开始 [安装识文君](/guide/installation) 了！
