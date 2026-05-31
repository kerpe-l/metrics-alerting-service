package osexit_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kerpe-l/metrics-alerting-service/cmd/staticlint/osexit"
)

// TestOSExit прогоняет анализатор по подготовленным пакетам testdata/src/*.
//
// Раскладка:
//   - a — main с прямым os.Exit (срабатывает);
//   - b — os.Exit вынесен в отдельную функцию (не срабатывает);
//   - c — пакет не main (пропуск);
//   - d — main без os.Exit (тишина);
//   - e — импорт os под алиасом (срабатывает, детект по type info).
func TestOSExit(t *testing.T) {
	t.Parallel()
	analysistest.Run(t, analysistest.TestData(), osexit.Analyzer, "a", "b", "c", "d", "e")
}
