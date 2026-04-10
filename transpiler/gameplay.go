package transpiler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/odvcencio/fyx/ast"
)

const sceneLifetimeFieldName = "_fyx_scene_lifetimes"
const sceneLifetimeFieldType = "Vec<FyxSceneLifetime>"

func augmentWithGameplayRuntimeFields(s ast.Script) ast.Script {
	extras := ReactiveFieldDecls(s.Fields)
	needsSceneLifetime := scriptUsesSceneLifetimeSupport(s)
	needsStateMachine := hasStateMachine(s.States)
	if len(extras) == 0 && !needsSceneLifetime && !needsStateMachine {
		return s
	}

	result := s
	result.Fields = make([]ast.Field, len(s.Fields))
	copy(result.Fields, s.Fields)

	for _, ex := range extras {
		result.Fields = append(result.Fields, ast.Field{
			Modifier: ast.FieldBare,
			Name:     ex.Name,
			TypeExpr: ex.TypeExpr,
		})
	}

	if needsSceneLifetime {
		result.Fields = append(result.Fields, ast.Field{
			Modifier: ast.FieldBare,
			Name:     sceneLifetimeFieldName,
			TypeExpr: sceneLifetimeFieldType,
		})
	}

	if needsStateMachine {
		result.Fields = append(result.Fields,
			ast.Field{
				Modifier: ast.FieldBare,
				Name:     stateFieldName,
				TypeExpr: stateEnumName(s.Name),
				Default:  initialStateExpr(s.Name, s.States),
			},
			ast.Field{
				Modifier: ast.FieldBare,
				Name:     stateTransitionFieldName,
				TypeExpr: "Option<" + stateEnumName(s.Name) + ">",
				Default:  "None",
			},
		)
	}

	return result
}

func timerFields(fields []ast.Field) []ast.Field {
	var timers []ast.Field
	for _, f := range fields {
		if f.Modifier == ast.FieldTimer {
			timers = append(timers, f)
		}
	}
	return timers
}

func timerFieldByName(fields []ast.Field) map[string]ast.Field {
	lookup := make(map[string]ast.Field)
	for _, f := range fields {
		if f.Modifier == ast.FieldTimer {
			lookup[f.Name] = f
		}
	}
	return lookup
}

func GenerateTimerUpdateCode(fields []ast.Field) string {
	timers := timerFields(fields)
	if len(timers) == 0 {
		return ""
	}

	e := NewEmitter()
	for i, f := range timers {
		if i > 0 {
			e.Blank()
		}
		e.Linef("if self.%s > 0.0 {", f.Name)
		e.Indent()
		e.Linef("self.%s = (self.%s - ctx.dt).max(0.0);", f.Name, f.Name)
		e.Dedent()
		e.Line("}")
	}

	return strings.TrimRight(e.String(), "\n")
}

func RewriteTimerSugar(body string, fields []ast.Field) string {
	timers := timerFieldByName(fields)
	if len(timers) == 0 {
		return body
	}

	for name, f := range timers {
		readyExpr := "(self." + name + " <= 0.0)"
		resetExpr := "self." + name + " = " + timerResetExpr(f) + ";"

		body = strings.ReplaceAll(body, "self."+name+".ready", readyExpr)
		body = strings.ReplaceAll(body, "self."+name+".reset()", resetExpr[:len(resetExpr)-1])

		bareReadyRe := regexp.MustCompile(`(^|[^[:alnum:]_\.])` + regexp.QuoteMeta(name) + `\.ready\b`)
		body = bareReadyRe.ReplaceAllString(body, `${1}`+readyExpr)

		bareResetRe := regexp.MustCompile(`(^|[^[:alnum:]_\.])` + regexp.QuoteMeta(name) + `\.reset\(\)`)
		body = bareResetRe.ReplaceAllString(body, `${1}`+resetExpr[:len(resetExpr)-1])
	}

	return body
}

func timerResetExpr(f ast.Field) string {
	if strings.TrimSpace(f.Default) == "" {
		return "0.0"
	}
	return strings.TrimSpace(f.Default)
}

func hasSceneLifetimeField(fields []ast.Field) bool {
	for _, f := range fields {
		if f.Name == sceneLifetimeFieldName && f.TypeExpr == sceneLifetimeFieldType {
			return true
		}
	}
	return false
}

func GenerateSceneLifetimeUpdateCode(fields []ast.Field) string {
	if !hasSceneLifetimeField(fields) {
		return ""
	}

	e := NewEmitter()
	e.Linef("if !self.%s.is_empty() {", sceneLifetimeFieldName)
	e.Indent()
	e.Line("let mut _fyx_expired_nodes = Vec::new();")
	e.Linef("for tracker in &mut self.%s {", sceneLifetimeFieldName)
	e.Indent()
	e.Line("tracker.remaining -= ctx.dt;")
	e.Line("if tracker.remaining <= 0.0 {")
	e.Indent()
	e.Line("_fyx_expired_nodes.push(tracker.handle);")
	e.Dedent()
	e.Line("}")
	e.Dedent()
	e.Line("}")
	e.Blank()
	e.Linef("self.%s.retain(|tracker| tracker.remaining > 0.0);", sceneLifetimeFieldName)
	e.Line("for handle in _fyx_expired_nodes {")
	e.Indent()
	e.Line("if handle.is_some() {")
	e.Indent()
	e.Line("ctx.scene.graph.remove_node(handle);")
	e.Dedent()
	e.Line("}")
	e.Dedent()
	e.Line("}")
	e.Dedent()
	e.Line("}")
	return strings.TrimRight(e.String(), "\n")
}

func GenerateSceneLifetimeDeinitCode(fields []ast.Field) string {
	if !hasSceneLifetimeField(fields) {
		return ""
	}

	e := NewEmitter()
	e.Linef("for tracker in self.%s.drain(..) {", sceneLifetimeFieldName)
	e.Indent()
	e.Line("if tracker.handle.is_some() {")
	e.Indent()
	e.Line("ctx.scene.graph.remove_node(tracker.handle);")
	e.Dedent()
	e.Line("}")
	e.Dedent()
	e.Line("}")
	return strings.TrimRight(e.String(), "\n")
}

func scriptUsesSceneLifetimeSupport(s ast.Script) bool {
	for _, h := range s.Handlers {
		if bodyHasSceneSpawnLifetime(h.Body) {
			return true
		}
	}
	for _, state := range s.States {
		for _, handler := range state.Handlers {
			if bodyHasSceneSpawnLifetime(handler.Body) {
				return true
			}
		}
	}
	for _, c := range s.Connects {
		if bodyHasSceneSpawnLifetime(c.Body) {
			return true
		}
	}
	for _, w := range s.Watches {
		if bodyHasSceneSpawnLifetime(w.Body) {
			return true
		}
	}
	return false
}

func fileNeedsEntityLifetimeSupport(file ast.File) bool {
	for _, script := range file.Scripts {
		for _, h := range script.Handlers {
			if bodyHasEcsSpawnLifetime(h.Body) {
				return true
			}
		}
		for _, state := range script.States {
			for _, handler := range state.Handlers {
				if bodyHasEcsSpawnLifetime(handler.Body) {
					return true
				}
			}
		}
		for _, c := range script.Connects {
			if bodyHasEcsSpawnLifetime(c.Body) {
				return true
			}
		}
		for _, w := range script.Watches {
			if bodyHasEcsSpawnLifetime(w.Body) {
				return true
			}
		}
	}
	for _, system := range file.Systems {
		if bodyHasEcsSpawnLifetime(system.Body) {
			return true
		}
		for _, q := range system.Queries {
			if bodyHasEcsSpawnLifetime(q.Body) {
				return true
			}
		}
	}
	return false
}

func generatedGameplayHelperLines(includeSceneLifetime, includeEntityLifetime bool) []string {
	var lines []string
	if includeSceneLifetime {
		lines = append(lines,
			"#[derive(Clone, Debug, Default)]",
			"struct FyxSceneLifetime {",
			"    handle: Handle<Node>,",
			"    remaining: f32,",
			"}",
		)
	}
	if includeEntityLifetime {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines,
			"#[derive(Clone, Debug, Default)]",
			"pub struct FyxEntityLifetime {",
			"    pub remaining: f32,",
			"}",
			"",
			"pub fn fyx_run_builtin_systems(world: &mut EcsWorld, ctx: &PluginContext) {",
			"    for (entity, (lifetime,)) in world.query_mut::<(&mut FyxEntityLifetime,)>() {",
			"        lifetime.remaining -= ctx.dt;",
			"        if lifetime.remaining <= 0.0 {",
			"            world.despawn(entity);",
			"        }",
			"    }",
			"}",
		)
	}
	return lines
}

func prependHandlerCode(existing, prefix string) string {
	return mergeBodyCode(prefix, existing)
}

func appendHandlerCode(existing, suffix string) string {
	return mergeBodyCode(existing, suffix)
}

func emitRuntimeFieldDefault(e *RustEmitter, f ast.Field) bool {
	switch f.Modifier {
	case ast.FieldTimer:
		e.LineWithSource(fmt.Sprintf("%s: Default::default(),", f.Name), f.Line)
		return true
	}
	return false
}
