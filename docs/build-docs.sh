#!/bin/bash

# VuePress 文档构建脚本

set -e

echo "🚀 开始构建识文君文档..."

# 检查 Node.js 和 npm
if ! command -v node &> /dev/null; then
    echo "❌ Node.js 未安装，请先安装 Node.js"
    exit 1
fi

if ! command -v npm &> /dev/null; then
    echo "❌ npm 未安装，请先安装 npm"
    exit 1
fi

# 进入文档目录
cd "$(dirname "$0")"

echo "📦 安装依赖..."
npm install

echo "🔧 构建文档..."
npm run build

echo "✅ 文档构建完成！"
echo "📁 构建输出: .vuepress/dist/"

# 检查是否需要部署
if [ "$1" = "--deploy" ]; then
    echo "🚀 开始部署到 GitHub Pages..."
    
    # 检查是否有 git
    if ! command -v git &> /dev/null; then
        echo "❌ Git 未安装，无法部署"
        exit 1
    fi
    
    # 进入构建输出目录
    cd .vuepress/dist
    
    # 初始化 git 仓库
    git init
    git add -A
    git commit -m "deploy docs: $(date)"
    
    # 推送到 gh-pages 分支
    git push -f git@github.com:your-repo/pdf-ocr-ai.git master:gh-pages
    
    echo "✅ 部署完成！"
    echo "🌐 文档地址: https://your-repo.github.io/pdf-ocr-ai/"
    
    cd ../..
fi

echo "🎉 所有任务完成！"
