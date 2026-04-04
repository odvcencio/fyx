use std::{
    env, fs,
    path::{Path, PathBuf},
    process::Command,
};

fn main() {
    let manifest_dir = PathBuf::from(env::var("CARGO_MANIFEST_DIR").expect("manifest dir"));
    let repo_root = manifest_dir
        .parent()
        .and_then(Path::parent)
        .expect("repo root")
        .to_path_buf();
    let out_dir = PathBuf::from(env::var("OUT_DIR").expect("out dir"));
    let generated_dir = out_dir.join("fyx_generated");
    let wrapper_path = out_dir.join("generated_modules.rs");
    let fyx_dir = manifest_dir.join("fyx");

    println!("cargo:rerun-if-changed={}", fyx_dir.display());
    println!("cargo:rerun-if-changed={}", repo_root.join("ast").display());
    println!("cargo:rerun-if-changed={}", repo_root.join("cmd/fyxc").display());
    println!("cargo:rerun-if-changed={}", repo_root.join("grammar").display());
    println!("cargo:rerun-if-changed={}", repo_root.join("transpiler").display());

    fs::create_dir_all(&generated_dir).expect("create generated dir");

    let status = Command::new("go")
        .current_dir(&repo_root)
        .args([
            "run",
            "./cmd/fyxc",
            "build",
            fyx_dir.to_str().expect("fyx dir"),
            "--out",
            generated_dir.to_str().expect("generated dir"),
        ])
        .status()
        .expect("run fyxc");
    if !status.success() {
        panic!("fyxc build failed for {}", fyx_dir.display());
    }

    let generated_mod = generated_dir.join("mod.rs");
    let wrapper = format!(
        "#[path = r#\"{}\"#]\npub mod generated;\n",
        generated_mod.display()
    );
    fs::write(wrapper_path, wrapper).expect("write generated wrapper");
}
