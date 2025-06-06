package system

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// DependencyStatus 依赖状态
type DependencyStatus struct {
	Name        string `json:"name"`
	Required    bool   `json:"required"`
	Installed   bool   `json:"installed"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Error       string `json:"error,omitempty"`
}

// SystemInfo 系统信息
type SystemInfo struct {
	OS           string              `json:"os"`
	Arch         string              `json:"arch"`
	Dependencies []*DependencyStatus `json:"dependencies"`
}

// CheckDependencies 检查系统依赖
func CheckDependencies() *SystemInfo {
	info := &SystemInfo{
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		Dependencies: make([]*DependencyStatus, 0),
	}

	// 检查libvips
	vipsStatus := checkLibVips()
	info.Dependencies = append(info.Dependencies, vipsStatus)

	// 检查其他可选依赖
	if runtime.GOOS == "darwin" {
		brewStatus := checkBrew()
		info.Dependencies = append(info.Dependencies, brewStatus)
	}

	return info
}

// checkLibVips 检查libvips依赖
func checkLibVips() *DependencyStatus {
	status := &DependencyStatus{
		Name:        "libvips",
		Required:    true,
		Description: "图像处理库，用于PDF页面渲染",
		Installed:   false,
	}

	// 尝试通过pkg-config检查
	if version, err := checkPkgConfig("vips"); err == nil {
		status.Installed = true
		status.Version = version
		return status
	}

	// 尝试通过vips命令检查
	if version, err := checkVipsCommand(); err == nil {
		status.Installed = true
		status.Version = version
		return status
	}

	// 尝试通过动态库检查
	if checkVipsLibrary() {
		status.Installed = true
		status.Version = "已安装（版本未知）"
		return status
	}

	status.Error = "libvips未安装或无法检测到"
	return status
}

// checkPkgConfig 通过pkg-config检查
func checkPkgConfig(pkg string) (string, error) {
	cmd := exec.Command("pkg-config", "--modversion", pkg)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// checkVipsCommand 通过vips命令检查
func checkVipsCommand() (string, error) {
	cmd := exec.Command("vips", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	
	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		// 提取版本号
		versionLine := lines[0]
		if strings.Contains(versionLine, "vips") {
			parts := strings.Fields(versionLine)
			if len(parts) >= 2 {
				return parts[1], nil
			}
		}
	}
	
	return strings.TrimSpace(string(output)), nil
}

// checkVipsLibrary 检查libvips动态库
func checkVipsLibrary() bool {
	var libPaths []string
	
	switch runtime.GOOS {
	case "darwin":
		libPaths = []string{
			"/usr/local/lib/libvips.dylib",
			"/opt/homebrew/lib/libvips.dylib",
			"/usr/lib/libvips.dylib",
		}
	case "linux":
		libPaths = []string{
			"/usr/lib/x86_64-linux-gnu/libvips.so",
			"/usr/lib/libvips.so",
			"/usr/local/lib/libvips.so",
		}
	case "windows":
		libPaths = []string{
			"C:\\vips\\bin\\libvips-42.dll",
			"libvips-42.dll",
		}
	}
	
	for _, path := range libPaths {
		if _, err := exec.LookPath(path); err == nil {
			return true
		}
	}
	
	return false
}

// checkBrew 检查Homebrew（仅macOS）
func checkBrew() *DependencyStatus {
	status := &DependencyStatus{
		Name:        "homebrew",
		Required:    false,
		Description: "macOS包管理器，用于安装libvips",
		Installed:   false,
	}

	cmd := exec.Command("brew", "--version")
	output, err := cmd.Output()
	if err != nil {
		status.Error = "Homebrew未安装"
		return status
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		status.Installed = true
		status.Version = strings.TrimSpace(lines[0])
	}

	return status
}

// GetInstallInstructions 获取安装说明
func GetInstallInstructions() map[string]string {
	instructions := make(map[string]string)
	
	switch runtime.GOOS {
	case "darwin":
		instructions["libvips"] = `macOS安装libvips:
1. 使用Homebrew: brew install vips
2. 使用MacPorts: sudo port install vips
3. 从源码编译: https://github.com/libvips/libvips`
		
	case "linux":
		instructions["libvips"] = `Linux安装libvips:
Ubuntu/Debian: sudo apt-get install libvips-dev
CentOS/RHEL: sudo yum install vips-devel
Fedora: sudo dnf install vips-devel
Arch: sudo pacman -S libvips`
		
	case "windows":
		instructions["libvips"] = `Windows安装libvips:
1. 下载预编译包: https://github.com/libvips/libvips/releases
2. 使用vcpkg: vcpkg install libvips
3. 使用MSYS2: pacman -S mingw-w64-x86_64-libvips`
	}
	
	return instructions
}

// FormatDependencyReport 格式化依赖报告
func FormatDependencyReport(info *SystemInfo) string {
	var report strings.Builder
	
	report.WriteString(fmt.Sprintf("系统信息: %s/%s\n", info.OS, info.Arch))
	report.WriteString("依赖检查结果:\n")
	
	for _, dep := range info.Dependencies {
		status := "❌"
		if dep.Installed {
			status = "✅"
		}
		
		required := ""
		if dep.Required {
			required = " (必需)"
		}
		
		report.WriteString(fmt.Sprintf("  %s %s%s", status, dep.Name, required))
		
		if dep.Installed && dep.Version != "" {
			report.WriteString(fmt.Sprintf(" - %s", dep.Version))
		}
		
		if dep.Error != "" {
			report.WriteString(fmt.Sprintf(" - %s", dep.Error))
		}
		
		report.WriteString("\n")
		
		if dep.Description != "" {
			report.WriteString(fmt.Sprintf("    %s\n", dep.Description))
		}
	}
	
	return report.String()
}
