#[derive(Debug, Clone)]
pub struct EnemyDiedMsg {
    pub position: Vector3,
}

#[derive(Debug, Clone)]
pub struct EnemyDamagedMsg {
    pub amount: f32,
}


#[derive(Visit, Reflect, Debug, Clone, TypeUuidProvider, ComponentProvider)]
#[type_uuid(id = "0021d5fe-20a0-8754-dd96-5d947e483074")]
#[visit(optional)]
pub struct Enemy {
    #[reflect(expand)]
    pub health: f32,
}

impl Default for Enemy {
    fn default() -> Self {
        let value = Self {
            health: 100.0,
        };
        value
    }
}

impl ScriptTrait for Enemy {
    fn on_update(&mut self, ctx: &mut ScriptContext) {
        if self.health <= 0.0 {
                    ctx.message_sender.send_global(EnemyDiedMsg { position: ctx.scene.graph[ctx.handle].global_position() });
                }
    }
}


#[derive(Visit, Reflect, Debug, Clone, TypeUuidProvider, ComponentProvider)]
#[type_uuid(id = "794caf2c-d47b-3003-b20f-28cb864812fb")]
#[visit(optional)]
pub struct ScoreTracker {
    #[reflect(expand)]
    pub score: i32,
}

impl Default for ScoreTracker {
    fn default() -> Self {
        let value = Self {
            score: 0,
        };
        value
    }
}

impl ScriptTrait for ScoreTracker {
    fn on_start(&mut self, ctx: &mut ScriptContext) {
        ctx.message_dispatcher.subscribe_to::<EnemyDiedMsg>(ctx.handle);
    }

    fn on_message(&mut self, message: &mut dyn ScriptMessagePayload, ctx: &mut ScriptMessageContext) {
        if let Some(msg) = message.downcast_ref::<EnemyDiedMsg>() {
            let pos = &msg.position;
            self.score += 100;
        }
    }
}


pub fn register_scripts(ctx: &mut PluginRegistrationContext) {
    ctx.serialization_context.script_constructors.add::<Enemy>("Enemy");
    ctx.serialization_context.script_constructors.add::<ScoreTracker>("ScoreTracker");
}
