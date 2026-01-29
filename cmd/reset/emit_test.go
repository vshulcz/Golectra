package main

import (
	"bytes"
	"go/types"
	"strings"
	"testing"
)

func TestCodeWriterLine(t *testing.T) {
	w := &codeWriter{}
	w.line(2, "hello")
	if w.buf == nil {
		t.Fatal("expected buffer to be initialized")
	}
	if got := w.buf.String(); got != "\t\thello\n" {
		t.Fatalf("output=%q", got)
	}
}

func TestEmitResetForExpr_BasicSliceMap(t *testing.T) {
	im := newImportManager("local")
	cases := []struct {
		name string
		typ  types.Type
		want string
	}{
		{name: "basic", typ: types.Typ[types.Int], want: "x = 0"},
		{name: "slice", typ: types.NewSlice(types.Typ[types.Int]), want: "x = x[:0]"},
		{name: "map", typ: types.NewMap(types.Typ[types.String], types.Typ[types.Int]), want: "clear(x)"},
	}
	for _, tc := range cases {
		w := &codeWriter{buf: &bytes.Buffer{}}
		emitResetForExpr(w, im, "local", "x", tc.typ)
		if !strings.Contains(w.buf.String(), tc.want) {
			t.Fatalf("%s: expected %q in %q", tc.name, tc.want, w.buf.String())
		}
	}
}

func TestEmitResetForExpr_PointerBasic(t *testing.T) {
	im := newImportManager("local")
	w := &codeWriter{buf: &bytes.Buffer{}}
	emitResetForExpr(w, im, "local", "x", types.NewPointer(types.Typ[types.Int]))
	out := w.buf.String()
	if !strings.Contains(out, "if x != nil") || !strings.Contains(out, "*x = 0") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestEmitResetForExpr_StructWithResetMethod(t *testing.T) {
	pkg := types.NewPackage("local", "local")
	obj := types.NewTypeName(0, pkg, "WithReset", nil)
	st := types.NewStruct(nil, nil)
	named := types.NewNamed(obj, st, nil)
	recv := types.NewVar(0, pkg, "", types.NewPointer(named))
	resetFn := types.NewFunc(0, pkg, "Reset", types.NewSignatureType(recv, nil, nil, nil, nil, false))
	named.AddMethod(resetFn)

	im := newImportManager("local")
	w := &codeWriter{buf: &bytes.Buffer{}}
	emitResetForExpr(w, im, "local", "x", named)
	if !strings.Contains(w.buf.String(), "(&x).Reset()") {
		t.Fatalf("expected Reset call, got %q", w.buf.String())
	}
}

func TestEmitResetForExpr_AccessibleStructFields(t *testing.T) {
	pkg := types.NewPackage("local", "local")
	field := types.NewField(0, pkg, "Field", types.Typ[types.String], false)
	st := types.NewStruct([]*types.Var{field}, nil)
	obj := types.NewTypeName(0, pkg, "Local", nil)
	named := types.NewNamed(obj, st, nil)

	im := newImportManager("local")
	w := &codeWriter{buf: &bytes.Buffer{}}
	emitResetForExpr(w, im, "local", "x", named)
	if !strings.Contains(w.buf.String(), "x.Field = \"\"") {
		t.Fatalf("unexpected output: %q", w.buf.String())
	}
}

func TestEmitResetForExpr_PointerStructWithReset(t *testing.T) {
	pkg := types.NewPackage("local", "local")
	obj := types.NewTypeName(0, pkg, "PtrReset", nil)
	st := types.NewStruct(nil, nil)
	named := types.NewNamed(obj, st, nil)
	recv := types.NewVar(0, pkg, "", types.NewPointer(named))
	resetFn := types.NewFunc(0, pkg, "Reset", types.NewSignatureType(recv, nil, nil, nil, nil, false))
	named.AddMethod(resetFn)

	im := newImportManager("local")
	w := &codeWriter{buf: &bytes.Buffer{}}
	emitResetForExpr(w, im, "local", "x", types.NewPointer(named))
	if !strings.Contains(w.buf.String(), "x.Reset()") {
		t.Fatalf("expected pointer Reset call, got %q", w.buf.String())
	}
}

func TestImportManagerSortedImports(t *testing.T) {
	im := newImportManager("local")
	pkg1 := types.NewPackage("example.com/foo", "foo")
	pkg2 := types.NewPackage("example.com/bar", "bar")
	_ = im.qualifier(pkg1)
	_ = im.qualifier(pkg2)
	imports := im.sortedImports()
	if len(imports) != 2 {
		t.Fatalf("expected 2 imports, got %d", len(imports))
	}
	if imports[0].Path != "example.com/bar" || imports[1].Path != "example.com/foo" {
		t.Fatalf("unexpected import order: %+v", imports)
	}
}
