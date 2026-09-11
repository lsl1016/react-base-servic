// Package env 提供应用名与配置加载的最小实现，替代原内部框架 env 包。
// 配置文件从工作目录下的 conf/mount/ 读取。
package env

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var (
	appName  string
	rootPath string
)

// SetAppName 设置应用名（本地实现仅存储）。
func SetAppName(name string) {
	appName = name
}

// AppName 读取应用名。
func AppName() string {
	return appName
}

// SetRootPath 设置配置加载根路径（测试中用于指向仓库根目录）。
func SetRootPath(path string) {
	rootPath = path
}

// SubConfType 配置加载子目录类型。
type SubConfType int

// SubConfMount 表示从 conf/mount 子目录加载。
const SubConfMount SubConfType = iota

// LoadConf 从 <rootPath>/conf/mount 读取指定配置文件并反序列化到 target。
func LoadConf(filename string, _ SubConfType, target interface{}) {
	path := filepath.Join(rootPath, "conf", "mount", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("load conf %s failed: %v", path, err))
	}
	if err := yaml.Unmarshal(data, target); err != nil {
		panic(fmt.Sprintf("parse conf %s failed: %v", path, err))
	}
}
