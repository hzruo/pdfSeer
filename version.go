package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// 版本信息 - 可以通过编译时的ldflags设置
var version = "1.0.0" // 默认版本，会被构建时覆盖

// VersionInfo 版本信息结构
type VersionInfo struct {
	Version     string `json:"version"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Email       string `json:"email"`
	Copyright   string `json:"copyright"`
}

var appVersion *VersionInfo

// LoadVersion 加载版本信息
func LoadVersion() (*VersionInfo, error) {
	if appVersion != nil {
		return appVersion, nil
	}

	// 获取可执行文件所在目录
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("获取可执行文件路径失败: %v", err)
	}

	execDir := filepath.Dir(execPath)

	// 尝试多个可能的位置
	possiblePaths := []string{
		// macOS应用包内的Resources目录
		filepath.Join(execDir, "..", "Resources", "version.json"),
		// 可执行文件同目录
		filepath.Join(execDir, "version.json"),
		// 当前工作目录
		"version.json",
	}

	var versionFile string
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			versionFile = path
			break
		}
	}

	if versionFile == "" {
		versionFile = "version.json" // 默认路径
	}
	
	data, err := os.ReadFile(versionFile)
	if err != nil {
		// 如果读取失败，返回默认版本信息
		return &VersionInfo{
			Version:     version, // 使用编译时设置的版本
			Name:        "识文君",
			Description: "PDF智能助手",
			Author:      "hzruo",
			Email:       "hzruo@outlook.com",
			Copyright:   "© 2025 识文君 PDF智能助手",
		}, nil
	}

	var versionInfo VersionInfo
	if err := json.Unmarshal(data, &versionInfo); err != nil {
		return nil, fmt.Errorf("解析版本信息失败: %v", err)
	}

	// 如果配置文件中没有版本号，使用编译时的版本
	if versionInfo.Version == "" {
		versionInfo.Version = version
	}

	appVersion = &versionInfo
	return appVersion, nil
}

// GetVersion 获取版本号
func GetVersion() string {
	version, _ := LoadVersion()
	return version.Version
}

// GetFullVersion 获取完整版本信息
func GetFullVersion() string {
	versionInfo, _ := LoadVersion()
	return versionInfo.Version
}

// GetAppInfo 获取应用信息
func GetAppInfo() map[string]string {
	versionInfo, _ := LoadVersion()
	return map[string]string{
		"name":        versionInfo.Name,
		"version":     versionInfo.Version,
		"description": versionInfo.Description,
		"author":      versionInfo.Author,
		"email":       versionInfo.Email,
		"copyright":   versionInfo.Copyright,
	}
}
