package gateio

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/kainhuck/signalix/pkg/exchange/perp"

	"github.com/gate/gateapi-go/v7"
)

// toGateContract 将规范 BASE/QUOTE 转为 Gate REST/WS 使用的 BASE_QUOTE。
func toGateContract(c perp.Contract) string {
	s := string(c.Canonical())
	if s == "" {
		return ""
	}
	return strings.ReplaceAll(s, "/", "_")
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

func trimFloatString(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

func decimalsInStep(step string) string {
	step = strings.TrimSpace(step)
	if step == "" || step == "0" {
		return "1"
	}
	return step
}

func gateSizeSide(sizeStr string) perp.Side {
	if strings.HasPrefix(strings.TrimSpace(sizeStr), "-") {
		return perp.SideSell
	}
	return perp.SideBuy
}

func futuresFilled(fo gateapi.FuturesOrder) float64 {
	total := math.Abs(parseFloat(fo.Size))
	left := math.Abs(parseFloat(fo.Left))
	return math.Max(0, total-left)
}

func mapFuturesOrderStatus(fo gateapi.FuturesOrder) perp.OrderStatus {
	if fo.Status == "open" {
		left := parseFloat(fo.Left)
		total := math.Abs(parseFloat(fo.Size))
		if left > 0 && left < total {
			return perp.OrderPartialFilled
		}
		return perp.OrderSubmitted
	}
	if fo.Status == "finished" {
		switch fo.FinishAs {
		case "filled":
			return perp.OrderFilled
		case "cancelled", "reduce_only", "position_closed", "stp", "reduce_out":
			return perp.OrderCancelled
		default:
			if fo.FinishAs == "ioc" || fo.FinishAs == "fok" {
				if parseFloat(fo.Left) == 0 {
					return perp.OrderFilled
				}
				return perp.OrderCancelled
			}
			return perp.OrderRejected
		}
	}
	return perp.OrderPending
}

func futuresTime(ts float64) time.Time {
	if ts <= 0 {
		return time.Time{}
	}
	// REST 常为带小数的 Unix 秒；WebSocket 常为毫秒整数（≥1e12）。
	if ts >= 1_000_000_000_000 {
		return time.UnixMilli(int64(ts))
	}
	sec := int64(ts)
	nsec := int64((ts - float64(sec)) * 1e9)
	return time.Unix(sec, nsec)
}

func orderTypeFromGate(fo gateapi.FuturesOrder) perp.OrderType {
	if strings.EqualFold(fo.Tif, "ioc") && parseFloat(fo.Price) == 0 {
		return perp.OrderTypeMarket
	}
	return perp.OrderTypeLimit
}

func orderViewFromFutures(fo gateapi.FuturesOrder) *perp.OrderSnapshot {
	price := strings.TrimSpace(fo.Price)
	var priceOut string
	if parseFloat(price) != 0 {
		priceOut = price
	}
	ap := strings.TrimSpace(fo.FillPrice)
	var avg string
	if parseFloat(ap) != 0 {
		avg = ap
	}
	return &perp.OrderSnapshot{
		ExchangeOrderID: strconv.FormatInt(fo.Id, 10),
		Contract:        perp.CanonicalContract(fo.Contract),
		Side:            gateSizeSide(fo.Size),
		Type:            orderTypeFromGate(fo),
		Size:            trimFloatString(math.Abs(parseFloat(fo.Size))),
		Price:           priceOut,
		FilledSize:      trimFloatString(futuresFilled(fo)),
		AvgPrice:        avg,
		Status:          mapFuturesOrderStatus(fo),
		ClientID:        strings.TrimSpace(fo.Text),
		CreatedAt:       futuresTime(fo.CreateTime),
		UpdatedAt:       futuresTime(fo.UpdateTime),
	}
}

func mapTIF(tif perp.TimeInForce) string {
	switch tif {
	case perp.TIFIOC:
		return "ioc"
	case perp.TIFFOK:
		return "fok"
	case perp.TIFPOC:
		return "poc"
	default:
		return "gtc"
	}
}

func gateOrderSizeString(size string, side perp.Side) (string, error) {
	s := strings.TrimSpace(size)
	if s == "" || s == "0" {
		return "", perp.NewError(perp.ErrInvalidParameter, "size must be positive", nil)
	}
	if strings.HasPrefix(s, "-") {
		return "", perp.NewError(perp.ErrInvalidParameter, "size must be positive in PlaceRequest", nil)
	}
	switch side {
	case perp.SideBuy:
		return s, nil
	case perp.SideSell:
		if strings.HasPrefix(s, "-") {
			return s, nil
		}
		return "-" + s, nil
	default:
		return "", perp.NewError(perp.ErrInvalidParameter, "invalid side", nil)
	}
}

// gateUnixTime 解析 Gate 返回的 Unix 时间戳：数值 ≥1e12 视为毫秒，否则视为秒（持仓 update_time 多为秒）。
func gateUnixTime(ts int64) time.Time {
	if ts <= 0 {
		return time.Time{}
	}
	if ts >= 1_000_000_000_000 {
		return time.UnixMilli(ts)
	}
	return time.Unix(ts, 0)
}

func positionView(contract string, p gateapi.Position) (*perp.PositionSnapshot, error) {
	sz := parseFloat(p.Size)
	if sz == 0 {
		return nil, perp.NewError(perp.ErrPositionNotFound, string(perp.CanonicalContract(contract)), nil)
	}
	side := perp.PositionLong
	if sz < 0 {
		side = perp.PositionShort
	}
	levStr := p.Lever
	if levStr == "" {
		levStr = p.Leverage
	}
	ts := time.Now()
	if p.UpdateTime > 0 {
		ts = gateUnixTime(p.UpdateTime)
	}
	return &perp.PositionSnapshot{
		Contract:      perp.CanonicalContract(contract),
		Side:          side,
		Size:          trimFloatString(math.Abs(sz)),
		EntryPrice:    strings.TrimSpace(p.EntryPrice),
		MarkPrice:     strings.TrimSpace(p.MarkPrice),
		UnrealizedPnl: strings.TrimSpace(p.UnrealisedPnl),
		Leverage:      int(parseFloat(levStr)),
		UpdatedAt:     ts,
	}, nil
}
