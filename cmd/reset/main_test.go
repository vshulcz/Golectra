package main

import (
	"bytes"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestFindModuleRoot(t *testing.T) {
	root, err := findModuleRoot()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "go.mod")); os.IsNotExist(err) {
		t.Errorf("expected go.mod file in module root, but not found")
	}
}

func TestGenerateResetFile(t *testing.T) {
	pkg := &packages.Package{
		Name:    "testpkg",
		PkgPath: "github.com/example/testpkg",
	}

	targets := []targetStruct{
		{
			Name: "TestStruct",
			Named: types.NewNamed(
				types.NewTypeName(0, nil, "TestStruct", nil),
				types.NewStruct(nil, nil),
				nil,
			),
		},
	}

	result, err := generateResetFile(pkg, targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Contains(result, []byte("package testpkg")) {
		t.Errorf("expected package declaration in generated file")
	}
}

func TestHasMarker(t *testing.T) {
	comment := &ast.CommentGroup{
		List: []*ast.Comment{
			{Text: "// +generate:reset"},
		},
	}

	if !hasMarker(comment, nil) {
		t.Errorf("expected marker to be detected")
	}
}

func TestHasMarkerNoMarker(t *testing.T) {
	comment := &ast.CommentGroup{
		List: []*ast.Comment{
			{Text: "// regular comment"},
		},
	}

	if hasMarker(comment, nil) {
		t.Errorf("expected no marker to be detected")
	}
}

func TestHasMarkerBothDocs(t *testing.T) {
	genDoc := &ast.CommentGroup{
		List: []*ast.Comment{
			{Text: "// +generate:reset"},
		},
	}

	if !hasMarker(genDoc, nil) {
		t.Errorf("expected marker in genDoc to be detected")
	}
}

func TestZeroBasic(t *testing.T) {
	tests := []struct {
		name     string
		kind     types.BasicKind
		expected string
	}{
		{
			name:     "bool zero",
			kind:     types.Bool,
			expected: "false",
		},
		{
			name:     "string zero",
			kind:     types.String,
			expected: `""`,
		},
		{
			name:     "int zero",
			kind:     types.Int,
			expected: "0",
		},
		{
			name:     "float64 zero",
			kind:     types.Float64,
			expected: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			basic := types.Typ[tt.kind]
			result := zeroBasic(basic)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestZeroExpr(t *testing.T) {
	im := newImportManager("test")

	tests := []struct {
		name     string
		typ      types.Type
		expected string
	}{
		{
			name:     "basic nil",
			typ:      types.Typ[types.UntypedNil],
			expected: "nil",
		},
		{
			name:     "slice",
			typ:      types.NewSlice(types.Typ[types.Int]),
			expected: "nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := zeroExpr(im, tt.typ)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
