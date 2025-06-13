#!/bin/bash

# 本地开发构建脚本

VERSION=${1:-"dev"}
echo "构建版本: $VERSION"

# 构建应用
echo "正在构建应用..."
wails build -platform darwin/amd64 -ldflags "-X main.version=$VERSION"

# 检查构建是否成功
if [ $? -ne 0 ]; then
    echo "构建失败"
    exit 1
fi

# 复制版本文件到应用包内
if [ -d "build/bin/识文君.app/Contents/Resources" ]; then
    cp version.json "build/bin/识文君.app/Contents/Resources/"
    echo "版本文件已复制到应用包内"
fi

# 修复Info.plist中的可执行文件名
if [ -f "build/bin/识文君.app/Contents/Info.plist" ]; then
    # 使用更简单的方法修复CFBundleExecutable
    sed -i '' '/CFBundleExecutable/{n;s/<string>识文君<\/string>/<string>pdfSeer<\/string>/;}' "build/bin/识文君.app/Contents/Info.plist"
    echo "Info.plist中的可执行文件名已修复为pdfSeer"
fi

# 移除扩展属性
xattr -cr "build/bin/识文君.app"

# 检查是否有代码签名证书
SIGNING_IDENTITY=""
if security find-identity -v -p codesigning | grep -q "Developer ID Application"; then
    SIGNING_IDENTITY=$(security find-identity -v -p codesigning | grep "Developer ID Application" | head -1 | sed 's/.*"\(.*\)".*/\1/')
    echo "找到签名证书: $SIGNING_IDENTITY"

    # 对应用进行代码签名
    echo "正在对应用进行代码签名..."
    codesign --force --deep --sign "$SIGNING_IDENTITY" "build/bin/识文君.app"

    if [ $? -eq 0 ]; then
        echo "✅ 代码签名成功"
        # 验证签名
        codesign --verify --verbose "build/bin/识文君.app"
    else
        echo "⚠️  代码签名失败，但应用仍可使用"
    fi
elif security find-identity -v -p codesigning | grep -q "Mac Developer\|Apple Development"; then
    SIGNING_IDENTITY=$(security find-identity -v -p codesigning | grep -E "Mac Developer|Apple Development" | head -1 | sed 's/.*"\(.*\)".*/\1/')
    echo "找到开发证书: $SIGNING_IDENTITY"

    # 使用开发证书签名（仅限本地使用）
    echo "正在使用开发证书签名..."
    codesign --force --deep --sign "$SIGNING_IDENTITY" "build/bin/识文君.app"

    if [ $? -eq 0 ]; then
        echo "✅ 开发证书签名成功（仅限本地使用）"
    else
        echo "⚠️  签名失败，但应用仍可使用"
    fi
else
    echo "⚠️  未找到代码签名证书"
    echo "💡 提示: 应用可能会被 macOS 标记为'已损坏'"
    echo "   解决方法: sudo xattr -rd com.apple.quarantine \"build/bin/识文君.app\""
fi

echo "构建完成！"
echo "应用位置: build/bin/识文君.app"
echo "版本: $VERSION"
