// Package osexit реализует анализатор, запрещающий прямой вызов os.Exit
// в теле функции main пакета main.
//
// Мотивация: ранний выход через os.Exit обходит отложенные функции (defer),
// мешает корректному завершению (закрытие пулов, сброс буферов, остановка
// горутин по сигналу). В функции main принято возвращать управление обычным
// путём, а коды возврата выставлять через возвращаемое значение запускаемой
// процедуры или через panic в самом крайнем случае.
//
// Что считается «прямым вызовом»:
//   - вызов идентификатора, который через type info соответствует функции
//     os.Exit из стандартной библиотеки (импорт под алиасом учитывается);
//   - находится непосредственно в теле функции main пакета main (вложенные
//     функции, объявленные внутри main, тоже подпадают — это часть main).
package osexit

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Analyzer запрещает прямой вызов os.Exit в функции main пакета main.
var Analyzer = &analysis.Analyzer{
	Name: "osexit",
	Doc:  "запрещает прямой вызов os.Exit в функции main пакета main",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	if pass.Pkg.Name() != "main" {
		return nil, nil
	}

	for _, file := range pass.Files {
		if isGenerated(file) {
			continue
		}
		// Тесты в пакете main не покрываем.
		if name := pass.Fset.Position(file.Pos()).Filename; strings.HasSuffix(name, "_test.go") {
			continue
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name.Name != "main" || fn.Body == nil {
				continue
			}
			checkBody(pass, fn.Body)
		}
	}
	return nil, nil
}

func checkBody(pass *analysis.Pass, body *ast.BlockStmt) {
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		obj, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
		if !ok {
			return true
		}
		if obj.Pkg() == nil || obj.Pkg().Path() != "os" || obj.Name() != "Exit" {
			return true
		}
		pass.Reportf(call.Pos(), "прямой вызов os.Exit в функции main запрещён")
		return true
	})
}

// isGenerated возвращает true, если файл содержит маркер сгенерированного кода
// согласно соглашению https://golang.org/s/generatedcode.
func isGenerated(file *ast.File) bool {
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if strings.HasPrefix(c.Text, "// Code generated") &&
				strings.Contains(c.Text, "DO NOT EDIT.") {
				return true
			}
		}
	}
	return false
}
