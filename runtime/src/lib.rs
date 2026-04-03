//! fyx-runtime — Minimal ECS runtime for Fyx transpiled code.
//!
//! Provides sparse-set ECS storage with typed component access.
//! No external dependencies.

pub mod ecs;

pub use ecs::{
    ComponentBundle, EcsWorld, Entity, Mut, QueryBundle, QueryBundleMut, Ref,
};
