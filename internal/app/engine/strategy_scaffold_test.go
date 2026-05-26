package engine

import (
	"errors"
	"testing"

	"github.com/kainhuck/signalix/internal/app/strategy/scaffold"
	"github.com/kainhuck/signalix/internal/testutil"
)

func TestCreateStrategyScaffold_reload(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	e := NewEngine(dir, testutil.NewStubExchange(), BuildParams{})
	if err := e.LoadStrategies(); err != nil {
		t.Fatal(err)
	}
	path, n, err := e.CreateStrategyScaffold(scaffold.CreateOptions{
		Name:       "new_one",
		TemplateID: "blank",
	})
	if err != nil {
		t.Fatal(err)
	}
	if path == "" || n != 1 {
		t.Fatalf("path=%q n=%d", path, n)
	}
	st, ok := e.GetStrategy("new_one")
	if !ok || st == nil {
		t.Fatal("strategy not in catalog")
	}
	if st.Enabled {
		t.Fatal("expected disabled")
	}
}

func TestCreateStrategyScaffold_noLoader(t *testing.T) {
	t.Parallel()
	e := &Engine{}
	_, _, err := e.CreateStrategyScaffold(scaffold.CreateOptions{Name: "x", TemplateID: "blank"})
	if !errors.Is(err, ErrStrategyLoaderNotConfigured) {
		t.Fatalf("err = %v", err)
	}
}
