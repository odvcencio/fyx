#[allow(unused_imports)]
use fyrox::asset::Resource;
#[allow(unused_imports)]
use fyrox::core::pool::Handle;
#[allow(unused_imports)]
use fyrox::core::reflect::prelude::*;
#[allow(unused_imports)]
use fyrox::core::type_traits::prelude::*;
#[allow(unused_imports)]
use fyrox::core::visitor::prelude::*;
#[allow(unused_imports)]
use fyrox::event::{Event, WindowEvent};
#[allow(unused_imports)]
use fyrox::plugin::error::GameResult;
#[allow(unused_imports)]
use fyrox::plugin::{PluginContext, PluginRegistrationContext};
#[allow(unused_imports)]
use fyrox::resource::model::Model;
#[allow(unused_imports)]
use fyrox::scene::node::Node;
#[allow(unused_imports)]
use fyrox::script::{ScriptContext, ScriptDeinitContext, ScriptMessageContext, ScriptMessagePayload, ScriptTrait};
#[allow(unused_imports)]
use fyrox::graph::SceneGraph;
#[allow(unused_imports)]
use fyrox::scene::graph::Graph;

fn fyx_find_node_path(graph: &Graph, path: &str) -> Handle<Node> {
    let parts = path
        .split('/')
        .filter(|segment| !segment.is_empty())
        .collect::<Vec<_>>();
    if parts.is_empty() {
        panic!("Fyx node path is empty");
    };
    if parts.len() == 1 {
        return graph
            .find_by_name_from_root(parts[0])
            .map(|(handle, _)| handle)
            .unwrap_or_else(|| panic!("Fyx node path not found: {}", path));
    }
    let Some((mut current, _)) = graph.find_by_name_from_root(parts[0]) else {
        panic!("Fyx node path not found: {}", path);
    };
    for segment in parts.iter().skip(1) {
        let current_node = graph
            .try_get_node(current)
            .unwrap_or_else(|_| panic!("Fyx node path not found: {}", path));
        let Some(next) = current_node.children().iter().copied().find(|child| {
            graph
                .try_get_node(*child)
                .map(|node| node.name() == *segment)
                .unwrap_or(false)
        }) else {
            panic!("Fyx node path not found: {}", path);
        };
        current = next;
    }
    current
}

fn fyx_expect_node_type<T>(graph: &Graph, handle: Handle<Node>, path: &str, expected_type: &str) -> Handle<Node> {
    if graph.try_get_of_type::<T>(handle).is_err() {
        panic!("Fyx node path '{}' did not resolve to expected type {}", path, expected_type);
    }
    handle
}

fn fyx_expect_nodes_type<T>(graph: &Graph, handles: Vec<Handle<Node>>, path: &str, expected_type: &str) -> Vec<Handle<Node>> {
    for handle in &handles {
        if graph.try_get_of_type::<T>(*handle).is_err() {
            panic!("Fyx node path '{}' did not resolve to expected type {}", path, expected_type);
        }
    }
    handles
}

fn fyx_find_nodes_path(graph: &Graph, pattern: &str) -> Vec<Handle<Node>> {
    if let Some(parent_path) = pattern.strip_suffix("/*") {
        if parent_path.is_empty() {
            return graph
                .try_get_node(graph.root())
                .map(|node| node.children().to_vec())
                .unwrap_or_else(|_| panic!("Fyx node path not found: {}", pattern));
        }
        let parent = fyx_find_node_path(graph, parent_path);
        return graph
            .try_get_node(parent)
            .map(|node| node.children().to_vec())
            .unwrap_or_else(|_| panic!("Fyx node path not found: {}", pattern));
    }
    vec![fyx_find_node_path(graph, pattern)]
}


use super::support::helpers::*;


use fyrox::prelude::*;


fn target_visible(scene: &Scene, origin: Vector3, direction: Vector3, range: f32) -> bool {
    scene.physics.raycast(origin, direction, range).next().is_some()
}


#[derive(Debug, Clone)]
pub struct TurretControllerFiredMsg {
    pub origin: Vector3,
    pub direction: Vector3,
}

#[derive(Debug, Clone)]
pub struct TurretControllerHeatChangedMsg {
    pub value: f32,
}


#[derive(Clone)]
pub struct HeatTrail {
    pub heat: f32,
    pub ttl: f32,
}


#[derive(Clone)]
pub struct ShotOwner {
    pub node: Handle<Node>,
}


#[derive(Visit, Reflect, Debug, Clone, TypeUuidProvider, ComponentProvider)]
#[type_uuid(id = "63be1f31-6d6f-f7b3-a520-630c121e9f3f")]
#[visit(optional)]
pub struct TurretController {
    pub range: f32,
    pub cooldown_time: f32,
    pub turn_rate: f32,
    pub pivot: Handle<Node>,
    pub muzzle: Handle<Node>,
    pub heat: f32,
    #[reflect(hidden)]
    #[visit(skip)]
    overheated: bool,
    #[reflect(hidden)]
    #[visit(skip)]
    plan: ShotPlan,
    #[reflect(hidden)]
    #[visit(skip)]
    shots_fired: i32,
    #[reflect(hidden)]
    #[visit(skip)]
    cooldown: f32,
    #[reflect(hidden)]
    #[visit(skip)]
    _heat_prev: f32,
}

impl Default for TurretController {
    fn default() -> Self {
        let mut value = Self {
            range: 18.0,
            cooldown_time: 0.2,
            turn_rate: 1.5,
            pivot: Default::default(),
            muzzle: Default::default(),
            heat: 0.0,
            overheated: Default::default(),
            plan: Default::default(),
            shots_fired: Default::default(),
            cooldown: Default::default(),
            _heat_prev: Default::default(),
        };
        value.overheated = value.heat >= 1.0;
        value._heat_prev = value.heat.clone();
        value
    }
}

impl ScriptTrait for TurretController {
    #[allow(unused_variables)]
    fn on_init(&mut self, ctx: &mut ScriptContext) -> GameResult {
        {
            self.plan = ShotPlan::for_heat(self.heat);
        };
        Ok(())
    }

    #[allow(unused_variables)]
    fn on_start(&mut self, ctx: &mut ScriptContext) -> GameResult {
        self.pivot = fyx_find_node_path(&ctx.scene.graph, "Turret/Pivot");
        self.muzzle = fyx_find_node_path(&ctx.scene.graph, "Turret/Muzzle");
        {
            self.cooldown = 0.0;
        };
        Ok(())
    }

    #[allow(unused_variables)]
    fn on_update(&mut self, ctx: &mut ScriptContext) -> GameResult {
        {
            let dt = ctx.dt;
            self.cooldown -= dt;
                    self.heat = cool_heat(self.heat, dt);
                    self.plan = ShotPlan::for_heat(self.heat);
            
                    let origin = ctx.scene.graph[self.muzzle].global_position();
                    let direction = aim_direction(&ctx.scene.graph[self.muzzle]);
            
                    if self.cooldown <= 0.0 && !self.overheated && target_visible(&ctx.scene, origin, direction, self.range) {
                        ctx.message_sender.send_global(TurretControllerFiredMsg { origin: origin, direction: direction });
                        self.cooldown = self.cooldown_time;
                        self.heat += 0.35;
                        ctx.message_sender.send_global(TurretControllerHeatChangedMsg { value: self.heat });
                        self.shots_fired += 1;
                        let _heat_marker = ctx.ecs.spawn((HeatTrail { heat: self.heat, ttl: 0.5 }, ShotOwner { node: self.muzzle }));
                    }
            
            let _fyx_heat_changed = self.heat != self._heat_prev;
            
            let _fyx_overheated_changed = if _fyx_heat_changed {
                let _fyx_overheated_prev = self.overheated.clone();
                self.overheated = self.heat >= 1.0;
                self.overheated != _fyx_overheated_prev
            } else {
                false
            };
            
            if _fyx_heat_changed {
                self._heat_prev = self.heat.clone();
            }
        };
        Ok(())
    }

    #[allow(unused_variables)]
    fn on_os_event(&mut self, event: &Event<()>, ctx: &mut ScriptContext) -> GameResult {
        if let Event::WindowEvent { event: WindowEvent::MouseButton(button), .. } = event {
            let _ = button;
                    ctx.scene.graph[self.pivot].set_rotation_y(self.turn_rate * 0.25);
        };
        Ok(())
    }

    #[allow(unused_variables)]
    fn on_deinit(&mut self, ctx: &mut ScriptDeinitContext) -> GameResult {
        {
            let _ = ctx.elapsed_time;
                    self.cooldown = 0.0;
        };
        Ok(())
    }
}


#[derive(Visit, Reflect, Debug, Clone, TypeUuidProvider, ComponentProvider)]
#[type_uuid(id = "1c1de7be-472e-2fc7-3ea9-26c1eefee752")]
#[visit(optional)]
pub struct TurretHud {
    pub heat_bar: Handle<Node>,
    pub status_label: Handle<Node>,
    pub visible_heat: f32,
    #[reflect(hidden)]
    #[visit(skip)]
    overheated: bool,
    #[reflect(hidden)]
    #[visit(skip)]
    _visible_heat_prev: f32,
}

impl Default for TurretHud {
    fn default() -> Self {
        let mut value = Self {
            heat_bar: Default::default(),
            status_label: Default::default(),
            visible_heat: 0.0,
            overheated: Default::default(),
            _visible_heat_prev: Default::default(),
        };
        value.overheated = value.visible_heat >= 1.0;
        value._visible_heat_prev = value.visible_heat.clone();
        value
    }
}

impl ScriptTrait for TurretHud {
    #[allow(unused_variables)]
    fn on_message(&mut self, message: &mut dyn ScriptMessagePayload, ctx: &mut ScriptMessageContext) -> GameResult {
        {
            let _ = (&mut *message, ctx.dt);
            
            if let Some(msg) = message.downcast_ref::<TurretControllerFiredMsg>() {
                let origin = &msg.origin;
                let direction = &msg.direction;
                let _ = (origin, direction);
                        ctx.scene.graph[self.heat_bar].set_value(self.visible_heat);
            }
            
            if let Some(msg) = message.downcast_ref::<TurretControllerHeatChangedMsg>() {
                let value = &msg.value;
                self.visible_heat = *value;
                        ctx.scene.graph[self.heat_bar].set_value(*value);
            }
        };
        Ok(())
    }

    #[allow(unused_variables)]
    fn on_update(&mut self, ctx: &mut ScriptContext) -> GameResult {
        {
            let _fyx_visible_heat_changed = self.visible_heat != self._visible_heat_prev;
            
            let _fyx_overheated_changed = if _fyx_visible_heat_changed {
                let _fyx_overheated_prev = self.overheated.clone();
                self.overheated = self.visible_heat >= 1.0;
                self.overheated != _fyx_overheated_prev
            } else {
                false
            };
            
            if _fyx_overheated_changed {
                if self.overheated {
                            ctx.scene.graph[self.status_label].set_text("OVERHEATED");
                            ctx.scene.graph[self.status_label].set_color(Color::RED);
                        } else {
                            ctx.scene.graph[self.status_label].set_text("READY");
                        }
            }
            
            if _fyx_visible_heat_changed {
                self._visible_heat_prev = self.visible_heat.clone();
            }
        };
        Ok(())
    }

    #[allow(unused_variables)]
    fn on_start(&mut self, ctx: &mut ScriptContext) -> GameResult {
        self.heat_bar = fyx_expect_node_type::<ProgressBar>(&ctx.scene.graph, fyx_find_node_path(&ctx.scene.graph, "UI/HeatBar"), "UI/HeatBar", "ProgressBar");
        self.status_label = fyx_expect_node_type::<Text>(&ctx.scene.graph, fyx_find_node_path(&ctx.scene.graph, "UI/Status"), "UI/Status", "Text");
        {
            ctx.message_dispatcher.subscribe_to::<TurretControllerFiredMsg>(ctx.handle);
            ctx.message_dispatcher.subscribe_to::<TurretControllerHeatChangedMsg>(ctx.handle);
        };
        Ok(())
    }
}


pub fn system_decay_heat_trails(world: &mut EcsWorld, ctx: &PluginContext) {
    let dt: f32 = ctx.dt;
    for (entity, (trail,)) in world.query_mut::<(&mut HeatTrail,)>() {
        trail.ttl -= dt;
                if trail.ttl <= 0.0 {
                    world.despawn(entity);
                }
    }
}


pub fn system_inspect_heat_trails(world: &mut EcsWorld, ctx: &PluginContext) {
    for (_entity, (trail, owner)) in world.query_mut::<(&HeatTrail, &ShotOwner)>() {
        let trail_origin = ctx.scene.graph[owner.node].global_position();
                let _ = (trail.heat, trail_origin);
    }
}


pub fn run_ecs_systems(world: &mut EcsWorld, ctx: &PluginContext) {
    system_decay_heat_trails(world, ctx);
    system_inspect_heat_trails(world, ctx);
}


pub fn register_scripts(ctx: &mut PluginRegistrationContext) {
    ctx.serialization_context.script_constructors.add::<TurretController>("TurretController");
    ctx.serialization_context.script_constructors.add::<TurretHud>("TurretHud");
}
