#!/bin/bash

# 发布脚本 - 创建Git标签并触发GitHub Actions构建

if [ $# -eq 0 ]; then
    echo "用法: $0 <版本号>"
    echo "示例: $0 1.0.1"
    exit 1
fi

VERSION=$1

# 验证版本号格式
if [[ ! $VERSION =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "错误: 版本号格式不正确，应该是 x.y.z 格式"
    exit 1
fi

echo "准备发布版本: v$VERSION"

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

# 更新版本文件
echo "更新版本信息..."
jq --arg version "$VERSION" '.version = $version' version.json > tmp.json && mv tmp.json version.json

# 更新前端package.json
cd frontend
npm version $VERSION --no-git-tag-version
cd ..

# 提交版本更改
git add version.json frontend/package.json
git commit -m "chore: bump version to v$VERSION"

# 创建标签
echo "创建Git标签: v$VERSION"
git tag -a "v$VERSION" -m "Release v$VERSION"

# 推送到远程仓库
echo "推送到远程仓库..."
git push origin main
git push origin "v$VERSION"

echo "发布完成！"
echo "GitHub Actions将自动构建多平台包"
echo "查看构建状态: https://github.com/$(git config --get remote.origin.url | sed 's/.*github.com[:/]\([^.]*\).*/\1/')/actions"
