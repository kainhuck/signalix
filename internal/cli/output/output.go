package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/olekukonko/tablewriter"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

type Format string

const (
	Table Format = "table"
	JSON  Format = "json"
	YAML  Format = "yaml"
)

var protoMarshal = protojson.MarshalOptions{
	EmitUnpopulated: false,
	UseProtoNames:   true,
}

// Printer renders command results to stdout.
type Printer struct {
	format Format
	w      io.Writer
}

func NewPrinter(format string, w io.Writer) (*Printer, error) {
	f := Format(strings.ToLower(strings.TrimSpace(format)))
	switch f {
	case Table, JSON, YAML:
		return &Printer{format: f, w: w}, nil
	default:
		return nil, fmt.Errorf("invalid output format %q", format)
	}
}

func (p *Printer) PrintProto(msg proto.Message) error {
	switch p.format {
	case JSON:
		return p.printJSON(msg)
	case YAML:
		return p.printYAML(msg)
	default:
		return fmt.Errorf("PrintProto unsupported for table format")
	}
}

func (p *Printer) PrintPing(reply *enginev1.PingReply) error {
	switch p.format {
	case JSON, YAML:
		return p.printStructured(reply)
	case Table:
		return p.printKeyValue([][2]string{
			{"pong", reply.GetPong()},
			{"server_time", formatUnixMs(reply.GetServerTimeUnixMs())},
		})
	default:
		return fmt.Errorf("unknown format")
	}
}

func (p *Printer) PrintHealth(reply *enginev1.GetHealthReply) error {
	switch p.format {
	case JSON, YAML:
		return p.printStructured(reply)
	case Table:
		if err := p.printKeyValue([][2]string{
			{"status", healthStatusLabel(reply.GetStatus())},
			{"kill_switch_active", fmt.Sprintf("%t", reply.GetKillSwitchActive())},
			{"server_time", formatUnixMs(reply.GetServerTimeUnixMs())},
		}); err != nil {
			return err
		}
		checks := reply.GetChecks()
		if len(checks) == 0 {
			return nil
		}
		if _, err := fmt.Fprintln(p.w); err != nil {
			return err
		}
		tbl := tablewriter.NewWriter(p.w)
		tbl.SetHeader([]string{"name", "status", "message"})
		tbl.SetBorder(true)
		for _, c := range checks {
			tbl.Append([]string{
				c.GetName(),
				componentStatusLabel(c.GetStatus()),
				c.GetMessage(),
			})
		}
		tbl.Render()
		return nil
	default:
		return fmt.Errorf("unknown format")
	}
}

func (p *Printer) PrintEngineInfo(reply *enginev1.GetEngineInfoReply) error {
	switch p.format {
	case JSON, YAML:
		return p.printStructured(reply)
	case Table:
		return p.printKeyValue([][2]string{
			{"version", reply.GetVersion()},
			{"strategies_dir", reply.GetStrategiesDir()},
			{"go_version", reply.GetGoVersion()},
		})
	default:
		return fmt.Errorf("unknown format")
	}
}

func (p *Printer) PrintStrategyList(reply *enginev1.ListStrategiesReply) error {
	switch p.format {
	case JSON, YAML:
		return p.printStructured(reply)
	case Table:
		strategies := append([]*enginev1.StrategySummary(nil), reply.GetStrategies()...)
		sort.Slice(strategies, func(i, j int) bool {
			return strategies[i].GetName() < strategies[j].GetName()
		})
		tbl := tablewriter.NewWriter(p.w)
		tbl.SetHeader([]string{"name", "enabled", "running", "symbols", "crash_count", "crash_window", "circuit_open", "backoff_sec"})
		tbl.SetBorder(true)
		for _, s := range strategies {
			tbl.Append([]string{
				s.GetName(),
				fmt.Sprintf("%t", s.GetEnabled()),
				fmt.Sprintf("%t", s.GetRunning()),
				strings.Join(s.GetSymbols(), ","),
				fmt.Sprintf("%d", s.GetCrashCount()),
				fmt.Sprintf("%d", s.GetCrashCountInWindow()),
				fmt.Sprintf("%t", s.GetCircuitOpen()),
				fmt.Sprintf("%.1f", s.GetRestartBackoffSec()),
			})
		}
		tbl.Render()
		return nil
	default:
		return fmt.Errorf("unknown format")
	}
}

func (p *Printer) PrintStrategyStatus(reply *enginev1.GetStrategyStatusReply) error {
	switch p.format {
	case JSON, YAML:
		return p.printStructured(reply)
	case Table:
		s := reply.GetStatus()
		if s == nil {
			return fmt.Errorf("empty strategy status")
		}
		return p.printKeyValue([][2]string{
			{"name", s.GetName()},
			{"enabled", fmt.Sprintf("%t", s.GetEnabled())},
			{"symbols", strings.Join(s.GetSymbols(), ",")},
			{"running", fmt.Sprintf("%t", s.GetRunning())},
			{"last_heartbeat", formatUnixMs(s.GetLastHeartbeatUnixMs())},
			{"crash_count", fmt.Sprintf("%d", s.GetCrashCount())},
			{"crash_count_in_window", fmt.Sprintf("%d", s.GetCrashCountInWindow())},
			{"auto_restart_enabled", fmt.Sprintf("%t", s.GetAutoRestartEnabled())},
			{"restart_backoff_sec", fmt.Sprintf("%.1f", s.GetRestartBackoffSec())},
			{"circuit_open", fmt.Sprintf("%t", s.GetCircuitOpen())},
		})
	default:
		return fmt.Errorf("unknown format")
	}
}

func (p *Printer) PrintStrategyAction(name, action string) error {
	switch p.format {
	case JSON, YAML:
		doc := map[string]string{"name": name, "action": action}
		if p.format == JSON {
			b, err := json.MarshalIndent(doc, "", "  ")
			if err != nil {
				return err
			}
			b = append(b, '\n')
			_, err = p.w.Write(b)
			return err
		}
		enc := yaml.NewEncoder(p.w)
		enc.SetIndent(2)
		defer enc.Close()
		return enc.Encode(doc)
	case Table:
		return p.printKeyValue([][2]string{
			{"action", action},
			{"name", name},
		})
	default:
		return fmt.Errorf("unknown format")
	}
}

func (p *Printer) PrintReloadResult(count int32) error {
	reply := &enginev1.ReloadStrategiesReply{CatalogCount: count}
	switch p.format {
	case JSON, YAML:
		return p.printStructured(reply)
	case Table:
		return p.printKeyValue([][2]string{
			{"catalog_count", fmt.Sprintf("%d", count)},
		})
	default:
		return fmt.Errorf("unknown format")
	}
}

func (p *Printer) PrintBalance(reply *enginev1.GetBalanceReply) error {
	switch p.format {
	case JSON, YAML:
		return p.printStructured(reply)
	case Table:
		b := reply.GetBalance()
		if b == nil {
			return p.printKeyValue([][2]string{{"balance", "(none)"}})
		}
		return p.printKeyValue([][2]string{
			{"currency", b.GetCurrency()},
			{"total", b.GetTotal()},
			{"available", b.GetAvailable()},
			{"frozen", b.GetFrozen()},
			{"updated_at", formatUnixMs(b.GetUpdatedAtUnixMs())},
		})
	default:
		return fmt.Errorf("unknown format")
	}
}

func (p *Printer) PrintPosition(reply *enginev1.GetPositionReply) error {
	switch p.format {
	case JSON, YAML:
		return p.printStructured(reply)
	case Table:
		pos := reply.GetPosition()
		if pos == nil {
			return p.printKeyValue([][2]string{{"position", "(none)"}})
		}
		return p.printKeyValue([][2]string{
			{"symbol", pos.GetSymbol()},
			{"side", pos.GetSide()},
			{"size", pos.GetSize()},
			{"entry_price", pos.GetEntryPrice()},
			{"mark_price", pos.GetMarkPrice()},
			{"unrealized_pnl", pos.GetUnrealizedPnl()},
			{"leverage", fmt.Sprintf("%d", pos.GetLeverage())},
			{"updated_at", formatUnixMs(pos.GetUpdatedAtUnixMs())},
		})
	default:
		return fmt.Errorf("unknown format")
	}
}

func (p *Printer) PrintPositionList(reply *enginev1.ListPositionsReply) error {
	switch p.format {
	case JSON, YAML:
		return p.printStructured(reply)
	case Table:
		positions := append([]*enginev1.Position(nil), reply.GetPositions()...)
		sort.Slice(positions, func(i, j int) bool {
			return positions[i].GetSymbol() < positions[j].GetSymbol()
		})
		tbl := tablewriter.NewWriter(p.w)
		tbl.SetHeader([]string{"symbol", "side", "size", "entry_price", "mark_price", "unrealized_pnl", "leverage"})
		tbl.SetBorder(true)
		for _, pos := range positions {
			tbl.Append([]string{
				pos.GetSymbol(),
				pos.GetSide(),
				pos.GetSize(),
				pos.GetEntryPrice(),
				pos.GetMarkPrice(),
				pos.GetUnrealizedPnl(),
				fmt.Sprintf("%d", pos.GetLeverage()),
			})
		}
		tbl.Render()
		return nil
	default:
		return fmt.Errorf("unknown format")
	}
}

func (p *Printer) PrintOrder(reply *enginev1.GetOrderReply) error {
	switch p.format {
	case JSON, YAML:
		return p.printStructured(reply)
	case Table:
		o := reply.GetOrder()
		if o == nil {
			return fmt.Errorf("empty order")
		}
		return p.printKeyValue(orderKeyValues(o))
	default:
		return fmt.Errorf("unknown format")
	}
}

func (p *Printer) PrintOrderList(reply *enginev1.ListOpenOrdersReply) error {
	switch p.format {
	case JSON, YAML:
		return p.printStructured(reply)
	case Table:
		orders := append([]*enginev1.Order(nil), reply.GetOrders()...)
		sort.Slice(orders, func(i, j int) bool {
			return orders[i].GetUpdatedAtUnixMs() > orders[j].GetUpdatedAtUnixMs()
		})
		tbl := tablewriter.NewWriter(p.w)
		tbl.SetHeader([]string{"id", "symbol", "side", "status", "size", "filled_size", "strategy_name", "updated_at"})
		tbl.SetBorder(true)
		for _, o := range orders {
			tbl.Append([]string{
				o.GetId(),
				o.GetSymbol(),
				o.GetSide(),
				o.GetStatus(),
				o.GetSize(),
				o.GetFilledSize(),
				o.GetStrategyName(),
				formatUnixMs(o.GetUpdatedAtUnixMs()),
			})
		}
		tbl.Render()
		return nil
	default:
		return fmt.Errorf("unknown format")
	}
}

func (p *Printer) PrintOrderCancel(orderID string) error {
	switch p.format {
	case JSON, YAML:
		doc := map[string]string{"order_id": orderID, "action": "cancelled"}
		if p.format == JSON {
			b, err := json.MarshalIndent(doc, "", "  ")
			if err != nil {
				return err
			}
			b = append(b, '\n')
			_, err = p.w.Write(b)
			return err
		}
		enc := yaml.NewEncoder(p.w)
		enc.SetIndent(2)
		defer enc.Close()
		return enc.Encode(doc)
	case Table:
		return p.printKeyValue([][2]string{
			{"action", "cancelled"},
			{"order_id", orderID},
		})
	default:
		return fmt.Errorf("unknown format")
	}
}

// UsesOrderWatchNDJSON reports whether order watch emits NDJSON lines.
func (p *Printer) UsesOrderWatchNDJSON() bool {
	return p.format == JSON || p.format == YAML
}

func (p *Printer) PrintOrderEventHeader() error {
	if p.format != Table {
		return nil
	}
	_, err := fmt.Fprintln(p.w, strings.Join([]string{
		"id", "symbol", "side", "status", "size", "filled_size", "updated_at",
	}, "\t"))
	return err
}

func (p *Printer) PrintOrderEvent(order *enginev1.Order) error {
	if order == nil {
		return nil
	}
	if p.UsesOrderWatchNDJSON() {
		return p.printOrderNDJSON(order)
	}
	_, err := fmt.Fprintln(p.w, strings.Join([]string{
		order.GetId(),
		order.GetSymbol(),
		order.GetSide(),
		order.GetStatus(),
		order.GetSize(),
		order.GetFilledSize(),
		formatUnixMs(order.GetUpdatedAtUnixMs()),
	}, "\t"))
	return err
}

func (p *Printer) printOrderNDJSON(order *enginev1.Order) error {
	b, err := protoMarshal.Marshal(order)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = p.w.Write(b)
	return err
}

func (p *Printer) PrintTicker(reply *enginev1.GetTickerReply) error {
	switch p.format {
	case JSON, YAML:
		return p.printStructured(reply)
	case Table:
		t := reply.GetTicker()
		if t == nil {
			return fmt.Errorf("empty ticker")
		}
		return p.printKeyValue([][2]string{
			{"symbol", t.GetSymbol()},
			{"last", t.GetLast()},
			{"mark_price", t.GetMarkPrice()},
			{"index_price", t.GetIndexPrice()},
			{"funding_rate", t.GetFundingRate()},
			{"change_pct_24h", t.GetChangePct_24H()},
			{"volume_24h", t.GetVolume_24H()},
			{"open_interest", t.GetOpenInterest()},
			{"high_24h", t.GetHigh_24H()},
			{"low_24h", t.GetLow_24H()},
			{"timestamp", formatUnixMs(t.GetTimestampUnixMs())},
		})
	default:
		return fmt.Errorf("unknown format")
	}
}

func (p *Printer) PrintKlines(reply *enginev1.GetKlinesReply) error {
	switch p.format {
	case JSON, YAML:
		return p.printStructured(reply)
	case Table:
		tbl := tablewriter.NewWriter(p.w)
		tbl.SetHeader([]string{"timestamp", "open", "high", "low", "close", "volume", "closed"})
		tbl.SetBorder(true)
		for _, k := range reply.GetKlines() {
			tbl.Append([]string{
				formatUnixSec(k.GetTimestampUnixSec()),
				k.GetOpen(),
				k.GetHigh(),
				k.GetLow(),
				k.GetClose(),
				k.GetVolume(),
				fmt.Sprintf("%t", k.GetWindowClosed()),
			})
		}
		tbl.Render()
		return nil
	default:
		return fmt.Errorf("unknown format")
	}
}

func (p *Printer) PrintTickerList(reply *enginev1.ListTickersReply) error {
	switch p.format {
	case JSON, YAML:
		return p.printStructured(reply)
	case Table:
		tickers := append([]*enginev1.Ticker(nil), reply.GetTickers()...)
		sort.Slice(tickers, func(i, j int) bool {
			return tickers[i].GetSymbol() < tickers[j].GetSymbol()
		})
		tbl := tablewriter.NewWriter(p.w)
		tbl.SetHeader([]string{"symbol", "last", "mark_price", "change_pct_24h", "volume_24h", "timestamp"})
		tbl.SetBorder(true)
		for _, t := range tickers {
			tbl.Append([]string{
				t.GetSymbol(),
				t.GetLast(),
				t.GetMarkPrice(),
				t.GetChangePct_24H(),
				t.GetVolume_24H(),
				formatUnixMs(t.GetTimestampUnixMs()),
			})
		}
		tbl.Render()
		return nil
	default:
		return fmt.Errorf("unknown format")
	}
}

func (p *Printer) PrintKillSwitchStatus(reply *enginev1.GetKillSwitchStatusReply) error {
	switch p.format {
	case JSON, YAML:
		return p.printStructured(reply)
	case Table:
		return p.printKeyValue(killSwitchStatusRows(reply.GetStatus()))
	default:
		return fmt.Errorf("unknown format")
	}
}

func (p *Printer) PrintActivateKillSwitch(reply *enginev1.ActivateKillSwitchReply) error {
	switch p.format {
	case JSON, YAML:
		return p.printStructured(reply)
	case Table:
		rows := killSwitchStatusRows(reply.GetStatus())
		rows = append(rows,
			[2]string{"cancel_attempted", fmt.Sprintf("%d", reply.GetCancelAttempted())},
			[2]string{"cancel_failed", fmt.Sprintf("%d", reply.GetCancelFailed())},
		)
		return p.printKeyValue(rows)
	default:
		return fmt.Errorf("unknown format")
	}
}

func (p *Printer) PrintDeactivateKillSwitch(reply *enginev1.DeactivateKillSwitchReply) error {
	switch p.format {
	case JSON, YAML:
		return p.printStructured(reply)
	case Table:
		return p.printKeyValue(killSwitchStatusRows(reply.GetStatus()))
	default:
		return fmt.Errorf("unknown format")
	}
}

func killSwitchStatusRows(st *enginev1.KillSwitchStatus) [][2]string {
	if st == nil {
		return [][2]string{{"active", "false"}}
	}
	return [][2]string{
		{"active", fmt.Sprintf("%t", st.GetActive())},
		{"reason", st.GetReason()},
		{"activated_at", formatUnixMs(st.GetActivatedAtUnixMs())},
	}
}

func orderKeyValues(o *enginev1.Order) [][2]string {
	return [][2]string{
		{"id", o.GetId()},
		{"exchange_id", o.GetExchangeId()},
		{"symbol", o.GetSymbol()},
		{"side", o.GetSide()},
		{"order_type", o.GetOrderType()},
		{"size", o.GetSize()},
		{"filled_size", o.GetFilledSize()},
		{"status", o.GetStatus()},
		{"price", o.GetPrice()},
		{"stop_price", o.GetStopPrice()},
		{"strategy_name", o.GetStrategyName()},
		{"created_at", formatUnixMs(o.GetCreatedAtUnixMs())},
		{"updated_at", formatUnixMs(o.GetUpdatedAtUnixMs())},
	}
}

func (p *Printer) printStructured(msg proto.Message) error {
	switch p.format {
	case JSON:
		return p.printJSON(msg)
	case YAML:
		return p.printYAML(msg)
	default:
		return fmt.Errorf("unknown format")
	}
}

func (p *Printer) printJSON(msg proto.Message) error {
	b, err := protoMarshal.Marshal(msg)
	if err != nil {
		return err
	}
	var indented json.RawMessage
	if err := json.Unmarshal(b, &indented); err != nil {
		return err
	}
	pretty, err := json.MarshalIndent(indented, "", "  ")
	if err != nil {
		return err
	}
	pretty = append(pretty, '\n')
	_, err = p.w.Write(pretty)
	return err
}

func (p *Printer) printYAML(msg proto.Message) error {
	b, err := protoMarshal.Marshal(msg)
	if err != nil {
		return err
	}
	var doc any
	if err := json.Unmarshal(b, &doc); err != nil {
		return err
	}
	enc := yaml.NewEncoder(p.w)
	enc.SetIndent(2)
	defer enc.Close()
	return enc.Encode(doc)
}

func (p *Printer) printKeyValue(rows [][2]string) error {
	tbl := tablewriter.NewWriter(p.w)
	tbl.SetHeader([]string{"FIELD", "VALUE"})
	tbl.SetBorder(true)
	for _, row := range rows {
		tbl.Append([]string{row[0], row[1]})
	}
	tbl.Render()
	return nil
}

func formatUnixMs(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339Nano)
}

func formatUnixSec(sec int64) string {
	if sec <= 0 {
		return ""
	}
	return time.Unix(sec, 0).UTC().Format(time.RFC3339)
}

func healthStatusLabel(s enginev1.HealthStatus) string {
	name := enginev1.HealthStatus_name[int32(s)]
	name = strings.TrimPrefix(name, "HEALTH_STATUS_")
	return strings.ToLower(name)
}

func componentStatusLabel(s enginev1.ComponentStatus) string {
	name := enginev1.ComponentStatus_name[int32(s)]
	name = strings.TrimPrefix(name, "COMPONENT_STATUS_")
	return strings.ToLower(name)
}

// HealthProbeOK reports whether status is HEALTHY (for --probe exit code).
func HealthProbeOK(status enginev1.HealthStatus) bool {
	return status == enginev1.HealthStatus_HEALTH_STATUS_HEALTHY
}
