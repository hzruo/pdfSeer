#!/bin/bash

# 发布脚本 - 创建Git标签并触发GitHub Actions构建

if [ $# -eq 0 ]; then
    echo "用法: $0 <版本号> [选项]"
    echo "示例: $0 1.0.1"
    echo ""
    echo "注意: 请先手动更新版本文件，然后运行此脚本创建标签"
    echo "需要更新的文件:"
    echo "  - version.json"
    echo "  - frontend/package.json"
    echo ""
    echo "选项:"
    echo "  --force, -f    强制重新打标签（删除已存在的标签）"
    exit 1
fi

VERSION=$1
FORCE_RETAG=false

# 处理命令行参数
shift
while [[ $# -gt 0 ]]; do
    case $1 in
        --force|-f)
            FORCE_RETAG=true
            shift
            ;;
        *)
            echo "未知选项: $1"
            exit 1
            ;;
    esac
done

# 验证版本号格式
if [[ ! $VERSION =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "错误: 版本号格式不正确，应该是 x.y.z 格式"
    exit 1
fi

echo "准备发布版本: v$VERSION"

# 检查标签是否已存在
TAG_EXISTS=false
if git rev-parse "v$VERSION" >/dev/null 2>&1; then
    TAG_EXISTS=true
    echo "警告: 标签 v$VERSION 已存在"

    if [ "$FORCE_RETAG" = true ]; then
        echo "强制模式: 将删除现有标签并重新创建"
    else
        echo "选项:"
        echo "1. 退出 (默认)"
        echo "2. 删除现有标签并重新创建"
        echo "3. 仅更新版本文件，不创建标签"
        read -p "请选择 (1/2/3): " -n 1 -r
        echo
        case $REPLY in
            2)
                FORCE_RETAG=true
                ;;
            3)
                echo "仅更新版本文件模式"
                ;;
            *)
                echo "退出"
                exit 1
                ;;
        esac
    fi
fi

# 检查是否有未提交的更改
if [ -n "$(git status --porcelain)" ]; then
    echo "警告: 有未提交的更改，请先提交所有更改"
    git status --short
    read -p "是否继续? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# 验证版本文件
echo "验证版本信息..."

# 检查 version.json
VERSION_JSON_VER="unknown"
if [ -f version.json ]; then
    VERSION_JSON_VER=$(jq -r '.version' version.json 2>/dev/null || echo "unknown")
    echo "version.json 版本: $VERSION_JSON_VER"
else
    echo "警告: version.json 不存在"
fi

# 检查 package.json
PKG_JSON_VER="unknown"
if [ -f frontend/package.json ]; then
    PKG_JSON_VER=$(node -p "require('./frontend/package.json').version" 2>/dev/null || echo "unknown")
    echo "package.json 版本: $PKG_JSON_VER"
else
    echo "警告: frontend/package.json 不存在"
fi

# 验证版本一致性
echo "目标标签版本: $VERSION"
VERSION_MISMATCH=false

if [ "$VERSION_JSON_VER" != "unknown" ] && [ "$VERSION_JSON_VER" != "$VERSION" ]; then
    echo "错误: version.json ($VERSION_JSON_VER) 与目标版本 ($VERSION) 不匹配"
    VERSION_MISMATCH=true
fi

if [ "$PKG_JSON_VER" != "unknown" ] && [ "$PKG_JSON_VER" != "$VERSION" ]; then
    echo "错误: package.json ($PKG_JSON_VER) 与目标版本 ($VERSION) 不匹配"
    VERSION_MISMATCH=true
fi

if [ "$VERSION_MISMATCH" = true ]; then
    echo ""
    echo "请先手动更新版本文件到 $VERSION，然后重新运行此脚本"
    echo "需要更新的文件:"
    if [ "$VERSION_JSON_VER" != "$VERSION" ]; then
        echo "  - version.json"
    fi
    if [ "$PKG_JSON_VER" != "$VERSION" ]; then
        echo "  - frontend/package.json"
    fi
    exit 1
fi

echo "版本验证通过！"

# 处理标签
if [ "$FORCE_RETAG" = true ] && [ "$TAG_EXISTS" = true ]; then
    echo "删除现有标签: v$VERSION"

    # 删除本地标签
    git tag -d "v$VERSION"

    # 删除远程标签
    echo "删除远程标签..."
    git push origin --delete "v$VERSION" 2>/dev/null || echo "远程标签不存在或已删除"

    echo "等待2秒..."
    sleep 2
fi

# 创建标签
echo "创建Git标签: v$VERSION"
git tag -a "v$VERSION" -m "Release v$VERSION"

# 推送标签到远程仓库
echo "推送标签到远程仓库..."
git push origin "v$VERSION"

echo "发布完成！"
echo "GitHub Actions将自动构建多平台包"

# 显示构建状态链接
REPO_URL=$(git config --get remote.origin.url | sed 's/.*github.com[:/]\([^.]*\).*/\1/')
if [ -n "$REPO_URL" ]; then
    echo "查看构建状态: https://github.com/$REPO_URL/actions"
    echo "查看发布页面: https://github.com/$REPO_URL/releases"
fi
