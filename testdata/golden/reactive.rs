#[derive(Visit, Reflect, Default, Debug, Clone, TypeUuidProvider, ComponentProvider)]
#[type_uuid(id = "c40c747e-e79e-90e1-6e46-d58fa8ba3a28")]
#[visit(optional)]
pub struct HUD {
    #[reflect(expand)]
    pub health: f32,
    #[reflect(hidden)]
    #[visit(skip)]
    is_critical: bool,
    #[reflect(hidden)]
    #[visit(skip)]
    _health_prev: f32,
    #[reflect(hidden)]
    #[visit(skip)]
    _is_critical_prev: bool,
}

impl Default for HUD {
    fn default() -> Self {
        Self {
            health: 100.0,
            is_critical: self.health < 20.0,
            _health_prev: 100.0,
            _is_critical_prev: self.health < 20.0,
            ..Default::default()
        }
    }
}

impl ScriptTrait for HUD {
    fn on_update(&mut self, ctx: &mut ScriptContext) {
        self.is_critical = self.health < 20.0;
        
        if self.is_critical != self._is_critical_prev {
            println!("critical!");
            self._is_critical_prev = self.is_critical.clone();
        }
        
        self._health_prev = self.health.clone();
    }
}

pub fn register_scripts(ctx: &mut PluginRegistrationContext) {
    ctx.serialization_context.script_constructors.add::<HUD>("HUD");
}
