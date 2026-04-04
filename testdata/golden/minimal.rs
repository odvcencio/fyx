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
#[type_uuid(id = "64aee8c6-cbd0-26d9-ea92-21661f763235")]
#[visit(optional)]
pub struct Player {
    pub speed: f32,
    pub health: f32,
}

impl Default for Player {
    fn default() -> Self {
        let value = Self {
            speed: 10.0,
            health: 100.0,
        };
        value
    }
}

impl ScriptTrait for Player {
    #[allow(unused_variables)]
    fn on_update(&mut self, ctx: &mut ScriptContext) -> GameResult {
        {
            self.speed += 1.0;
        };
        Ok(())
    }
}


pub fn register_scripts(ctx: &mut PluginRegistrationContext) {
    ctx.serialization_context.script_constructors.add::<Player>("Player");
}
