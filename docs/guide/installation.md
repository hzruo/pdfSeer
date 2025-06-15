# 安装说明

本页面提供识文君在不同操作系统上的详细安装指南。

## 📦 下载应用

### 官方下载

访问 [GitHub Releases](https://github.com/hzruo/pdfSeer/releases) 下载最新版本：

- **macOS**: `pdfSeer-darwin-amd64.dmg` (Intel) / `pdfSeer-darwin-arm64.dmg` (Apple Silicon)
- **Windows**: `pdfSeer-windows-amd64.exe`
- **Linux**: `pdfSeer-linux-amd64.AppImage`

## 🍎 macOS 安装

### 系统要求

- macOS 10.15 (Catalina) 或更高版本
- 支持 Intel 和 Apple Silicon 芯片

### 安装步骤

1. **下载应用**
   ```bash
   # Intel Mac
   wget https://github.com/hzruo/pdfSeer/releases/latest/download/pdfSeer-darwin-amd64.dmg
   
   # Apple Silicon Mac
   wget https://github.com/hzruo/pdfSeer/releases/latest/download/pdfSeer-darwin-arm64.dmg
   ```

2. **安装依赖**
   ```bash
   # 安装 Homebrew (如果尚未安装)
   /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
   
   # 安装 libvips
   brew install vips
   ```

3. **安装应用**
   - 双击下载的 `.dmg` 文件
   - 将应用拖拽到 Applications 文件夹
   - 首次运行时，右键点击应用选择"打开"

### 解决"软件已损坏"问题

如果遇到"软件已损坏"提示：

```bash
# 移除隔离属性
sudo xattr -rd com.apple.quarantine /Applications/识文君.app
```

## 🪟 Windows 安装

### 系统要求

- Windows 10 (1903) 或更高版本
- x64 架构

### 安装步骤

1. **下载应用**
   ```powershell
   # 使用 PowerShell 下载
   Invoke-WebRequest -Uri "https://github.com/hzruo/pdfSeer/releases/latest/download/pdfSeer-windows-amd64.exe" -OutFile "pdfSeer-setup.exe"
   ```

2. **安装依赖**

   **下载 libvips**:
   - GitHub官方: [vips-dev-w64-all-8.12.2.zip](https://github.com/libvips/build-win64-mxe/releases/download/v8.12.2/vips-dev-w64-all-8.12.2.zip)
   - 网盘下载: [TeraCloud网盘](https://zeze.teracloud.jp/share/1271ba0bfb2d3b46) (免登录下载)

   **详细配置步骤**:
   1. **解压文件**: 将下载的zip文件解压到D盘根目录
   2. **配置环境变量**:
      - 右键"此电脑" → "属性" → "高级系统设置"
      - 点击"环境变量"按钮
      - 在"系统变量"中找到"Path"，点击"编辑"
      - 点击"新建"，添加: `D:\vips-dev-w64-all-8.12.2\bin`
      - 点击"确定"保存所有设置
   3. **验证安装**:
      - 重启命令提示符
      - 输入 `vips --version` 验证安装成功

3. **安装应用**
   - 双击 `pdfSeer-setup.exe`
   - 按照安装向导完成安装
   - 首次运行时可能需要允许防火墙访问

::: warning 重要提醒
必须先完成 libvips 依赖配置，再运行识文君软件。如果依赖未正确配置，软件将无法启动。
:::

### Windows Defender 提示

如果 Windows Defender 提示未知发布者：
1. 点击"更多信息"
2. 选择"仍要运行"

## 🐧 Linux 安装

### 系统要求

- Ubuntu 18.04+ / CentOS 7+ / Fedora 30+ 或其他主流发行版
- x64 架构

### Ubuntu/Debian 安装

1. **安装依赖**
   ```bash
   sudo apt-get update
   sudo apt-get install libvips-dev libwebkit2gtk-4.0-dev
   ```

2. **下载并安装应用**
   ```bash
   # 下载 AppImage
   wget https://github.com/hzruo/pdfSeer/releases/latest/download/pdfSeer-linux-amd64.AppImage
   
   # 添加执行权限
   chmod +x pdfSeer-linux-amd64.AppImage
   
   # 运行应用
   ./pdfSeer-linux-amd64.AppImage
   ```

### CentOS/RHEL 安装

1. **安装依赖**
   ```bash
   sudo yum install vips-devel webkit2gtk3-devel
   ```

2. **安装应用**
   ```bash
   # 下载并运行
   wget https://github.com/hzruo/pdfSeer/releases/latest/download/pdfSeer-linux-amd64.AppImage
   chmod +x pdfSeer-linux-amd64.AppImage
   ./pdfSeer-linux-amd64.AppImage
   ```

### Fedora 安装

1. **安装依赖**
   ```bash
   sudo dnf install vips-devel webkit2gtk3-devel
   ```

2. **安装应用**
   ```bash
   # 下载并运行
   wget https://github.com/hzruo/pdfSeer/releases/latest/download/pdfSeer-linux-amd64.AppImage
   chmod +x pdfSeer-linux-amd64.AppImage
   ./pdfSeer-linux-amd64.AppImage
   ```

### Arch Linux 安装

1. **安装依赖**
   ```bash
   sudo pacman -S libvips webkit2gtk
   ```

2. **安装应用**
   ```bash
   # 下载并运行
   wget https://github.com/hzruo/pdfSeer/releases/latest/download/pdfSeer-linux-amd64.AppImage
   chmod +x pdfSeer-linux-amd64.AppImage
   ./pdfSeer-linux-amd64.AppImage
   ```

## 🔧 验证安装

安装完成后，您可以通过以下方式验证：

1. **启动应用**
   - 应用应该能够正常启动
   - 首次启动会显示关于页面

2. **检查依赖**
   - 应用会自动检测 libvips 是否可用
   - 如有缺失会在界面显示提示

3. **测试功能**
   - 尝试加载一个小的PDF文件
   - 配置AI服务并进行测试识别

## ❗ 常见问题

### 依赖缺失

**问题**: 启动时提示缺少 libvips
**解决**: 按照上述步骤安装对应系统的 libvips 包

### 权限问题

**问题**: Linux 下无法执行 AppImage
**解决**: 
```bash
chmod +x pdfSeer-linux-amd64.AppImage
```

### 网络问题

**问题**: 下载速度慢或失败
**解决**: 
- 使用代理或VPN
- 尝试从镜像站点下载

## 📞 获取帮助

如果安装过程中遇到问题：

- 📖 查看 [常见问题](/faq/)
- 📧 发送邮件至 [hzruo@outlook.com](mailto:hzruo@outlook.com)
- 🐛 在 [GitHub Issues](https://github.com/hzruo/pdfSeer/issues) 报告问题

---

安装完成后，请查看 [快速开始](/guide/getting-started) 了解如何使用识文君。
