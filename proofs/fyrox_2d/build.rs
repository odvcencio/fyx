use std::{env, fs, path::PathBuf};

fn main() {
    let manifest_dir = PathBuf::from(env::var("CARGO_MANIFEST_DIR").expect("manifest dir"));
    let out_dir = PathBuf::from(env::var("OUT_DIR").expect("out dir"));
    let generated_dir = out_dir.join("fyx_generated");
    let wrapper_path = out_dir.join("generated_modules.rs");
    let fyx_dir = manifest_dir.join("fyx");

    println!("cargo:rerun-if-changed={}", fyx_dir.display());

    fs::create_dir_all(&generated_dir).expect("create generated dir");

    fyxc_host::build_support::build_fyx(&fyx_dir, &generated_dir)
        .unwrap_or_else(|err| panic!("fyxc build failed for {}: {}", fyx_dir.display(), err));

    let generated_mod = generated_dir.join("mod.rs");
    let wrapper = format!(
        "#[path = r#\"{}\"#]\npub mod generated;\n",
        generated_mod.display()
    );
    fs::write(wrapper_path, wrapper).expect("write generated wrapper");
}
