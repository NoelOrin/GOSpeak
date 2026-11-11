package oauth

import (
	"errors"
	"net/url"
	"strings"
)

// sensitiveQueryKeys 是绝不能出现在日志或对外错误文案里的 URL query 参数。
// QQ 等 provider 要求把 access_token 放在 userinfo URL 的 query 中，
// 而 Go 的 *url.Error 会把完整 URL 带进 Error()；一旦上层把 err.Error()
// 写进响应或重定向地址，token 就会进入浏览器历史与反代日志。
var sensitiveQueryKeys = map[string]struct{}{
	"access_token":  {},
	"oauth_token":   {},
	"refresh_token": {},
	"token":         {},
	"code":          {},
	"client_secret": {},
	"client_id":     {},
	"key":           {},
	"appkey":        {},
}

// RedactURL 把 URL 中敏感 query 值替换为占位符，并丢弃 userinfo/fragment。
// 解析失败时只返回固定占位串，不回显原始输入。
func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<redacted url>"
	}
	q := u.Query()
	for k := range q {
		if _, sensitive := sensitiveQueryKeys[strings.ToLower(k)]; sensitive {
			q.Set(k, "REDACTED")
		}
	}
	u.RawQuery = q.Encode()
	u.User = nil
	u.Fragment = ""
	return u.String()
}

// redactURLError 保留 errors.Is/As 链，但确保 Error() 输出的是脱敏 URL。
func redactURLError(err error) error {
	var ue *url.Error
	if !errors.As(err, &ue) {
		return err
	}
	return &url.Error{Op: ue.Op, URL: RedactURL(ue.URL), Err: ue.Err}
}
