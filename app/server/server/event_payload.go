package server

// parseUintPayload 解析内部事件 payload 中的 user_id（JSON 数字经 NATS 反序列化为 float64）。
func parseUintPayload(payload interface{}) uint {
	switch v := payload.(type) {
	case map[string]interface{}:
		if f, ok := v["user_id"].(float64); ok && f > 0 {
			return uint(f)
		}
	case float64:
		if v > 0 {
			return uint(v)
		}
	case int:
		if v > 0 {
			return uint(v)
		}
	case uint:
		return v
	}
	return 0
}

// parseStringPairs 从 payload map 取两个字符串键；payload 为 map[string]string 或 map[string]interface{}。
func parseStringPairs(payload interface{}, keys ...string) (string, string) {
	var vals [2]string
	m, ok := payload.(map[string]interface{})
	if !ok {
		if ms, ok2 := payload.(map[string]string); ok2 {
			for i, k := range keys {
				vals[i] = ms[k]
			}
		}
		return vals[0], vals[1]
	}
	for i, k := range keys {
		if s, ok := m[k].(string); ok {
			vals[i] = s
		}
	}
	return vals[0], vals[1]
}
