pub const FYX_ARBITER_BUNDLE: &str = r#"source npc_senses {
    path sensor://vision
}

worker decide_directive {
    input ThreatOutcome
    output NpcDirective
    exec "npc-directive"
}

rule PlayerDetected {
    when distance_to_player < 8.0
    then threat_high
}

arbiter npc_brain {
    poll every_frame
    use_worker decide_directive
}"#;


#[derive(Clone)]
pub struct SpawnedTag {
    pub active: bool,
}


#[derive(Visit, Reflect, Debug, Clone, TypeUuidProvider, ComponentProvider)]
#[type_uuid(id = "a5b42962-b52d-d7b2-ba79-49baffdad5f9")]
#[visit(optional)]
pub struct SpawnBridge {
}

impl Default for SpawnBridge {
    fn default() -> Self {
        let value = Self {
        };
        value
    }
}

impl ScriptTrait for SpawnBridge {
    fn on_update(&mut self, ctx: &mut ScriptContext) {
        let spawned = ctx.ecs.spawn((SpawnedTag { active: true },));
    }
}


pub fn register_scripts(ctx: &mut PluginRegistrationContext) {
    ctx.serialization_context.script_constructors.add::<SpawnBridge>("SpawnBridge");
}
