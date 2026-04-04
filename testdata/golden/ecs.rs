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


#[derive(Clone)]
pub struct Velocity {
    pub linear: Vector3,
    pub angular: Vector3,
}


#[derive(Clone)]
pub struct Projectile {
    pub damage: f32,
    pub lifetime: f32,
}


pub fn system_move_things(world: &mut EcsWorld, ctx: &PluginContext) {
    let dt: f32 = ctx.dt;
    for (_entity, (pos, vel)) in world.query_mut::<(&mut Transform, &Velocity)>() {
        pos.translate(vel.linear * dt);
    }
}


pub fn system_expire(world: &mut EcsWorld, ctx: &PluginContext) {
    let dt: f32 = ctx.dt;
    for (entity, (proj,)) in world.query_mut::<(&mut Projectile,)>() {
        proj.lifetime -= dt;
                if proj.lifetime <= 0.0 {
                    world.despawn(entity);
                }
    }
}


pub fn run_ecs_systems(world: &mut EcsWorld, ctx: &PluginContext) {
    system_move_things(world, ctx);
    system_expire(world, ctx);
}
