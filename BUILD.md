# 构建和发布指南

## 本地开发构建

```bash
# 开发版本构建
./build.sh

# 指定版本号构建
./build.sh 1.0.0
```

## 发布新版本

```bash
# 发布版本 1.0.1
./release.sh 1.0.1
```

这会自动：
1. 更新 `version.json` 和 `frontend/package.json`
2. 提交更改
3. 创建Git标签 `v1.0.1`
4. 推送到GitHub
5. 触发GitHub Actions自动构建多平台包

## GitHub Actions自动构建

当推送标签时，GitHub Actions会自动构建：

- **macOS Intel** (darwin/amd64) - DMG包
- **macOS Apple Silicon** (darwin/arm64) - DMG包  
- **Windows x64** (windows/amd64) - ZIP包
- **Linux x64** (linux/amd64) - tar.gz包

构建完成后会自动创建GitHub Release并上传所有平台的包。

## 手动触发构建

在GitHub仓库的Actions页面，可以手动触发构建并指定版本号。

## 版本管理

- 版本号通过Git标签管理
- 编译时通过 `-ldflags "-X main.version=x.x.x"` 注入版本号
- 应用信息存储在 `version.json` 中
- 前端版本号与后端保持同步

## 优势

✅ **简单**: 只需要一个命令发布新版本  
✅ **自动化**: GitHub Actions处理所有平台构建  
✅ **一致性**: 所有平台使用相同的源代码和版本号  
✅ **可追溯**: 每个版本都有对应的Git标签  
✅ **分发**: 自动创建GitHub Release
