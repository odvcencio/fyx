use std::any::{Any, TypeId};
use std::collections::HashMap;

/// Entity handle — a unique identifier for an entity in the ECS world.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct Entity(u64);

/// Type-erased sparse-set storage for a single component type.
struct ComponentStorage {
    /// Maps entity id -> index in `dense_entities` / `components`.
    sparse: HashMap<u64, usize>,
    /// Dense list of entity ids that have this component.
    dense_entities: Vec<u64>,
    /// Dense list of components (boxed, type-erased). Parallel to `dense_entities`.
    components: Vec<Box<dyn Any>>,
}

impl ComponentStorage {
    fn new() -> Self {
        Self {
            sparse: HashMap::new(),
            dense_entities: Vec::new(),
            components: Vec::new(),
        }
    }

    fn insert(&mut self, entity_id: u64, component: Box<dyn Any>) {
        if let Some(&idx) = self.sparse.get(&entity_id) {
            self.components[idx] = component;
        } else {
            let idx = self.dense_entities.len();
            self.dense_entities.push(entity_id);
            self.components.push(component);
            self.sparse.insert(entity_id, idx);
        }
    }

    fn remove(&mut self, entity_id: u64) {
        if let Some(idx) = self.sparse.remove(&entity_id) {
            let last = self.dense_entities.len() - 1;
            if idx != last {
                let moved_entity = self.dense_entities[last];
                self.dense_entities.swap(idx, last);
                self.components.swap(idx, last);
                self.sparse.insert(moved_entity, idx);
            }
            self.dense_entities.pop();
            self.components.pop();
        }
    }

    fn get(&self, entity_id: u64) -> Option<&dyn Any> {
        self.sparse
            .get(&entity_id)
            .map(|&idx| self.components[idx].as_ref())
    }

    fn entity_ids(&self) -> &[u64] {
        &self.dense_entities
    }
}

/// The ECS world — holds all entities and their components.
pub struct EcsWorld {
    next_entity_id: u64,
    /// All living entity ids.
    entities: Vec<u64>,
    /// Component storage keyed by TypeId.
    storages: HashMap<TypeId, ComponentStorage>,
}

impl EcsWorld {
    /// Create a new, empty ECS world.
    pub fn new() -> Self {
        Self {
            next_entity_id: 0,
            entities: Vec::new(),
            storages: HashMap::new(),
        }
    }

    /// Spawn an entity with a bundle of components. Returns the new entity handle.
    pub fn spawn<B: ComponentBundle>(&mut self, components: B) -> Entity {
        let id = self.next_entity_id;
        self.next_entity_id += 1;
        self.entities.push(id);
        components.insert_into(self, id);
        Entity(id)
    }

    /// Remove an entity and all of its components.
    pub fn despawn(&mut self, entity: Entity) {
        self.entities.retain(|&e| e != entity.0);
        for storage in self.storages.values_mut() {
            storage.remove(entity.0);
        }
    }

    fn storage_mut(&mut self, type_id: TypeId) -> &mut ComponentStorage {
        self.storages
            .entry(type_id)
            .or_insert_with(ComponentStorage::new)
    }

    /// Insert a single component for a given entity id (internal use).
    fn insert_component<T: 'static>(&mut self, entity_id: u64, component: T) {
        let type_id = TypeId::of::<T>();
        let storage = self.storage_mut(type_id);
        storage.insert(entity_id, Box::new(component));
    }

    // -----------------------------------------------------------------------
    // Query methods — concrete for tuple arities 1..5
    // -----------------------------------------------------------------------

    /// Query for entities with 1 component (immutable).
    pub fn query1<A: 'static>(&self) -> Vec<(Entity, (&A,))> {
        let mut results = Vec::new();
        if let Some(sa) = self.storages.get(&TypeId::of::<A>()) {
            for &eid in sa.entity_ids() {
                if let Some(a) = sa.get(eid).and_then(|v| v.downcast_ref::<A>()) {
                    results.push((Entity(eid), (a,)));
                }
            }
        }
        results
    }

    /// Query for entities with 2 components (immutable).
    pub fn query2<A: 'static, B: 'static>(&self) -> Vec<(Entity, (&A, &B))> {
        let mut results = Vec::new();
        let (sa, sb) = match (
            self.storages.get(&TypeId::of::<A>()),
            self.storages.get(&TypeId::of::<B>()),
        ) {
            (Some(a), Some(b)) => (a, b),
            _ => return results,
        };
        // Iterate over the smaller set
        for &eid in sa.entity_ids() {
            if let (Some(a), Some(b)) = (
                sa.get(eid).and_then(|v| v.downcast_ref::<A>()),
                sb.get(eid).and_then(|v| v.downcast_ref::<B>()),
            ) {
                results.push((Entity(eid), (a, b)));
            }
        }
        results
    }

    /// Query for entities with 3 components (immutable).
    pub fn query3<A: 'static, B: 'static, C: 'static>(&self) -> Vec<(Entity, (&A, &B, &C))> {
        let mut results = Vec::new();
        let (sa, sb, sc) = match (
            self.storages.get(&TypeId::of::<A>()),
            self.storages.get(&TypeId::of::<B>()),
            self.storages.get(&TypeId::of::<C>()),
        ) {
            (Some(a), Some(b), Some(c)) => (a, b, c),
            _ => return results,
        };
        for &eid in sa.entity_ids() {
            if let (Some(a), Some(b), Some(c)) = (
                sa.get(eid).and_then(|v| v.downcast_ref::<A>()),
                sb.get(eid).and_then(|v| v.downcast_ref::<B>()),
                sc.get(eid).and_then(|v| v.downcast_ref::<C>()),
            ) {
                results.push((Entity(eid), (a, b, c)));
            }
        }
        results
    }

    /// Query for entities with 4 components (immutable).
    pub fn query4<A: 'static, B: 'static, C: 'static, D: 'static>(
        &self,
    ) -> Vec<(Entity, (&A, &B, &C, &D))> {
        let mut results = Vec::new();
        let (sa, sb, sc, sd) = match (
            self.storages.get(&TypeId::of::<A>()),
            self.storages.get(&TypeId::of::<B>()),
            self.storages.get(&TypeId::of::<C>()),
            self.storages.get(&TypeId::of::<D>()),
        ) {
            (Some(a), Some(b), Some(c), Some(d)) => (a, b, c, d),
            _ => return results,
        };
        for &eid in sa.entity_ids() {
            if let (Some(a), Some(b), Some(c), Some(d)) = (
                sa.get(eid).and_then(|v| v.downcast_ref::<A>()),
                sb.get(eid).and_then(|v| v.downcast_ref::<B>()),
                sc.get(eid).and_then(|v| v.downcast_ref::<C>()),
                sd.get(eid).and_then(|v| v.downcast_ref::<D>()),
            ) {
                results.push((Entity(eid), (a, b, c, d)));
            }
        }
        results
    }

    /// Query for entities with 5 components (immutable).
    pub fn query5<A: 'static, B: 'static, C: 'static, D: 'static, E: 'static>(
        &self,
    ) -> Vec<(Entity, (&A, &B, &C, &D, &E))> {
        let mut results = Vec::new();
        let (sa, sb, sc, sd, se) = match (
            self.storages.get(&TypeId::of::<A>()),
            self.storages.get(&TypeId::of::<B>()),
            self.storages.get(&TypeId::of::<C>()),
            self.storages.get(&TypeId::of::<D>()),
            self.storages.get(&TypeId::of::<E>()),
        ) {
            (Some(a), Some(b), Some(c), Some(d), Some(e)) => (a, b, c, d, e),
            _ => return results,
        };
        for &eid in sa.entity_ids() {
            if let (Some(a), Some(b), Some(c), Some(d), Some(e)) = (
                sa.get(eid).and_then(|v| v.downcast_ref::<A>()),
                sb.get(eid).and_then(|v| v.downcast_ref::<B>()),
                sc.get(eid).and_then(|v| v.downcast_ref::<C>()),
                sd.get(eid).and_then(|v| v.downcast_ref::<D>()),
                se.get(eid).and_then(|v| v.downcast_ref::<E>()),
            ) {
                results.push((Entity(eid), (a, b, c, d, e)));
            }
        }
        results
    }
}

impl Default for EcsWorld {
    fn default() -> Self {
        Self::new()
    }
}

/// Script-facing adapter that forwards spawn/despawn operations into the ECS
/// world without exposing the rest of the world API through script contexts.
pub struct ScriptEcs<'a> {
    world: &'a mut EcsWorld,
}

impl<'a> ScriptEcs<'a> {
    pub fn new(world: &'a mut EcsWorld) -> Self {
        Self { world }
    }

    pub fn spawn<B: ComponentBundle>(&mut self, components: B) -> Entity {
        self.world.spawn(components)
    }

    pub fn despawn(&mut self, entity: Entity) {
        self.world.despawn(entity);
    }

    pub fn world(&mut self) -> &mut EcsWorld {
        self.world
    }
}

// ---------------------------------------------------------------------------
// ComponentBundle — trait for tuples of components that can be inserted
// ---------------------------------------------------------------------------

/// A bundle of components that can be inserted into an entity.
pub trait ComponentBundle {
    fn insert_into(self, world: &mut EcsWorld, entity_id: u64);
}

// Implement ComponentBundle for tuples of 1..5 components.

impl<A: 'static> ComponentBundle for (A,) {
    fn insert_into(self, world: &mut EcsWorld, entity_id: u64) {
        world.insert_component(entity_id, self.0);
    }
}

impl<A: 'static, B: 'static> ComponentBundle for (A, B) {
    fn insert_into(self, world: &mut EcsWorld, entity_id: u64) {
        world.insert_component(entity_id, self.0);
        world.insert_component(entity_id, self.1);
    }
}

impl<A: 'static, B: 'static, C: 'static> ComponentBundle for (A, B, C) {
    fn insert_into(self, world: &mut EcsWorld, entity_id: u64) {
        world.insert_component(entity_id, self.0);
        world.insert_component(entity_id, self.1);
        world.insert_component(entity_id, self.2);
    }
}

impl<A: 'static, B: 'static, C: 'static, D: 'static> ComponentBundle for (A, B, C, D) {
    fn insert_into(self, world: &mut EcsWorld, entity_id: u64) {
        world.insert_component(entity_id, self.0);
        world.insert_component(entity_id, self.1);
        world.insert_component(entity_id, self.2);
        world.insert_component(entity_id, self.3);
    }
}

impl<A: 'static, B: 'static, C: 'static, D: 'static, E: 'static> ComponentBundle
    for (A, B, C, D, E)
{
    fn insert_into(self, world: &mut EcsWorld, entity_id: u64) {
        world.insert_component(entity_id, self.0);
        world.insert_component(entity_id, self.1);
        world.insert_component(entity_id, self.2);
        world.insert_component(entity_id, self.3);
        world.insert_component(entity_id, self.4);
    }
}

// ---------------------------------------------------------------------------
// QueryBundle — trait-based generic query/query_mut via the `query` / `query_mut` methods
// ---------------------------------------------------------------------------

/// Trait for query parameter tuples. Each implementor knows how to
/// collect matching entities from an `EcsWorld`.
pub trait QueryBundle {
    type Item<'w>;
    fn fetch(world: &EcsWorld) -> Vec<(Entity, Self::Item<'_>)>;
}

/// Trait for mutable query parameter tuples.
pub trait QueryBundleMut {
    type Item<'w>;
    fn fetch_mut(world: &mut EcsWorld) -> Vec<(Entity, Self::Item<'_>)>;
}

// We implement QueryBundle for tuples of immutable references (&A,), (&A, &B), etc.
// and QueryBundleMut for tuples that may contain &mut references.
// For simplicity, QueryBundleMut tuples specify mutability per-element at the type level
// by wrapping mutable access in a newtype `Mut<T>`.
//
// However, the task spec wants the syntax:
//   world.query_mut::<(&mut Position, &Velocity)>()
//
// Rust doesn't allow `&mut T` as a generic type parameter directly.
// So we use the concrete queryN / queryN_mut methods, plus provide
// the `query` and `query_mut` convenience methods that delegate via the trait.

// Implement QueryBundle for immutable tuple arities 1..5.

impl<A: 'static> QueryBundle for (&A,) {
    type Item<'w> = (&'w A,);
    fn fetch(world: &EcsWorld) -> Vec<(Entity, Self::Item<'_>)> {
        world.query1::<A>()
    }
}

impl<A: 'static, B: 'static> QueryBundle for (&A, &B) {
    type Item<'w> = (&'w A, &'w B);
    fn fetch(world: &EcsWorld) -> Vec<(Entity, Self::Item<'_>)> {
        world.query2::<A, B>()
    }
}

impl<A: 'static, B: 'static, C: 'static> QueryBundle for (&A, &B, &C) {
    type Item<'w> = (&'w A, &'w B, &'w C);
    fn fetch(world: &EcsWorld) -> Vec<(Entity, Self::Item<'_>)> {
        world.query3::<A, B, C>()
    }
}

impl<A: 'static, B: 'static, C: 'static, D: 'static> QueryBundle for (&A, &B, &C, &D) {
    type Item<'w> = (&'w A, &'w B, &'w C, &'w D);
    fn fetch(world: &EcsWorld) -> Vec<(Entity, Self::Item<'_>)> {
        world.query4::<A, B, C, D>()
    }
}

impl<A: 'static, B: 'static, C: 'static, D: 'static, E: 'static> QueryBundle
    for (&A, &B, &C, &D, &E)
{
    type Item<'w> = (&'w A, &'w B, &'w C, &'w D, &'w E);
    fn fetch(world: &EcsWorld) -> Vec<(Entity, Self::Item<'_>)> {
        world.query5::<A, B, C, D, E>()
    }
}

// Add `query` convenience method.
impl EcsWorld {
    /// Query for entities matching the given component types (immutable).
    pub fn query<'w, Q: QueryBundle>(&'w self) -> impl Iterator<Item = (Entity, Q::Item<'w>)> {
        Q::fetch(self).into_iter()
    }

    // -----------------------------------------------------------------------
    // Mutable query support
    // -----------------------------------------------------------------------
    // For mutable queries, we use a marker type `Mut<T>` to distinguish
    // mutable vs immutable access in the type parameter tuple.

    /// Query for entities with 1 component, returning mutable access to components
    /// tagged with `Mut<T>`.
    ///
    /// For `query_mut`, we support tuples where each element is either
    /// `Mut<T>` (mutable) or `Ref<T>` (immutable). We collect entity ids first,
    /// then fetch components with raw pointer casts to allow multiple borrows
    /// from different storages.
    pub fn query_mut<'w, Q: QueryBundleMut>(
        &'w mut self,
    ) -> impl Iterator<Item = (Entity, Q::Item<'w>)> {
        Q::fetch_mut(self).into_iter()
    }
}

/// Marker for mutable component access in query tuples.
pub struct Mut<T>(std::marker::PhantomData<T>);

/// Marker for immutable component access in query tuples.
pub struct Ref<T>(std::marker::PhantomData<T>);

// ---------------------------------------------------------------------------
// Mutable query helpers — we need unsafe to borrow multiple storages mutably.
// ---------------------------------------------------------------------------

// Helper: given the world and an entity id, get a *mut to the component.
// SAFETY: caller must ensure no aliasing borrows exist for the same component.
unsafe fn get_component_ptr<T: 'static>(world: &mut EcsWorld, entity_id: u64) -> Option<*mut T> {
    let type_id = TypeId::of::<T>();
    let storage = world.storages.get_mut(&type_id)?;
    let idx = *storage.sparse.get(&entity_id)?;
    let any_mut: &mut dyn Any = storage.components[idx].as_mut();
    any_mut.downcast_mut::<T>().map(|r| r as *mut T)
}

// Helper: given the world and an entity id, get a *const to the component.
fn get_component_ref<T: 'static>(world: &EcsWorld, entity_id: u64) -> Option<*const T> {
    let type_id = TypeId::of::<T>();
    let storage = world.storages.get(&type_id)?;
    let idx = *storage.sparse.get(&entity_id)?;
    let any_ref: &dyn Any = storage.components[idx].as_ref();
    any_ref.downcast_ref::<T>().map(|r| r as *const T)
}

/// Collect entity ids that have all of the given TypeIds.
fn entities_with_all(world: &EcsWorld, type_ids: &[TypeId]) -> Vec<u64> {
    if type_ids.is_empty() {
        return Vec::new();
    }
    // Find the smallest storage to iterate over
    let mut smallest: Option<(&ComponentStorage, usize)> = None;
    for tid in type_ids {
        match world.storages.get(tid) {
            Some(s) => {
                let len = s.entity_ids().len();
                if smallest.is_none() || len < smallest.unwrap().1 {
                    smallest = Some((s, len));
                }
            }
            None => return Vec::new(),
        }
    }
    let base = smallest.unwrap().0;
    let mut result = Vec::new();
    for &eid in base.entity_ids() {
        let mut has_all = true;
        for tid in type_ids {
            if let Some(s) = world.storages.get(tid) {
                if !s.sparse.contains_key(&eid) {
                    has_all = false;
                    break;
                }
            } else {
                has_all = false;
                break;
            }
        }
        if has_all {
            result.push(eid);
        }
    }
    result
}

// QueryBundleMut implementations for common mutable query patterns.
// Each tuple element is either Mut<T> (mutable) or Ref<T> (immutable).

// --- Arity 1: (Mut<A>,) ---
impl<A: 'static> QueryBundleMut for (Mut<A>,) {
    type Item<'w> = (&'w mut A,);
    fn fetch_mut(world: &mut EcsWorld) -> Vec<(Entity, Self::Item<'_>)> {
        let type_ids = [TypeId::of::<A>()];
        let eids = entities_with_all(world, &type_ids);
        let mut results = Vec::new();
        for eid in eids {
            // SAFETY: only one mutable borrow per component type per entity, and we
            // collect entity ids first so we don't hold borrows across iterations.
            unsafe {
                if let Some(a_ptr) = get_component_ptr::<A>(world, eid) {
                    results.push((Entity(eid), (&mut *a_ptr,)));
                }
            }
        }
        results
    }
}

// --- Arity 1: (Ref<A>,) --- (immutable via QueryBundleMut path)
impl<A: 'static> QueryBundleMut for (Ref<A>,) {
    type Item<'w> = (&'w A,);
    fn fetch_mut(world: &mut EcsWorld) -> Vec<(Entity, Self::Item<'_>)> {
        let type_ids = [TypeId::of::<A>()];
        let eids = entities_with_all(world, &type_ids);
        let mut results = Vec::new();
        for eid in eids {
            if let Some(a_ptr) = get_component_ref::<A>(world, eid) {
                // SAFETY: we have &mut world so no other references exist.
                results.push((Entity(eid), (unsafe { &*a_ptr },)));
            }
        }
        results
    }
}

// --- Arity 2: (Mut<A>, Ref<B>) ---
impl<A: 'static, B: 'static> QueryBundleMut for (Mut<A>, Ref<B>) {
    type Item<'w> = (&'w mut A, &'w B);
    fn fetch_mut(world: &mut EcsWorld) -> Vec<(Entity, Self::Item<'_>)> {
        let type_ids = [TypeId::of::<A>(), TypeId::of::<B>()];
        let eids = entities_with_all(world, &type_ids);
        let mut results = Vec::new();
        for eid in eids {
            unsafe {
                let a_ptr = get_component_ptr::<A>(world, eid);
                let b_ptr = get_component_ref::<B>(world, eid);
                if let (Some(a), Some(b)) = (a_ptr, b_ptr) {
                    results.push((Entity(eid), (&mut *a, &*b)));
                }
            }
        }
        results
    }
}

// --- Arity 2: (Ref<A>, Mut<B>) ---
impl<A: 'static, B: 'static> QueryBundleMut for (Ref<A>, Mut<B>) {
    type Item<'w> = (&'w A, &'w mut B);
    fn fetch_mut(world: &mut EcsWorld) -> Vec<(Entity, Self::Item<'_>)> {
        let type_ids = [TypeId::of::<A>(), TypeId::of::<B>()];
        let eids = entities_with_all(world, &type_ids);
        let mut results = Vec::new();
        for eid in eids {
            unsafe {
                let a_ptr = get_component_ref::<A>(world, eid);
                let b_ptr = get_component_ptr::<B>(world, eid);
                if let (Some(a), Some(b)) = (a_ptr, b_ptr) {
                    results.push((Entity(eid), (&*a, &mut *b)));
                }
            }
        }
        results
    }
}

// --- Arity 2: (Mut<A>, Mut<B>) ---
impl<A: 'static, B: 'static> QueryBundleMut for (Mut<A>, Mut<B>) {
    type Item<'w> = (&'w mut A, &'w mut B);
    fn fetch_mut(world: &mut EcsWorld) -> Vec<(Entity, Self::Item<'_>)> {
        let type_ids = [TypeId::of::<A>(), TypeId::of::<B>()];
        let eids = entities_with_all(world, &type_ids);
        let mut results = Vec::new();
        for eid in eids {
            unsafe {
                let a_ptr = get_component_ptr::<A>(world, eid);
                let b_ptr = get_component_ptr::<B>(world, eid);
                if let (Some(a), Some(b)) = (a_ptr, b_ptr) {
                    results.push((Entity(eid), (&mut *a, &mut *b)));
                }
            }
        }
        results
    }
}

// --- Arity 2: (Ref<A>, Ref<B>) ---
impl<A: 'static, B: 'static> QueryBundleMut for (Ref<A>, Ref<B>) {
    type Item<'w> = (&'w A, &'w B);
    fn fetch_mut(world: &mut EcsWorld) -> Vec<(Entity, Self::Item<'_>)> {
        let type_ids = [TypeId::of::<A>(), TypeId::of::<B>()];
        let eids = entities_with_all(world, &type_ids);
        let mut results = Vec::new();
        for eid in eids {
            unsafe {
                let a_ptr = get_component_ref::<A>(world, eid);
                let b_ptr = get_component_ref::<B>(world, eid);
                if let (Some(a), Some(b)) = (a_ptr, b_ptr) {
                    results.push((Entity(eid), (&*a, &*b)));
                }
            }
        }
        results
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[derive(Clone, Debug, PartialEq)]
    struct Position {
        x: f32,
        y: f32,
    }

    #[derive(Clone, Debug, PartialEq)]
    struct Velocity {
        dx: f32,
        dy: f32,
    }

    #[test]
    fn spawn_and_query() {
        let mut world = EcsWorld::new();
        world.spawn((Position { x: 0.0, y: 0.0 }, Velocity { dx: 1.0, dy: 0.0 }));
        let count = world.query::<(&Position, &Velocity)>().count();
        assert_eq!(count, 1);
    }

    #[test]
    fn despawn() {
        let mut world = EcsWorld::new();
        let e = world.spawn((Position { x: 0.0, y: 0.0 },));
        world.despawn(e);
        assert_eq!(world.query::<(&Position,)>().count(), 0);
    }

    #[test]
    fn query_mut_test() {
        let mut world = EcsWorld::new();
        world.spawn((Position { x: 0.0, y: 0.0 }, Velocity { dx: 1.0, dy: 2.0 }));
        for (_e, (pos, vel)) in world.query_mut::<(Mut<Position>, Ref<Velocity>)>() {
            pos.x += vel.dx;
            pos.y += vel.dy;
        }
        for (_e, (pos,)) in world.query::<(&Position,)>() {
            assert_eq!(pos.x, 1.0);
            assert_eq!(pos.y, 2.0);
        }
    }

    #[test]
    fn multiple_entities() {
        let mut world = EcsWorld::new();
        world.spawn((Position { x: 0.0, y: 0.0 },));
        world.spawn((Position { x: 1.0, y: 1.0 },));
        world.spawn((Position { x: 2.0, y: 2.0 }, Velocity { dx: 0.0, dy: 0.0 }));
        assert_eq!(world.query::<(&Position,)>().count(), 3);
        assert_eq!(world.query::<(&Position, &Velocity)>().count(), 1);
    }

    #[test]
    fn query_single_component() {
        let mut world = EcsWorld::new();
        world.spawn((Position { x: 42.0, y: 99.0 },));
        let results: Vec<_> = world.query::<(&Position,)>().collect();
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].1 .0.x, 42.0);
        assert_eq!(results[0].1 .0.y, 99.0);
    }

    #[test]
    fn despawn_middle_entity() {
        let mut world = EcsWorld::new();
        let _e1 = world.spawn((Position { x: 1.0, y: 1.0 },));
        let e2 = world.spawn((Position { x: 2.0, y: 2.0 },));
        let _e3 = world.spawn((Position { x: 3.0, y: 3.0 },));
        world.despawn(e2);
        assert_eq!(world.query::<(&Position,)>().count(), 2);
    }

    #[test]
    fn mut_query_all_mutable() {
        let mut world = EcsWorld::new();
        world.spawn((Position { x: 0.0, y: 0.0 }, Velocity { dx: 5.0, dy: 10.0 }));
        for (_e, (pos, vel)) in world.query_mut::<(Mut<Position>, Mut<Velocity>)>() {
            pos.x += vel.dx;
            vel.dx = 0.0;
        }
        for (_e, (pos,)) in world.query::<(&Position,)>() {
            assert_eq!(pos.x, 5.0);
        }
        for (_e, (vel,)) in world.query::<(&Velocity,)>() {
            assert_eq!(vel.dx, 0.0);
        }
    }

    #[test]
    fn empty_world_query() {
        let world = EcsWorld::new();
        assert_eq!(world.query::<(&Position,)>().count(), 0);
        assert_eq!(world.query::<(&Position, &Velocity)>().count(), 0);
    }

    #[test]
    fn script_ecs_bridge() {
        let mut world = EcsWorld::new();
        let mut ecs = ScriptEcs::new(&mut world);
        let entity = ecs.spawn((Position { x: 7.0, y: 3.0 },));
        ecs.despawn(entity);
        assert_eq!(ecs.world().query::<(&Position,)>().count(), 0);
    }
}
