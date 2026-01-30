package main

import (
	"bytes"
	"go/types"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestEmitResetForExpr_ArrayAndStructFallback(t *testing.T) {
	im := newImportManager("local")

	array := types.NewArray(types.Typ[types.Int], 3)
	w := &codeWriter{buf: &bytes.Buffer{}}
	emitResetForExpr(w, im, "local", "x", array)
	if !strings.Contains(w.buf.String(), "x = [3]int{}") {
		t.Fatalf("array output: %q", w.buf.String())
	}

	otherPkg := types.NewPackage("example.com/other", "other")
	obj := types.NewTypeName(0, otherPkg, "Thing", nil)
	named := types.NewNamed(obj, types.NewStruct(nil, nil), nil)
	w = &codeWriter{buf: &bytes.Buffer{}}
	emitResetForExpr(w, im, "local", "x", named)
	if !strings.Contains(w.buf.String(), "x = other.Thing{}") {
		t.Fatalf("struct output: %q", w.buf.String())
	}
}

func TestEmitResetThroughPointer_PointerCases(t *testing.T) {
	im := newImportManager("local")
	ptr := types.NewPointer(types.NewPointer(types.Typ[types.Int]))
	w := &codeWriter{buf: &bytes.Buffer{}}
	emitResetForExpr(w, im, "local", "x", ptr)
	out := w.buf.String()
	if !strings.Contains(out, "if x != nil") || !strings.Contains(out, "if *x != nil") || !strings.Contains(out, "**x = 0") {
		t.Fatalf("pointer output: %q", out)
	}
}

func TestEmitResetThroughPointer_MapSlice(t *testing.T) {
	im := newImportManager("local")
	w := &codeWriter{buf: &bytes.Buffer{}}
	emitResetForExpr(w, im, "local", "x", types.NewPointer(types.NewMap(types.Typ[types.String], types.Typ[types.Int])))
	if !strings.Contains(w.buf.String(), "clear(*x)") {
		t.Fatalf("map pointer output: %q", w.buf.String())
	}

	w = &codeWriter{buf: &bytes.Buffer{}}
	emitResetForExpr(w, im, "local", "x", types.NewPointer(types.NewSlice(types.Typ[types.Int])))
	if !strings.Contains(w.buf.String(), "*x = (*x)[:0]") {
		t.Fatalf("slice pointer output: %q", w.buf.String())
	}
}

func TestAccessibleStruct_NonLocal(t *testing.T) {
	pkg := types.NewPackage("example.com/other", "other")
	obj := types.NewTypeName(0, pkg, "Thing", nil)
	named := types.NewNamed(obj, types.NewStruct(nil, nil), nil)
	if st, ok := accessibleStruct(named, "local"); ok || st != nil {
		t.Fatal("expected non-local struct to be inaccessible")
	}
}

func TestImportManagerQualifier_DuplicateAlias(t *testing.T) {
	im := newImportManager("local")
	pkg1 := types.NewPackage("example.com/a", "dup")
	pkg2 := types.NewPackage("example.com/b", "dup")
	alias1 := im.qualifier(pkg1)
	alias2 := im.qualifier(pkg2)
	if alias1 != "dup" {
		t.Fatalf("alias1=%q", alias1)
	}
	if alias2 != "dup2" {
		t.Fatalf("alias2=%q", alias2)
	}
}

func TestEmitResetMethod_NotStruct(t *testing.T) {
	pkg := &packages.Package{PkgPath: "local"}
	im := newImportManager("local")
	w := &codeWriter{buf: &bytes.Buffer{}}
	target := targetStruct{Name: "Bad", Named: types.NewNamed(types.NewTypeName(0, nil, "Bad", nil), types.Typ[types.Int], nil)}
	if err := emitResetMethod(w, im, pkg, target); err == nil {
		t.Fatal("expected error for non-struct type")
	}
}
