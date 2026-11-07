// Package knowledge 负责把上传文件解析为可入库的正文文本。
// 本迭代只支持非空 UTF-8 纯文本（.txt/.md）；PDF/Word 等格式属 Non-goals。
package knowledge

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// ErrInvalidText 表示内容无法作为非空 UTF-8 文本解析。
var ErrInvalidText = errors.New("内容不是非空的 UTF-8 文本")

// ParseText 校验并返回正文。失败时调用方必须删除已落盘文件且不写数据库。
func ParseText(data []byte) (string, error) {
	if len(data) == 0 || !utf8.Valid(data) {
		return "", ErrInvalidText
	}
	body := string(data)
	if strings.TrimSpace(body) == "" {
		return "", ErrInvalidText
	}
	return body, nil
}
