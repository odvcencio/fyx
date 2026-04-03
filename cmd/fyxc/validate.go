package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type diagnostic struct {
	SourcePath string
	SourceLine int
	Column     int
	Message    string
}

type cargoSpan struct {
	FileName    string `json:"file_name"`
	LineStart   int    `json:"line_start"`
	ColumnStart int    `json:"column_start"`
	IsPrimary   bool   `json:"is_primary"`
}

func validateGeneratedFiles(files []compiledFile) ([]diagnostic, error) {
	tmp, err := os.MkdirTemp("", "fyxc-check-*")
	if err != nil {
		return nil, err
	}
	if os.Getenv("FYXC_KEEP_TMP") == "" {
		defer os.RemoveAll(tmp)
	}

	validationFiles := prepareValidationFiles(files)
	if err := writeValidationWorkspace(tmp, validationFiles); err != nil {
		return nil, err
	}

	diagnostics, err := runCargoCheck(filepath.Join(tmp, "fyxc_check_harness"), validationFiles)
	if err != nil && os.Getenv("FYXC_KEEP_TMP") != "" {
		return nil, fmt.Errorf("%w (workspace kept at %s)", err, tmp)
	}
	return diagnostics, err
}

func writeValidationWorkspace(root string, files []compiledFile) error {
	macrosDir := filepath.Join(root, "fyxc_check_macros")
	harnessDir := filepath.Join(root, "fyxc_check_harness")

	if err := os.MkdirAll(filepath.Join(macrosDir, "src"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(harnessDir, "src"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(macrosDir, "Cargo.toml"), []byte(validationMacrosCargoToml), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(macrosDir, "src", "lib.rs"), []byte(validationMacrosLibRS), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(harnessDir, "Cargo.toml"), []byte(validationHarnessCargoToml), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(harnessDir, "src", "lib.rs"), []byte(validationHarnessLibRS), 0o644); err != nil {
		return err
	}
	return writeOutputTree(filepath.Join(harnessDir, "src", "generated"), files, false)
}

func prepareValidationFiles(files []compiledFile) []compiledFile {
	prepared := make([]compiledFile, len(files))
	copy(prepared, files)
	for i := range prepared {
		prepared[i].Output.Code = "use crate::*;\n\n" + prepared[i].Output.Code
		prepared[i].Output.LineMap = append([]int{0, 0}, prepared[i].Output.LineMap...)
	}
	return prepared
}

func runCargoCheck(harnessDir string, files []compiledFile) ([]diagnostic, error) {
	sourceMaps := make(map[string]compiledFile, len(files))
	for _, file := range files {
		path := filepath.Join(harnessDir, "src", "generated", strings.TrimSuffix(file.SourcePath, ".fyx")+".rs")
		sourceMaps[filepath.Clean(path)] = file
	}

	cmd := exec.Command("cargo", "check", "--message-format=json")
	cmd.Dir = harnessDir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	type cargoInnerMessage struct {
		Message  string      `json:"message"`
		Level    string      `json:"level"`
		Rendered string      `json:"rendered"`
		Spans    []cargoSpan `json:"spans"`
	}
	type cargoMessage struct {
		Reason  string            `json:"reason"`
		Message cargoInnerMessage `json:"message"`
	}

	var diagnostics []diagnostic
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		var msg cargoMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}
		if msg.Reason != "compiler-message" || msg.Message.Level != "error" {
			continue
		}
		span, ok := primaryGeneratedSpan(harnessDir, msg.Message.Spans, sourceMaps)
		if !ok {
			continue
		}
		file := sourceMaps[normalizeCargoPath(harnessDir, span.FileName)]
		sourceLine := span.LineStart
		if span.LineStart > 0 && span.LineStart <= len(file.Output.LineMap) {
			for i := span.LineStart - 1; i >= 0; i-- {
				if file.Output.LineMap[i] != 0 {
					sourceLine = file.Output.LineMap[i]
					break
				}
			}
		}
		diagnostics = append(diagnostics, diagnostic{
			SourcePath: file.SourcePath,
			SourceLine: sourceLine,
			Column:     span.ColumnStart,
			Message:    msg.Message.Message,
		})
	}

	errOut, _ := ioReadAll(stderr)
	waitErr := cmd.Wait()
	if scanErr := scanner.Err(); scanErr != nil {
		return nil, scanErr
	}
	if waitErr != nil && len(diagnostics) == 0 {
		return nil, fmt.Errorf("%w\n%s", waitErr, strings.TrimSpace(string(errOut)))
	}
	return diagnostics, nil
}

func primaryGeneratedSpan(harnessDir string, spans []cargoSpan, files map[string]compiledFile) (cargoSpan, bool) {
	for _, span := range spans {
		if !span.IsPrimary {
			continue
		}
		if _, ok := files[normalizeCargoPath(harnessDir, span.FileName)]; ok {
			return span, true
		}
	}
	return cargoSpan{}, false
}

func normalizeCargoPath(harnessDir, name string) string {
	if filepath.IsAbs(name) {
		return filepath.Clean(name)
	}
	return filepath.Clean(filepath.Join(harnessDir, name))
}

func ioReadAll(r io.Reader) ([]byte, error) { return io.ReadAll(r) }

const validationMacrosCargoToml = `[package]
name = "fyxc_check_macros"
version = "0.1.0"
edition = "2024"

[lib]
proc-macro = true
`

const validationMacrosLibRS = `use proc_macro::TokenStream;

#[proc_macro_derive(Visit, attributes(visit))]
pub fn derive_visit(_input: TokenStream) -> TokenStream { TokenStream::new() }

#[proc_macro_derive(Reflect, attributes(reflect))]
pub fn derive_reflect(_input: TokenStream) -> TokenStream { TokenStream::new() }

#[proc_macro_derive(TypeUuidProvider, attributes(type_uuid))]
pub fn derive_type_uuid_provider(_input: TokenStream) -> TokenStream { TokenStream::new() }

#[proc_macro_derive(ComponentProvider)]
pub fn derive_component_provider(_input: TokenStream) -> TokenStream { TokenStream::new() }

#[proc_macro_attribute]
pub fn type_uuid(_attr: TokenStream, item: TokenStream) -> TokenStream { item }

#[proc_macro_attribute]
pub fn visit(_attr: TokenStream, item: TokenStream) -> TokenStream { item }

#[proc_macro_attribute]
pub fn reflect(_attr: TokenStream, item: TokenStream) -> TokenStream { item }
`

const validationHarnessCargoToml = `[package]
name = "fyxc_check_harness"
version = "0.1.0"
edition = "2024"

[dependencies]
fyxc_check_macros = { path = "../fyxc_check_macros" }
`

const validationHarnessLibRS = `#![allow(dead_code, unused_variables, unused_imports)]

use std::any::Any;
use std::marker::PhantomData;
use std::ops::{Index, IndexMut};

pub use fyxc_check_macros::{reflect, type_uuid, visit, ComponentProvider, Reflect, TypeUuidProvider, Visit};

pub mod fyrox {
    pub mod prelude {
        pub use crate::*;
    }
}

#[derive(Default, Clone, Copy, Debug)]
pub struct Handle<T>(PhantomData<T>);

#[derive(Clone, Copy, Debug)]
pub struct Resource<T>(PhantomData<T>);
impl<T> Default for Resource<T> {
    fn default() -> Self { Self(PhantomData) }
}

#[derive(Default, Clone, Copy, Debug)]
pub struct Node;
pub type Camera3D = Node;
pub type Light = Node;
pub type Text = Node;
pub type Sprite = Node;
pub type ProgressBar = Node;
pub type SoundBuffer = Node;
pub type Model = Node;

#[derive(Default, Clone, Copy, Debug)]
pub struct Vector3;
impl Vector3 {
    pub const ZERO: Self = Self;
    pub fn new(_x: f32, _y: f32, _z: f32) -> Self { Self }
    pub fn normalized(self) -> Self { self }
}
impl std::ops::Mul<f32> for Vector3 {
    type Output = Vector3;
    fn mul(self, _rhs: f32) -> Self::Output { self }
}

#[derive(Default, Clone, Debug)]
pub struct Transform;
impl Transform {
    pub fn from<T>(_value: T) -> Self { Self }
    pub fn translate(&mut self, _value: Vector3) {}
    pub fn position(&self) -> Vector3 { Vector3 }
}

#[derive(Default, Clone, Debug)]
pub struct Color;
impl Color {
    pub const RED: Self = Self;
}

#[derive(Default, Clone, Debug)]
pub struct KeyboardInput;

#[derive(Default, Clone, Debug)]
pub enum MouseButton {
    #[default]
    Left,
}

#[derive(Clone, Debug)]
pub enum WindowEvent {
    KeyboardInput(KeyboardInput),
    MouseButton(MouseButton),
    Other,
}
impl Default for WindowEvent {
    fn default() -> Self { Self::Other }
}

#[derive(Clone, Debug)]
pub enum Event<T> {
    Tick(PhantomData<T>),
    WindowEvent { event: WindowEvent, marker: PhantomData<T> },
}
impl<T> Default for Event<T> {
    fn default() -> Self { Self::Tick(PhantomData) }
}

#[derive(Default, Clone, Debug)]
pub struct NodeRef;
impl NodeRef {
    pub fn global_position(&self) -> Vector3 { Vector3 }
    pub fn look_direction(&self) -> Vector3 { Vector3 }
    pub fn parent(&self) -> Handle<Node> { Handle::default() }
    pub fn rotate_y(&mut self, _value: f32) {}
    pub fn set_visibility(&mut self, _value: bool) {}
    pub fn animate(&mut self, _value: &str) {}
    pub fn set_color(&mut self, _value: Color) {}
    pub fn set_value<T>(&mut self, _value: T) {}
    pub fn set_text<T>(&mut self, _value: T) {}
    pub fn local_transform_mut(&mut self) -> &mut Self { self }
    pub fn set_position<T>(&mut self, _value: T) {}
}

#[derive(Default, Clone, Debug)]
pub struct Graph {
    node: NodeRef,
}
impl Graph {
    pub fn find_by_name_from_root(&self, _name: &str) -> Option<(Handle<Node>, ())> {
        Some((Handle::default(), ()))
    }
}
impl Index<Handle<Node>> for Graph {
    type Output = NodeRef;
    fn index(&self, _index: Handle<Node>) -> &Self::Output { &self.node }
}
impl IndexMut<Handle<Node>> for Graph {
    fn index_mut(&mut self, _index: Handle<Node>) -> &mut Self::Output { &mut self.node }
}

#[derive(Default, Clone, Debug)]
pub struct RayHit {
    pub node: Handle<Node>,
}

#[derive(Default, Clone, Debug)]
pub struct Physics;
impl Physics {
    pub fn raycast(&self, _origin: Vector3, _dir: Vector3, _distance: f32) -> std::vec::IntoIter<RayHit> {
        Vec::new().into_iter()
    }
}

#[derive(Default, Clone, Debug)]
pub struct Scene {
    pub graph: Graph,
    pub physics: Physics,
}

#[derive(Default, Clone, Debug)]
pub struct ResourceManager;
impl ResourceManager {
    pub fn request<T>(&self, _path: impl AsRef<str>) -> Resource<T> { Resource::default() }
}
impl Resource<Model> {
    pub fn instantiate(&self, _graph: &mut Graph) -> Handle<Node> { Handle::default() }
}

#[derive(Default, Clone, Debug)]
pub struct MessageSender;
impl MessageSender {
    pub fn send_global<T>(&self, _message: T) {}
    pub fn send_to_target<T>(&self, _target: Handle<Node>, _message: T) {}
}

#[derive(Default, Clone, Debug)]
pub struct MessageDispatcher;
impl MessageDispatcher {
    pub fn subscribe_to<T>(&mut self, _target: Handle<Node>) {}
}

#[derive(Default, Clone, Debug)]
pub struct ScriptContext {
    pub scene: Scene,
    pub handle: Handle<Node>,
    pub resource_manager: ResourceManager,
    pub message_sender: MessageSender,
    pub message_dispatcher: MessageDispatcher,
    pub ecs: EcsWorld,
    pub dt: f32,
}

#[derive(Default, Clone, Debug)]
pub struct ScriptMessageContext {
    pub scene: Scene,
    pub handle: Handle<Node>,
    pub resource_manager: ResourceManager,
    pub message_dispatcher: MessageDispatcher,
    pub message_sender: MessageSender,
    pub ecs: EcsWorld,
    pub dt: f32,
}

#[derive(Default, Clone, Debug)]
pub struct ScriptDeinitContext {
    pub scene: Scene,
    pub handle: Handle<Node>,
    pub resource_manager: ResourceManager,
    pub message_sender: MessageSender,
    pub message_dispatcher: MessageDispatcher,
    pub ecs: EcsWorld,
    pub dt: f32,
}

pub trait ScriptTrait {
    fn on_init(&mut self, _ctx: &mut ScriptContext) {}
    fn on_start(&mut self, _ctx: &mut ScriptContext) {}
    fn on_update(&mut self, _ctx: &mut ScriptContext) {}
    fn on_deinit(&mut self, _ctx: &mut ScriptDeinitContext) {}
    fn on_os_event(&mut self, _event: &Event<()>, _ctx: &mut ScriptContext) {}
    fn on_message(&mut self, _message: &mut dyn ScriptMessagePayload, _ctx: &mut ScriptMessageContext) {}
}

pub trait ScriptMessagePayload: Any {}
impl<T: Any> ScriptMessagePayload for T {}
impl dyn ScriptMessagePayload {
    pub fn downcast_ref<T: Any>(&self) -> Option<&T> {
        (self as &dyn Any).downcast_ref::<T>()
    }
}

#[derive(Default, Clone, Debug)]
pub struct ScriptConstructors;
impl ScriptConstructors {
    pub fn add<T>(&mut self, _name: &str) {}
}

#[derive(Default, Clone, Debug)]
pub struct SerializationContext {
    pub script_constructors: ScriptConstructors,
}

#[derive(Default, Clone, Debug)]
pub struct PluginRegistrationContext {
    pub serialization_context: SerializationContext,
}

#[derive(Default, Clone, Debug)]
pub struct PluginContext {
    pub dt: f32,
    pub scene: Scene,
}

pub fn do_hit_check<T1, T2, T3, T4>(_a: T1, _b: T2, _c: T3, _d: T4) {}

pub type Entity = u64;

#[derive(Default, Clone, Debug)]
pub struct EcsWorld;
impl EcsWorld {
    pub fn query_mut<T>(&mut self) -> std::vec::IntoIter<(Entity, T)> {
        Vec::new().into_iter()
    }

    pub fn spawn<T>(&mut self, _bundle: T) -> Entity { 0 }

    pub fn despawn(&mut self, _entity: Entity) {}
}

pub mod generated;
`
