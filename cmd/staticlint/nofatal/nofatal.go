// Package nofatal реализует анализатор, запрещающий panic и
// log.Fatal/log.Panic вне функций main и init.
//
// Мотивация: panic и log.Fatal (последний вызывает os.Exit) аварийно
// завершают процесс, обходя отложенные функции (defer) и graceful shutdown.
// В библиотечном коде это недопустимо — функции должны возвращать ошибки, а
// решение о завершении принимает точка входа. Единственный легальный «сток»
// аварийного выхода — тело func main (пакет main) и func init (любой пакет);
// связка с анализатором osexit (который запрещает os.Exit в main) даёт цельное
// правило: os.Exit — нельзя нигде, panic/log.Fatal — только в точках входа.
//
// Что считается нарушением:
//   - вызов встроенного panic (по type info, а не по имени — локальная функция
//     с именем panic не ловится);
//   - вызов log.Fatal/Fatalf/Fatalln и log.Panic/Panicf/Panicln из стандартного
//     пакета log (по type info);
//   - находящийся в теле любой функции, кроме func main пакета main и func init.
//     Вложенные литералы и горутины считаются частью объемлющей функции.
package nofatal

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Analyzer запрещает panic и log.Fatal/log.Panic вне main и init.
var Analyzer = &analysis.Analyzer{
	Name: "nofatal",
	Doc:  "запрещает panic и log.Fatal/log.Panic вне функций main и init",
	Run:  run,
}

// logTerminators — методы пакета log, аварийно завершающие процесс.
var logTerminators = map[string]bool{
	"Fatal": true, "Fatalf": true, "Fatalln": true,
	"Panic": true, "Panicf": true, "Panicln": true,
}

func run(pass *analysis.Pass) (any, error) {
	isMainPkg := pass.Pkg.Name() == "main"

	for _, file := range pass.Files {
		if isGenerated(file) {
			continue
		}
		// Тесты не покрываем.
		if name := pass.Fset.Position(file.Pos()).Filename; strings.HasSuffix(name, "_test.go") {
			continue
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if isEntrypoint(fn, isMainPkg) {
				continue
			}
			checkBody(pass, fn.Body)
		}
	}
	return nil, nil
}

// isEntrypoint сообщает, является ли функция легальным «стоком» аварийного
// выхода: func main пакета main или func init любого пакета (оба без receiver).
func isEntrypoint(fn *ast.FuncDecl, isMainPkg bool) bool {
	if fn.Recv != nil {
		return false
	}
	if fn.Name.Name == "init" {
		return true
	}
	return isMainPkg && fn.Name.Name == "main"
}

func checkBody(pass *analysis.Pass, body *ast.BlockStmt) {
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			// Встроенный panic; локальную функцию с именем panic пропускаем.
			if fun.Name == "panic" {
				if _, ok := pass.TypesInfo.Uses[fun].(*types.Builtin); ok {
					pass.Reportf(call.Pos(), "panic вне main/init запрещён")
				}
			}
		case *ast.SelectorExpr:
			name := fun.Sel.Name
			if !logTerminators[name] {
				return true
			}
			obj, ok := pass.TypesInfo.Uses[fun.Sel].(*types.Func)
			if !ok || obj.Pkg() == nil || obj.Pkg().Path() != "log" {
				return true
			}
			pass.Reportf(call.Pos(), "log.%s вне main/init запрещён", name)
		}
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
