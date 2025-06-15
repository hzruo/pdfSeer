#!/bin/bash

# VuePress 文档开发服务器脚本

set -e

echo "🚀 启动识文君文档开发服务器..."

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

# 检查是否已安装依赖
if [ ! -d "node_modules" ]; then
    echo "📦 首次运行，安装依赖..."
    npm install
fi

echo "🔧 启动开发服务器..."
echo "📖 文档将在 http://localhost:8080 打开"
echo "🔄 文件变更会自动重新加载"
echo ""
echo "按 Ctrl+C 停止服务器"

npm run dev
