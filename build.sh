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
    sed -i '' 's/<string>识文君<\/string>/<string>pdfSeer<\/string>/g' "build/bin/识文君.app/Contents/Info.plist"
    echo "Info.plist已修复"
fi

# 移除扩展属性
xattr -cr "build/bin/识文君.app"

echo "构建完成！"
echo "应用位置: build/bin/识文君.app"
echo "版本: $VERSION"
