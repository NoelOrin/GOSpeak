package genprotocol

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

type eventEntry struct {
	Name    string
	Dir     string
	Ack     bool
	Payload string
}

type structDef struct {
	Name   string
	Fields []tsField
}

type tsField struct {
	Name     string
	Type     string
	Optional bool
	Doc      string
}

// Generate 解析 pkgDir 下的 Go 源码，返回 文件名→内容。
// 不执行任何代码，仅静态解析：字符串常量、Catalog 复合字面量、struct 定义。
func Generate(pkgDir string) (map[string]string, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, pkgDir, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", pkgDir, err)
	}
	g := &gen{fset: fset, consts: map[string]string{}, structs: map[string]*structDef{}, goNameByWire: map[string]string{}}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			g.walkFile(file)
		}
	}
	for constName, value := range g.consts {
		if strings.HasPrefix(constName, "Event") {
			g.goNameByWire[value] = constName
		}
	}
	if len(g.catalog) == 0 {
		return nil, fmt.Errorf("no Catalog var found in %s", pkgDir)
	}
	if err := g.validate(); err != nil {
		return nil, err
	}
	return map[string]string{
		"events.ts":   g.emitEvents(),
		"payloads.ts": g.emitPayloads(),
		"envelope.ts": g.emitEnvelope(),
	}, nil
}

type gen struct {
	fset    *token.FileSet
	consts  map[string]string
	structs map[string]*structDef
	catalog []eventEntry
	// goNameByWire：wire 事件名 → Go 常量名（EventXxx），供 EVENTS 键名生成
	goNameByWire map[string]string
}

func (g *gen) walkFile(file *ast.File) {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			g.genSpec(spec)
		}
	}
}

func (g *gen) genSpec(spec ast.Spec) {
	switch s := spec.(type) {
	case *ast.ValueSpec:
		// 字符串常量收集（事件名 + Direction 等，统一供 Ident 解析）
		for i, name := range s.Names {
			if i >= len(s.Values) {
				continue
			}
			if lit, ok := constString(s.Values[i]); ok {
				g.consts[name.Name] = lit
			}
		}
		// Catalog 变量：var Catalog = []Event{...}
		for i, name := range s.Names {
			if name.Name != "Catalog" || i >= len(s.Values) {
				continue
			}
			cl, ok := s.Values[i].(*ast.CompositeLit)
			if !ok {
				continue
			}
			g.parseCatalog(cl)
		}
	case *ast.TypeSpec:
		st, ok := s.Type.(*ast.StructType)
		if !ok {
			return
		}
		def := &structDef{Name: s.Name.Name}
		for _, field := range st.Fields.List {
			def.Fields = append(def.Fields, g.field(field)...)
		}
		g.structs[s.Name.Name] = def
	}
}

func (g *gen) parseCatalog(cl *ast.CompositeLit) {
	for _, elt := range cl.Elts {
		e, ok := elt.(*ast.CompositeLit)
		if !ok {
			continue
		}
		entry := eventEntry{}
		for _, kv := range e.Elts {
			p, ok := kv.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			keyIdent, ok := p.Key.(*ast.Ident)
			if !ok {
				continue
			}
			switch keyIdent.Name {
			case "Name", "Dir":
				v, err := g.strOrConst(p.Value)
				if err != nil {
					g.fail(p.Pos(), err)
				}
				if keyIdent.Name == "Name" {
					entry.Name = v
				} else {
					entry.Dir = v
				}
			case "Ack":
				id, ok := p.Value.(*ast.Ident)
				if !ok {
					g.fail(p.Pos(), fmt.Errorf("Ack must be bool literal"))
				}
				entry.Ack = id.Name == "true"
			case "Payload":
				v, ok := constString(p.Value)
				if !ok {
					g.fail(p.Pos(), fmt.Errorf("Payload must be a string literal naming a struct"))
				}
				entry.Payload = v
			}
		}
		g.catalog = append(g.catalog, entry)
	}
}

// fail 以编译期错误的形式终止生成（零静默兜底：协议错误不允许被吞掉）。
func (g *gen) fail(pos token.Pos, err error) {
	panic(fmt.Sprintf("%s: %v", g.fset.Position(pos), err))
}

func constString(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return v, true
}

// strOrConst 解析字符串字面量或同包常量标识符。
func (g *gen) strOrConst(expr ast.Expr) (string, error) {
	if v, ok := constString(expr); ok {
		return v, nil
	}
	if id, ok := expr.(*ast.Ident); ok {
		if v, ok := g.consts[id.Name]; ok {
			return v, nil
		}
		return "", fmt.Errorf("unresolved identifier %s", id.Name)
	}
	return "", fmt.Errorf("unsupported expression %T", expr)
}

// field 映射一个 struct 字段；嵌入字段报错（P1 的 DTO 均无嵌入）。
func (g *gen) field(f *ast.Field) []tsField {
	if len(f.Names) == 0 {
		g.fail(f.Pos(), fmt.Errorf("embedded fields not supported in protocol payload structs"))
	}
	jsonName, omit, ok := jsonTag(f.Tag)
	if !ok || jsonName == "-" {
		return nil
	}
	typ, err := g.tsType(f.Type)
	if err != nil {
		g.fail(f.Pos(), fmt.Errorf("field %s: %w", f.Names[0].Name, err))
	}
	return []tsField{{
		Name:     jsonName,
		Type:     typ,
		Optional: omit || isPointer(f.Type),
		Doc:      strings.TrimSpace(f.Doc.Text()),
	}}
}

func jsonTag(tag ast.Expr) (name string, omit bool, ok bool) {
	lit, ok := tag.(*ast.BasicLit)
	if !ok || lit == nil {
		return "", false, false
	}
	v, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false, false
	}
	if !strings.HasPrefix(v, `json:"`) {
		return "", false, false
	}
	parts := strings.Split(strings.TrimPrefix(v, `json:"`), `"`)
	opts := strings.Split(parts[0], ",")
	name = opts[0]
	for _, o := range opts[1:] {
		if o == "omitempty" {
			omit = true
		}
	}
	return name, omit, name != ""
}

func isPointer(t ast.Expr) bool {
	_, ok := t.(*ast.StarExpr)
	return ok
}

var errUnsupported = fmt.Errorf("unsupported type")

// tsType 递归映射 Go 类型 → TS。
func (g *gen) tsType(expr ast.Expr) (string, error) {
	switch t := expr.(type) {
	case *ast.Ident:
		switch t.Name {
		case "string":
			return "string", nil
		case "bool":
			return "boolean", nil
		case "int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64",
			"float32", "float64":
			return "number", nil
		case "any":
			return "unknown", nil
		default:
			if _, ok := g.structs[t.Name]; ok {
				return t.Name, nil
			}
			return "", fmt.Errorf("%w: unknown ident %s", errUnsupported, t.Name)
		}
	case *ast.StarExpr:
		return g.tsType(t.X)
	case *ast.ArrayType:
		inner, err := g.tsType(t.Elt)
		if err != nil {
			return "", err
		}
		return inner + "[]", nil
	case *ast.MapType:
		key, err := g.tsType(t.Key)
		if err != nil {
			return "", err
		}
		if key != "string" {
			return "", fmt.Errorf("%w: map key must be string", errUnsupported)
		}
		val, err := g.tsType(t.Value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Record<string, %s>", val), nil
	case *ast.SelectorExpr:
		if id, ok := t.X.(*ast.Ident); ok && id.Name == "json" && t.Sel.Name == "RawMessage" {
			return "unknown", nil
		}
		return "", fmt.Errorf("%w: selector %s.%s", errUnsupported, t.X, t.Sel)
	case *ast.InterfaceType:
		return "unknown", nil
	default:
		return "", fmt.Errorf("%w: %T", errUnsupported, expr)
	}
}

func (g *gen) validate() error {
	names := map[string]bool{}
	for _, e := range g.catalog {
		if e.Name == "" || names[e.Name] {
			return fmt.Errorf("catalog entry empty/duplicate name: %q", e.Name)
		}
		names[e.Name] = true
		switch e.Dir {
		case "c2s", "s2c", "both", "internal":
		default:
			return fmt.Errorf("event %s: invalid dir %q", e.Name, e.Dir)
		}
		if e.Payload != "" {
			if _, ok := g.structs[e.Payload]; !ok {
				return fmt.Errorf("event %s: payload struct %q not found in package", e.Name, e.Payload)
			}
		}
	}
	return nil
}

func (g *gen) wireEvents() []eventEntry {
	var out []eventEntry
	for _, e := range g.catalog {
		if e.Dir != "internal" {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// goEventName 返回 wire 事件名对应的 Go 常量名；缺失即目录非法。
func (g *gen) goEventName(wire string) string {
	name, ok := g.goNameByWire[wire]
	if !ok {
		g.failNoPos(fmt.Errorf("event %q has no Event* constant in package", wire))
	}
	return name
}

func (g *gen) failNoPos(err error) {
	panic(err.Error())
}

// tsEventsKey 生成 EVENTS 键名：EventRoomJoinSFU → ROOM_JOIN_SFU。
func tsEventsKey(goConst string) string {
	name := strings.TrimPrefix(goConst, "Event")
	var b strings.Builder
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' && i > 0 {
			prev := name[i-1]
			if (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9') {
				b.WriteByte('_')
			} else if prev >= 'A' && prev <= 'Z' && i+1 < len(name) {
				if next := name[i+1]; next >= 'a' && next <= 'z' {
					b.WriteByte('_')
				}
			}
		}
		b.WriteByte(c)
	}
	return strings.ToUpper(b.String())
}

const generatedHeader = "// GENERATED — 手改无效，跑 pnpm gen:protocol\n"

func (g *gen) emitEvents() string {
	var b bytes.Buffer
	b.WriteString(generatedHeader)
	b.WriteString("// WS 事件协议常量：来源 app/server/internal/protocol/events.go Catalog\n")
	b.WriteString("export const EVENTS = {\n")
	for _, e := range g.wireEvents() {
		fmt.Fprintf(&b, "\t%s: %q,\n", tsEventsKey(g.goEventName(e.Name)), e.Name)
	}
	b.WriteString("} as const;\n\n")
	b.WriteString("export type EventName = keyof typeof EVENTS;\n")
	b.WriteString(`export type EventDirection = "c2s" | "s2c" | "both";

`)
	b.WriteString("export const EVENT_META = {\n")
	for _, e := range g.wireEvents() {
		fmt.Fprintf(&b, "\t%s: { dir: %q, ack: %t },\n", tsEventsKey(g.goEventName(e.Name)), e.Dir, e.Ack)
	}
	b.WriteString("} as const satisfies Record<EventName, { dir: EventDirection; ack: boolean }>;\n")
	return b.String()
}

func (g *gen) emitPayloads() string {
	var b bytes.Buffer
	b.WriteString(generatedHeader)
	b.WriteString("// WS 事件 payload 类型：来源 app/server/internal/protocol/payloads.go\n")
	names := make([]string, 0, len(g.structs))
	for n := range g.structs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		def := g.structs[n]
		if len(def.Fields) == 0 {
			continue
		}
		fmt.Fprintf(&b, "export interface %s {\n", n)
		for _, f := range def.Fields {
			if f.Doc != "" {
				fmt.Fprintf(&b, "\t/** %s */\n", f.Doc)
			}
			opt := ""
			if f.Optional {
				opt = "?"
			}
			fmt.Fprintf(&b, "\t%s%s: %s;\n", f.Name, opt, f.Type)
		}
		b.WriteString("}\n\n")
	}
	return b.String()
}

func (g *gen) emitEnvelope() string {
	var b bytes.Buffer
	b.WriteString(generatedHeader)
	b.WriteString(`// ACK 信封约定（与 ws.ACK 对应）：成功 data 携带结果、失败走 error 通道。
// emitAck 成功路径 resolve 的是 data 本身（非信封）。
import type { EventName } from "./events";

export interface AckFrame<T = unknown> {
	id?: string;
	event: string;
	data?: T;
	error?: { code: number; message: string };
}

// AckPayload<T>：emitAck 成功路径 resolve 的 data 类型。
// P1 各事件 data 尚未全部类型化（PayloadOf 于 P2 补齐），统一先 unknown。
export type AckPayload<T> = T;

export type AckResult<E extends EventName = EventName> = AckPayload<unknown>;
`)
	return b.String()
}
