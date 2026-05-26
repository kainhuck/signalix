package cmd

import (
	"fmt"
	"strings"
	"time"
)

func parseOptionalTimeFlag(name, value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		t, err = time.Parse(time.RFC3339, value)
	}
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}
	return t.UTC().UnixMilli(), nil
}

func parseListTimeFlags(since, until string) (startMs, endMs int64, err error) {
	startMs, err = parseOptionalTimeFlag("--since", since)
	if err != nil {
		return 0, 0, err
	}
	endMs, err = parseOptionalTimeFlag("--until", until)
	if err != nil {
		return 0, 0, err
	}
	return startMs, endMs, nil
}
