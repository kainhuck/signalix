package gateio

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gate/gateapi-go/v7"
)

// WebSocket futures.orders 推送里部分字段为 JSON 数字，而 gateapi.FuturesOrder 对应字段为 string；
// 大整数 id 在 map 中常为 float64 会丢精度，需优先使用 id_string。
var wsFuturesOrderStringKeys = []string{
	"size", "iceberg", "price", "left", "fill_price", "tkfr", "mkfr",
	"market_order_slip_ratio",
}

func normalizeWSOrderMap(m map[string]interface{}) {
	for _, k := range wsFuturesOrderStringKeys {
		if v, ok := m[k]; ok {
			m[k] = wsJSONToOrderString(v)
		}
	}
	if s, ok := m["id_string"].(string); ok {
		s = strings.TrimSpace(s)
		if s != "" {
			if id, err := strconv.ParseInt(s, 10, 64); err == nil {
				m["id"] = id
			}
		}
	}
	if u, ok := m["user"]; ok {
		if s, ok := u.(string); ok {
			s = strings.TrimSpace(s)
			if s != "" {
				if n, err := strconv.ParseInt(s, 10, 32); err == nil {
					m["user"] = int32(n)
				}
			}
		}
	}
	if v, ok := m["stp_id"]; ok {
		if s, ok := v.(string); ok {
			if n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 32); err == nil {
				m["stp_id"] = int32(n)
			}
		}
	}
}

func wsJSONToOrderString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return trimFloatString(t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

// futuresOrderFromWSElement 将 futures.orders 单条 result 转为 gateapi.FuturesOrder。
func futuresOrderFromWSElement(e interface{}) (gateapi.FuturesOrder, bool) {
	m, ok := e.(map[string]interface{})
	if !ok {
		return gateapi.FuturesOrder{}, false
	}
	normalizeWSOrderMap(m)
	b, err := json.Marshal(m)
	if err != nil {
		return gateapi.FuturesOrder{}, false
	}
	var fo gateapi.FuturesOrder
	if err := json.Unmarshal(b, &fo); err != nil {
		return gateapi.FuturesOrder{}, false
	}
	return fo, true
}
