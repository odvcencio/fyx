use std::{env, fs, path::PathBuf};

fn artifact_dir_for_host(host: &str) -> Option<&'static str> {
    match host {
        "x86_64-unknown-linux-gnu" | "x86_64-unknown-linux-musl" => Some("linux-amd64"),
        "aarch64-unknown-linux-gnu" | "aarch64-unknown-linux-musl" => Some("linux-arm64"),
        "x86_64-apple-darwin" => Some("darwin-amd64"),
        "aarch64-apple-darwin" => Some("darwin-arm64"),
        "x86_64-pc-windows-msvc" | "x86_64-pc-windows-gnu" => Some("windows-amd64"),
        "aarch64-pc-windows-msvc" => Some("windows-arm64"),
        _ => None,
    }
}

fn main() {
    let manifest_dir = PathBuf::from(env::var("CARGO_MANIFEST_DIR").expect("manifest dir"));
    let out_dir = PathBuf::from(env::var("OUT_DIR").expect("out dir"));
    let host = env::var("HOST").expect("host triple");
    let artifact_dir = artifact_dir_for_host(&host).unwrap_or_else(|| {
        panic!(
            "unsupported fyxc-host build platform {host}; supported hosts: x86_64/aarch64 linux, x86_64/aarch64 windows, x86_64/aarch64 macOS"
        )
    });
    let binary_name = if host.contains("windows") {
        "fyxc.exe"
    } else {
        "fyxc"
    };
    let embedded_source = manifest_dir.join("bin").join(artifact_dir).join(binary_name);
    let generated_path = out_dir.join("embedded_binary.rs");

    println!("cargo:rerun-if-changed={}", embedded_source.display());
    println!("cargo:rerun-if-changed={}", manifest_dir.join("build.rs").display());

    if !embedded_source.exists() {
        panic!(
            "missing embedded fyxc binary for host {host}: {}",
            embedded_source.display()
        );
    }

    let generated = format!(
        "pub const FYXC_BINARY_NAME: &str = {:?};\npub static FYXC_BINARY: &[u8] = include_bytes!(r#\"{}\"#);\n",
        binary_name,
        embedded_source.display(),
    );
    fs::write(generated_path, generated).expect("write embedded binary metadata");
}
