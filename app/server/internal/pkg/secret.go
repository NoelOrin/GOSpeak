package pkg

// MaskSecret 对密钥做脱敏展示：长度 ≤5 返回 "***"，否则保留前 2 与后 3 位。
// 用于管理后台读取配置时不回显完整密钥。
func MaskSecret(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 5 {
		return "***"
	}
	return key[:2] + "***" + key[len(key)-3:]
}

// KeepSecret 在更新配置时保留旧密钥：请求值为空则沿用 existing。
func KeepSecret(next, existing string) string {
	if next == "" {
		return existing
	}
	return next
}
