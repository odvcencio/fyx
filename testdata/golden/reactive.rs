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


#[derive(Visit, Reflect, Debug, Clone, TypeUuidProvider, ComponentProvider)]
#[type_uuid(id = "c40c747e-e79e-90e1-6e46-d58fa8ba3a28")]
#[visit(optional)]
pub struct HUD {
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
        let mut value = Self {
            health: 100.0,
            is_critical: Default::default(),
            _health_prev: Default::default(),
            _is_critical_prev: Default::default(),
        };
        value._health_prev = value.health.clone();
        value.is_critical = value.health < 20.0;
        value._is_critical_prev = value.is_critical.clone();
        value
    }
}

impl ScriptTrait for HUD {
    #[allow(unused_variables)]
    fn on_update(&mut self, ctx: &mut ScriptContext) -> GameResult {
        {
            self.is_critical = self.health < 20.0;
            
            if self.is_critical != self._is_critical_prev {
                println!("critical!");
                self._is_critical_prev = self.is_critical.clone();
            }
            
            self._health_prev = self.health.clone();
        };
        Ok(())
    }
}


pub fn register_scripts(ctx: &mut PluginRegistrationContext) {
    ctx.serialization_context.script_constructors.add::<HUD>("HUD");
}
