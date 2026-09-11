package skillchatfile

import (
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
)

func mustGB18030(t *testing.T, s string) []byte {
	t.Helper()
	data, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(s))
	if err != nil {
		t.Fatalf("gb18030 encode: %v", err)
	}
	return data
}

func mustUTF16LE(t *testing.T, s string) []byte {
	t.Helper()
	data, err := unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM).NewEncoder().Bytes([]byte(s))
	if err != nil {
		t.Fatalf("utf16 encode: %v", err)
	}
	return data
}

func TestDetectChatFileCharset(t *testing.T) {
	cases := []struct {
		name    string
		data    []byte
		want    string
		wantErr bool
	}{
		{name: "纯ASCII", data: []byte("hello,world\n1,2,3"), want: CharsetUTF8},
		{name: "UTF-8中文", data: []byte("列名,数值\n销售额,100"), want: CharsetUTF8},
		{name: "UTF-8带BOM", data: append([]byte{0xEF, 0xBB, 0xBF}, []byte("中文内容")...), want: CharsetUTF8},
		{name: "GBK中文", data: mustGB18030(t, "列名,数值\n销售额,100"), want: CharsetGB18030},
		{name: "UTF-16LE带BOM", data: mustUTF16LE(t, "中文,内容\nabc,123"), want: CharsetUTF16},
		{name: "空文件", data: nil, want: CharsetUTF8},
		{name: "含NUL的二进制", data: []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x01}, wantErr: true},
		{name: "截断的GBK", data: append(mustGB18030(t, "中文"), 0x81), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DetectChatFileCharset(tc.data)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("期望拒绝，实际判定为 %s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("期望 %s，实际报错: %v", tc.want, err)
			}
			if got != tc.want {
				t.Fatalf("期望 %s，实际 %s", tc.want, got)
			}
		})
	}
}

func TestDecodeChatFileToUTF8RoundTrip(t *testing.T) {
	const text = "列名,数值\n销售额,100\nabc,def"
	cases := []struct {
		name    string
		data    []byte
		charset string
	}{
		{name: "UTF-8", data: []byte(text), charset: CharsetUTF8},
		{name: "历史文件charset为空", data: []byte(text), charset: ""},
		{name: "UTF-8带BOM", data: append([]byte{0xEF, 0xBB, 0xBF}, []byte(text)...), charset: CharsetUTF8},
		{name: "GB18030", data: mustGB18030(t, text), charset: CharsetGB18030},
		{name: "UTF-16LE带BOM", data: mustUTF16LE(t, text), charset: CharsetUTF16},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeChatFileToUTF8(tc.data, tc.charset)
			if err != nil {
				t.Fatalf("解码失败: %v", err)
			}
			if got != text {
				t.Fatalf("解码结果不一致:\n期望 %q\n实际 %q", text, got)
			}
		})
	}
}

func TestDecodeChatFileToUTF8Reject(t *testing.T) {
	if _, err := DecodeChatFileToUTF8([]byte{0xD6, 0xD0}, CharsetUTF8); err == nil {
		t.Fatal("GBK 字节按 UTF-8 解码应报错")
	}
	if _, err := DecodeChatFileToUTF8([]byte("abc"), "big5"); err == nil {
		t.Fatal("未知字符集应报错")
	}
}

func TestDetectThenDecodeConsistency(t *testing.T) {
	// 检测通过的文件，按检测出的字符集解码必须成功——这是上传/读取两侧的核心契约
	samples := [][]byte{
		[]byte("plain ascii"),
		[]byte("UTF-8 中文"),
		mustGB18030(t, "ANSI 编码的中文文本，模拟 Excel 导出 CSV"),
		mustUTF16LE(t, "记事本 Unicode 存法"),
	}
	for _, data := range samples {
		charset, err := DetectChatFileCharset(data)
		if err != nil {
			t.Fatalf("检测失败: %v", err)
		}
		if _, err := DecodeChatFileToUTF8(data, charset); err != nil {
			t.Fatalf("charset=%s 解码失败: %v", charset, err)
		}
	}
}
