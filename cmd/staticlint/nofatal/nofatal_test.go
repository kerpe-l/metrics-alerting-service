package nofatal_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kerpe-l/metrics-alerting-service/cmd/staticlint/nofatal"
)

// TestNoFatal прогоняет анализатор по подготовленным пакетам testdata/src/*.
//
// Раскладка:
//   - a — библиотечная функция с panic (срабатывает);
//   - b — библиотечная функция с log.Fatal/Fatalf/Panicln (срабатывает);
//   - c — func main пакета main с panic и log.Fatal (тишина — точка входа);
//   - d — func init с panic (тишина — точка входа);
//   - e — чистая библиотечная функция (тишина);
//   - f — метод с receiver и panic (срабатывает — метод не main/init);
//   - g — panic во вложенном литерале внутри main (тишина — часть main).
func TestNoFatal(t *testing.T) {
	t.Parallel()
	analysistest.Run(t, analysistest.TestData(), nofatal.Analyzer,
		"a", "b", "c", "d", "e", "f", "g")
}
