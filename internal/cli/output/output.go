package output

import (
	"encoding/json"
	"fmt"
	"io"
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
