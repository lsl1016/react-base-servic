package zlog

import "fmt"

// sprintf 独立小函数，避免 zlog.go 里因引入 fmt 而与不同构建标签产生耦合。
func sprintf(format string, v ...interface{}) string {
	return fmt.Sprintf(format, v...)
}
