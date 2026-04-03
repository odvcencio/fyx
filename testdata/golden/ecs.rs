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
