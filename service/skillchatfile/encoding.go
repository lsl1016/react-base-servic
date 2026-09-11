package skillchatfile

import (
	"bytes"
	"unicode/utf8"

	"react-base-service/components"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
)

// 附件字符集标识，存入文件元数据；COS 中保留原始字节，读取时按此标识转码
const (
	CharsetUTF8    = "utf-8"
	CharsetUTF16   = "utf-16"
	CharsetGB18030 = "gb18030"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// DetectChatFileCharset 判定纯文本附件的字符集。
// 判定链：UTF-16 BOM → 含 NUL 判非文本 → utf8.Valid → GB18030 兜底（覆盖 GBK/GB2312 即 Windows ANSI）。
// 无法确定编码或内容不是文本时返回错误；Big5/Shift-JIS 等若恰好能被 GB18030 干净解码则无法识别，会被判为 gb18030。
func DetectChatFileCharset(data []byte) (string, error) {
	if hasUTF16BOM(data) {
		if _, err := decodeUTF16(data); err != nil {
			return "", err
		}
		return CharsetUTF16, nil
	}
	if bytes.IndexByte(data, 0x00) >= 0 {
		return "", components.ParamInvalidf("文件不是纯文本内容")
	}
	if utf8.Valid(data) {
		return CharsetUTF8, nil
	}
	if _, err := decodeGB18030(data); err != nil {
		return "", err
	}
	return CharsetGB18030, nil
}

// DecodeChatFileToUTF8 按元数据中的字符集把附件原始字节转成 UTF-8 文本。
// charset 为空时按 UTF-8 处理（历史文件上传时已强制 UTF-8）。
func DecodeChatFileToUTF8(data []byte, charset string) (string, error) {
	switch charset {
	case CharsetUTF16:
		return decodeUTF16(data)
	case CharsetGB18030:
		return decodeGB18030(data)
	case CharsetUTF8, "":
		data = bytes.TrimPrefix(data, utf8BOM)
		if !utf8.Valid(data) {
			return "", components.ParamInvalidf("文件不是 UTF-8 编码")
		}
		return string(data), nil
	default:
		return "", components.ParamInvalidf("不支持的字符集: %s", charset)
	}
}

func hasUTF16BOM(data []byte) bool {
	return len(data) >= 2 &&
		((data[0] == 0xFF && data[1] == 0xFE) || (data[0] == 0xFE && data[1] == 0xFF))
}

// decodeUTF16 解码带 BOM 的 UTF-16 字节流，BOM 决定字节序并被剥离
func decodeUTF16(data []byte) (string, error) {
	decoder := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()
	decoded, err := decoder.Bytes(data)
	if err != nil || bytes.ContainsRune(decoded, utf8.RuneError) {
		return "", components.ParamInvalidf("无法识别文件编码，请将文件另存为 UTF-8 后重新上传")
	}
	return string(decoded), nil
}

func decodeGB18030(data []byte) (string, error) {
	decoded, err := simplifiedchinese.GB18030.NewDecoder().Bytes(data)
	if err != nil || bytes.ContainsRune(decoded, utf8.RuneError) {
		return "", components.ParamInvalidf("无法识别文件编码，请将文件另存为 UTF-8 后重新上传")
	}
	return string(decoded), nil
}
