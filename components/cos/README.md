# 腾讯云对象存储 (COS) 组件

该组件提供了对腾讯云对象存储 (COS) 服务的封装，简化了文件上传、下载、删除等操作。

## 功能特性

- 文件上传：支持本地文件上传和字节数据上传
- 文件下载：支持下载到本地文件或直接获取文件内容
- 文件管理：支持删除文件、批量删除、检查文件是否存在
- 文件列表：支持列出指定前缀的文件
- URL生成：支持生成带签名的临时访问URL
- 文件追加：支持向已有文件追加内容

## 客户端使用方式

本项目在初始化时已经通过 `helpers/cos.go` 中的 `InitCos()` 方法创建了全局 COS 客户端实例。因此，您无需重新创建客户端，可以直接使用全局 `helpers.CosClient` 实例进行所有操作。

```go
import (
    "bw-go/helpers"
    "github.com/gin-gonic/gin"
)

// 直接使用全局客户端上传文件
func uploadSampleFile(ctx *gin.Context, localFilePath string) error {
    return helpers.CosClient.UploadFile(ctx, localFilePath, "path/to/destination.txt")
}
```

### 项目初始化代码 (helpers/cos.go)

```go
package helpers

import (
    "fmt"

    "bw-go/components/cos"
    "bw-go/components/dlog"
    "bw-go/conf"
)

// CosClient 全局COS客户端实例
var CosClient *cos.CosClient

// InitCos 初始化腾讯云COS客户端
// 从配置文件中读取COS配置，创建全局COS客户端实例
// 如初始化失败，则直接panic使程序退出
func InitCos() {
    var err error
    // 从配置文件中获取COS配置并初始化客户端
    CosClient, err = cos.NewCosClient(cos.Config{
        SecretID:  conf.RConf.Cos.SecretID,
        SecretKey: conf.RConf.Cos.SecretKey,
        Bucket:    conf.RConf.Cos.Bucket,
        AppID:     conf.RConf.Cos.AppID,
        Region:    conf.RConf.Cos.Region,
        Timeout:   int(conf.RConf.Cos.Timeout),
    })

    if err != nil || CosClient == nil {
        errMsg := fmt.Sprintf("初始化COS客户端失败: %v", err)
        dlog.Fatal(nil, errMsg)
        panic(errMsg)
    }
}
```

## 在服务中使用

### 在控制器中使用

```go
// controller/file/controller.go
package file

import (
    "fmt"
    "path/filepath"
    "time"

    "bw-go/base"
    "bw-go/components/dlog"
    "bw-go/helpers"  // 导入helpers包
    "github.com/gin-gonic/gin"
)

// @Summary 上传文件
// @Description 上传文件到 COS
// @Tags 文件
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "文件"
// @Success 200 {object} base.Response
// @Router /bw-go/file/upload [post]
func Upload(ctx *gin.Context) {
    file, header, err := ctx.Request.FormFile("file")
    if err != nil {
        dlog.Errorf(ctx, "[Upload] 获取上传文件失败: %v", err)
        components.RenderJsonFail(ctx, err)
        return
    }
    defer file.Close()

    // 创建临时文件
    tempDir := os.TempDir()
    tempFile := filepath.Join(tempDir, header.Filename)

    out, err := os.Create(tempFile)
    if err != nil {
        dlog.Errorf(ctx, "[Upload] 创建临时文件失败: %v", err)
        components.RenderJsonFail(ctx, err)
        return
    }
    defer os.Remove(tempFile) // 确保删除临时文件
    defer out.Close()

    // 将上传的文件内容写入临时文件
    _, err = io.Copy(out, file)
    if err != nil {
        dlog.Errorf(ctx, "[Upload] 写入临时文件失败: %v", err)
        components.RenderJsonFail(ctx, err)
        return
    }

    // 上传到 COS（直接使用全局CosClient）
    cosPath := fmt.Sprintf("uploads/%s", header.Filename)
    err = helpers.CosClient.UploadFile(ctx, tempFile, cosPath)
    if err != nil {
        dlog.Errorf(ctx, "[Upload] 上传到 COS 失败: %v", err)
        components.RenderJsonFail(ctx, err)
        return
    }

    // 生成文件访问 URL（有效期 24 小时）
    url, err := helpers.CosClient.GetURL(ctx, cosPath, 24*time.Hour)
    if err != nil {
        dlog.Errorf(ctx, "[Upload] 生成文件访问 URL 失败: %v", err)
        components.RenderJsonFail(ctx, err)
        return
    }

    dlog.Infof(ctx, "[Upload] 文件上传成功: %s", url)
    base.RenderJsonSucc(ctx, gin.H{"url": url})
}
```

## 常见使用场景

### 上传图片并生成访问 URL

```go
// 上传本地图片并生成临时访问链接
func UploadImage(ctx *gin.Context, localImagePath string) (string, error) {
    // 生成 COS 路径
    filename := filepath.Base(localImagePath)
    cosPath := fmt.Sprintf("images/%s", filename)

    // 上传文件（直接使用全局CosClient）
    err := helpers.CosClient.UploadFile(ctx, localImagePath, cosPath)
    if err != nil {
        return "", err
    }

    // 生成有效期为 2 小时的访问 URL
    return helpers.CosClient.GetURL(ctx, cosPath, 2*time.Hour)
}
```

### 保存文本文件

```go
// 保存日志内容到 COS
func SaveLog(ctx *gin.Context, logContent string, logName string) error {
    cosPath := fmt.Sprintf("logs/%s.log", logName)
    return helpers.CosClient.UploadData(ctx, []byte(logContent), cosPath, "text/plain")
}
```

### 批量删除文件

```go
// 批量删除指定前缀的文件
func CleanupTempFiles(ctx *gin.Context) error {
    // 列出所有临时文件
    files, err := helpers.CosClient.ListObjects(ctx, "temp/", 1000)
    if err != nil {
        return err
    }

    // 批量删除
    return helpers.CosClient.DeleteObjects(ctx, files)
}
```

## 注意事项

1. 项目启动时会自动从配置文件初始化COS客户端，无需手动创建
2. 所有方法都需要传入 gin.Context 参数，用于日志记录
3. 对于大文件上传，应考虑使用分片上传（该功能当前未实现）
4. 文件追加操作是通过下载-修改-重新上传实现的，对于频繁追加的大文件可能会有性能问题
5. 临时 URL 有效期到期后将无法访问，请根据业务需求设置合理的有效期