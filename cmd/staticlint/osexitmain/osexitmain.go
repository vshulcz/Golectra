// Package osexitmain defines an analyzer that reports direct calls to os.Exit in the main.main function.
package osexitmain

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the osexitmain analyzer.
var Analyzer = &analysis.Analyzer{
	Name:     "osexitmain",
	Doc:      "reports direct os.Exit calls in main.main",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// run is the main function of the analyzer.
func run(pass *analysis.Pass) (any, error) {
	if pass.Pkg == nil || pass.Pkg.Name() != "main" {
		return nil, nil
	}

	insp, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, fmt.Errorf("failed to assert type: expected *inspector.Inspector")
	}

	insp.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
		fd, ok := n.(*ast.FuncDecl)
		if !ok || fd.Recv != nil || fd.Name == nil || fd.Name.Name != "main" || fd.Body == nil {
			return
		}

		ast.Inspect(fd.Body, func(nn ast.Node) bool {
			switch x := nn.(type) {
			case *ast.FuncLit:
				return false
			case *ast.CallExpr:
				if isOsExitCall(pass, x) {
					pass.Reportf(x.Pos(), "It is forbidden to call os.Exit directly in main function; use return code from main instead")
				}
			}
			return true
		})
	})

	return nil, nil
}

// isOsExitCall checks if the given call expression is a call to os.Exit.
func isOsExitCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	if call == nil || call.Fun == nil {
		return false
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil || sel.X == nil {
		return false
	}

	if pass.TypesInfo == nil || pass.TypesInfo.Uses == nil {
		return false
	}

	obj := pass.TypesInfo.Uses[sel.Sel]
	fn, ok := obj.(*types.Func)
	if !ok || fn.Pkg() == nil {
		return false
	}

	return fn.Pkg().Path() == "os" && fn.Name() == "Exit"
}
