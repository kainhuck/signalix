package gateio

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gate/gateapi-go/v7"
)

var wsSpotOrderStringKeys = []string{
	"amount", "iceberg", "price", "left", "filled_amount", "filled_total",
	"fill_price", "avg_deal_price", "fee", "point_fee", "gt_fee",
}

func normalizeWSSpotOrderMap(m map[string]interface{}) {
	for _, k := range wsSpotOrderStringKeys {
		if v, ok := m[k]; ok {
			m[k] = wsJSONToString(v)
		}
	}
	if v, ok := m["create_time"]; ok {
		m["create_time"] = wsJSONToString(v)
	}
	if v, ok := m["update_time"]; ok {
		m["update_time"] = wsJSONToString(v)
	}
	if v, ok := m["create_time_ms"]; ok {
		switch t := v.(type) {
		case float64:
			m["create_time_ms"] = int64(t)
		case json.Number:
			if n, err := t.Int64(); err == nil {
				m["create_time_ms"] = n
			}
		}
	}
	if v, ok := m["update_time_ms"]; ok {
		switch t := v.(type) {
		case float64:
			m["update_time_ms"] = int64(t)
		case json.Number:
			if n, err := t.Int64(); err == nil {
				m["update_time_ms"] = n
			}
		}
	}
}

func wsJSONToString(v interface{}) string {
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

func spotOrderFromWSElement(e interface{}) (gateapi.Order, bool) {
	m, ok := e.(map[string]interface{})
	if !ok {
		return gateapi.Order{}, false
	}
	normalizeWSSpotOrderMap(m)
	b, err := json.Marshal(m)
	if err != nil {
		return gateapi.Order{}, false
	}
	var o gateapi.Order
	if err := json.Unmarshal(b, &o); err != nil {
		return gateapi.Order{}, false
	}
	return o, true
}

func wsJSONInt64(v interface{}) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		return n
	default:
		return 0
	}
}
