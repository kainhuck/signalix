package output

import (
	"fmt"
	"strings"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/olekukonko/tablewriter"
)

func (p *Printer) PrintStrategyLogList(reply *enginev1.ListStrategyLogsReply) error {
	switch p.format {
	case JSON, YAML:
		return p.printStructured(reply)
	case Table:
		tbl := tablewriter.NewWriter(p.w)
		tbl.SetHeader([]string{"id", "strategy", "level", "created_at", "message"})
		tbl.SetBorder(true)
		for _, log := range reply.GetLogs() {
			if log == nil {
				continue
			}
			tbl.Append([]string{
				fmt.Sprintf("%d", log.GetId()),
				log.GetStrategyName(),
				log.GetLevel(),
				formatUnixMs(log.GetCreatedAtUnixMs()),
				log.GetMessage(),
			})
		}
		tbl.Render()
		return nil
	default:
		return fmt.Errorf("unknown format")
	}
}

func (p *Printer) UsesStrategyLogWatchNDJSON() bool {
	return p.format == JSON || p.format == YAML
}

func (p *Printer) PrintStrategyLogEventHeader() error {
	if p.format != Table {
		return nil
	}
	_, err := fmt.Fprintln(p.w, strings.Join([]string{
		"id", "strategy", "level", "created_at", "message",
	}, "\t"))
	return err
}

func (p *Printer) PrintStrategyLogEvent(log *enginev1.StrategyLog) error {
	if log == nil {
		return nil
	}
	if p.UsesStrategyLogWatchNDJSON() {
		return p.printStrategyLogNDJSON(log)
	}
	_, err := fmt.Fprintln(p.w, strings.Join([]string{
		fmt.Sprintf("%d", log.GetId()),
		log.GetStrategyName(),
		log.GetLevel(),
		formatUnixMs(log.GetCreatedAtUnixMs()),
		log.GetMessage(),
	}, "\t"))
	return err
}

func (p *Printer) printStrategyLogNDJSON(log *enginev1.StrategyLog) error {
	b, err := protoMarshal.Marshal(log)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = p.w.Write(b)
	return err
}

func (p *Printer) PrintAccountSnapshotList(reply *enginev1.ListAccountSnapshotsReply) error {
	switch p.format {
	case JSON, YAML:
		return p.printStructured(reply)
	case Table:
		tbl := tablewriter.NewWriter(p.w)
		tbl.SetHeader([]string{"snapshot_at", "total_equity", "currency", "revision", "id"})
		tbl.SetBorder(true)
		for _, snap := range reply.GetSnapshots() {
			sum := snap.GetSummary()
			if sum == nil {
				continue
			}
			tbl.Append([]string{
				formatUnixMs(sum.GetSnapshotAtUnixMs()),
				sum.GetTotalEquity(),
				sum.GetCurrency(),
				fmt.Sprintf("%d", sum.GetRevision()),
				fmt.Sprintf("%d", sum.GetId()),
			})
		}
		tbl.Render()
		return nil
	default:
		return fmt.Errorf("unknown format")
	}
}

func (p *Printer) PrintLatestAccountSnapshot(reply *enginev1.GetLatestAccountSnapshotReply) error {
	switch p.format {
	case JSON, YAML:
		return p.printStructured(reply)
	case Table:
		snap := reply.GetSnapshot()
		if snap == nil || snap.GetSummary() == nil {
			return p.printKeyValue([][2]string{{"snapshot", "(none)"}})
		}
		sum := snap.GetSummary()
		return p.printKeyValue([][2]string{
			{"id", fmt.Sprintf("%d", sum.GetId())},
			{"snapshot_at", formatUnixMs(sum.GetSnapshotAtUnixMs())},
			{"total_equity", sum.GetTotalEquity()},
			{"currency", sum.GetCurrency()},
			{"revision", fmt.Sprintf("%d", sum.GetRevision())},
		})
	default:
		return fmt.Errorf("unknown format")
	}
}
