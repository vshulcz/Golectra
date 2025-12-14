package osexitmain

import (
	"go/ast"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

func mockPass(pkgName string, nodes []ast.Node) *analysis.Pass {
	return &analysis.Pass{
		Pkg: types.NewPackage(pkgName, ""),
		ResultOf: map[*analysis.Analyzer]any{
			inspect.Analyzer: &mockInspector{nodes: nodes},
		},
	}
}

type mockInspector struct {
	nodes []ast.Node
}

func (m *mockInspector) Preorder(_ []ast.Node, fn func(ast.Node)) {
	for _, n := range m.nodes {
		fn(n)
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name    string
		pkgName string
		nodes   []ast.Node
	}{
		{
			name:    "no os.Exit call",
			pkgName: "main",
			nodes: []ast.Node{
				&ast.FuncDecl{
					Name: &ast.Ident{Name: "main"},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ExprStmt{
								X: &ast.BasicLit{Kind: token.STRING, Value: "\"hello\""},
							},
						},
					},
				},
			},
		},
		{
			name:    "os.Exit call in main",
			pkgName: "main",
			nodes: []ast.Node{
				&ast.FuncDecl{
					Name: &ast.Ident{Name: "main"},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: &ast.SelectorExpr{
										X:   &ast.Ident{Name: "os"},
										Sel: &ast.Ident{Name: "Exit"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pass := mockPass(tt.pkgName, tt.nodes)
			_, err := run(pass)
			if err != nil {
				t.Errorf("run() returned unexpected error: %v", err)
			}
		})
	}
}

func TestIsOsExitCall(t *testing.T) {
	tests := []struct {
		name      string
		callExpr  *ast.CallExpr
		expectRes bool
	}{
		{
			name: "valid os.Exit call",
			callExpr: &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   &ast.Ident{Name: "os"},
					Sel: &ast.Ident{Name: "Exit"},
				},
			},
			expectRes: true,
		},
		{
			name: "non os.Exit call",
			callExpr: &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   &ast.Ident{Name: "fmt"},
					Sel: &ast.Ident{Name: "Println"},
				},
			},
			expectRes: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pass := &analysis.Pass{
				TypesInfo: &types.Info{
					Uses: map[*ast.Ident]types.Object{
						tt.callExpr.Fun.(*ast.SelectorExpr).Sel: types.NewFunc(0, types.NewPackage("os", "os"), "Exit", types.NewSignatureType(nil, nil, nil, nil, nil, false)),
					},
				},
			}
			if tt.name == "non os.Exit call" {
				pass.TypesInfo.Uses[tt.callExpr.Fun.(*ast.SelectorExpr).Sel] = types.NewFunc(0, types.NewPackage("fmt", "fmt"), "Println", types.NewSignatureType(nil, nil, nil, nil, nil, false))
			}
			res := isOsExitCall(pass, tt.callExpr)
			if res != tt.expectRes {
				t.Errorf("isOsExitCall() = %v, expectRes %v", res, tt.expectRes)
			}
		})
	}
}
