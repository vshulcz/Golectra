package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"sort"
	"strings"
)

// hasMarker checks if the given comment groups contain the marker.
func hasMarker(genDoc, specDoc *ast.CommentGroup) bool {
	check := func(cg *ast.CommentGroup) bool {
		if cg == nil {
			return false
		}
		for _, c := range cg.List {
			txt := c.Text
			txt = strings.TrimSpace(txt)
			txt = strings.ToLower(txt)
			txt = strings.TrimPrefix(txt, "//")
			txt = strings.TrimPrefix(txt, "/*")
			txt = strings.TrimSuffix(txt, "*/")
			txt = strings.TrimSpace(txt)

			if txt == "+"+marker {
				return true
			}
		}
		return false
	}

	return check(specDoc) || check(genDoc)
}

// hasResetMethod checks if the given type has a Reset method.
func hasResetMethod(t types.Type) bool {
	ms := types.NewMethodSet(t)
	for i := 0; i < ms.Len(); i++ {
		sel := ms.At(i)
		if sel.Obj().Name() != "Reset" {
			continue
		}
		fn, ok := sel.Obj().(*types.Func)
		if !ok {
			continue
		}
		sig, ok := fn.Type().(*types.Signature)
		if !ok {
			continue
		}
		if sig.Params().Len() == 0 && sig.Results().Len() == 0 {
			return true
		}
	}
	return false
}

// zeroBasic returns the zero value for a basic type.
func zeroBasic(b *types.Basic) string {
	switch b.Kind() {
	case types.Bool:
		return "false"
	case types.String:
		return `""`
	case types.UntypedNil:
		const nilString = "nil"
		return nilString
	default:
		return "0"
	}
}

// zeroExpr returns the zero value for a given type.
func zeroExpr(im *importManager, t types.Type) string {
	u := t.Underlying()

	switch u := u.(type) {
	case *types.Basic:
		return zeroBasic(u)
	case *types.Slice, *types.Map, *types.Chan, *types.Signature, *types.Interface, *types.Pointer:
		return "nil"
	case *types.Struct, *types.Array:
		return im.typeString(t) + "{}"
	default:
		return "nil"
	}
}

// importSpec represents an import path and its alias.
type importSpec struct {
	Path  string
	Alias string
}

// importManager manages imports for generated code.
type importManager struct {
	localPkgPath string
	byPath       map[string]string
	usedAlias    map[string]bool
}

// newImportManager creates a new import manager.
func newImportManager(localPkgPath string) *importManager {
	return &importManager{
		localPkgPath: localPkgPath,
		byPath:       map[string]string{},
		usedAlias:    map[string]bool{},
	}
}

// qualifier returns the alias for a package.
func (im *importManager) qualifier(p *types.Package) string {
	if p == nil {
		return ""
	}
	if p.Path() == im.localPkgPath {
		return ""
	}
	if alias, ok := im.byPath[p.Path()]; ok {
		return alias
	}

	base := p.Name()
	alias := base
	if im.usedAlias[alias] {
		for i := 2; ; i++ {
			alias = fmt.Sprintf("%s%d", base, i)
			if !im.usedAlias[alias] {
				break
			}
		}
	}
	im.usedAlias[alias] = true
	im.byPath[p.Path()] = alias
	return alias
}

// typeString returns the string representation of a type.
func (im *importManager) typeString(t types.Type) string {
	return types.TypeString(t, im.qualifier)
}

// sortedImports returns the sorted list of imports.
func (im *importManager) sortedImports() []importSpec {
	paths := make([]string, 0, len(im.byPath))
	for p := range im.byPath {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	out := make([]importSpec, 0, len(paths))
	for _, p := range paths {
		alias := im.byPath[p]

		last := p
		if idx := strings.LastIndex(p, "/"); idx >= 0 {
			last = p[idx+1:]
		}
		if alias == last {
			alias = ""
		}

		out = append(out, importSpec{Path: p, Alias: alias})
	}
	return out
}
