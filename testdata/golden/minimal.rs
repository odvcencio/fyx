#[derive(Visit, Reflect, Default, Debug, Clone, TypeUuidProvider, ComponentProvider)]
#[type_uuid(id = "64aee8c6-cbd0-26d9-ea92-21661f763235")]
#[visit(optional)]
pub struct Player {
    #[reflect(expand)]
    pub speed: f32,
    #[reflect(expand)]
    pub health: f32,
}

impl Default for Player {
    fn default() -> Self {
        Self {
            speed: 10.0,
            health: 100.0,
            ..Default::default()
        }
    }
}

impl ScriptTrait for Player {
    fn on_update(&mut self, ctx: &mut ScriptContext) {
        self.speed += 1.0;
    }
}

pub fn register_scripts(ctx: &mut PluginRegistrationContext) {
    ctx.serialization_context.script_constructors.add::<Player>("Player");
}
