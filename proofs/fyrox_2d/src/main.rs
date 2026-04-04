use fyrox::core::log::Log;
use fyrox::engine::executor::Executor;
use fyrox::event_loop::EventLoop;
use fyx_fyrox_2d::Demo2DGame;

fn main() {
    Log::set_file_name("fyx-fyrox-2d.log");

    let mut executor = Executor::new(Some(EventLoop::new().unwrap()));
    executor.add_plugin(Demo2DGame::default());
    executor.run()
}
