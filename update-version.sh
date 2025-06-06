#!/bin/bash

# 版本更新脚本 - 更新项目中的版本文件

if [ $# -eq 0 ]; then
    echo "用法: $0 <版本号>"
    echo "示例: $0 1.0.1"
    echo ""
    echo "此脚本会更新以下文件:"
    echo "  - version.json"
    echo "  - frontend/package.json"
    echo ""
    echo "更新后请提交更改，然后运行 ./release.sh 创建标签"
    exit 1
fi

VERSION=$1

# 验证版本号格式
if [[ ! $VERSION =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "错误: 版本号格式不正确，应该是 x.y.z 格式"
    exit 1
fi

echo "更新版本到: $VERSION"

# 更新 version.json
if [ -f version.json ]; then
    CURRENT_VERSION=$(jq -r '.version' version.json 2>/dev/null || echo "unknown")
    if [ "$CURRENT_VERSION" != "$VERSION" ]; then
        jq --arg version "$VERSION" '.version = $version' version.json > tmp.json && mv tmp.json version.json
        echo "✅ 更新 version.json: $CURRENT_VERSION -> $VERSION"
    else
        echo "ℹ️  version.json 版本已是 $VERSION"
    fi
else
    echo "❌ version.json 不存在"
    exit 1
fi

# 更新前端 package.json
if [ -f frontend/package.json ]; then
    cd frontend
    PKG_VERSION=$(node -p "require('./package.json').version" 2>/dev/null || echo "unknown")
    if [ "$PKG_VERSION" != "$VERSION" ]; then
        npm version $VERSION --no-git-tag-version
        echo "✅ 更新 package.json: $PKG_VERSION -> $VERSION"
    else
        echo "ℹ️  package.json 版本已是 $VERSION"
    fi
    cd ..
else
    echo "❌ frontend/package.json 不存在"
    exit 1
fi

echo ""
echo "版本更新完成！"
echo ""
echo "下一步:"
echo "1. 检查更改: git diff"
echo "2. 提交更改: git add . && git commit -m 'chore: bump version to v$VERSION'"
echo "3. 创建标签: ./release.sh $VERSION"
