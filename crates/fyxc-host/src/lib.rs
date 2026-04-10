use std::{
    ffi::OsStr,
    fmt, fs, io,
    path::{Path, PathBuf},
    process::{Command, Output},
};

#[cfg(unix)]
use std::os::unix::fs::PermissionsExt;

include!(concat!(env!("OUT_DIR"), "/embedded_binary.rs"));

#[derive(Debug)]
pub enum Error {
    Io(io::Error),
    CommandFailed(CommandFailure),
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Io(err) => write!(f, "{err}"),
            Self::CommandFailed(err) => err.fmt(f),
        }
    }
}

impl std::error::Error for Error {}

impl From<io::Error> for Error {
    fn from(value: io::Error) -> Self {
        Self::Io(value)
    }
}

#[derive(Debug)]
pub struct CommandFailure {
    pub status: std::process::ExitStatus,
    pub stdout: String,
    pub stderr: String,
}

impl fmt::Display for CommandFailure {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "fyxc exited with status {}",
            self.status
                .code()
                .map(|code| code.to_string())
                .unwrap_or_else(|| "signal".to_string())
        )?;
        if !self.stderr.trim().is_empty() {
            write!(f, ": {}", self.stderr.trim())?;
        }
        Ok(())
    }
}

pub fn binary_bytes() -> &'static [u8] {
    FYXC_BINARY
}

pub fn extract_binary<P: AsRef<Path>>(dir: P) -> Result<PathBuf, Error> {
    let dir = dir.as_ref();
    fs::create_dir_all(dir)?;
    let path = dir.join(FYXC_BINARY_NAME);

    let should_write = match fs::read(&path) {
        Ok(existing) => existing != FYXC_BINARY,
        Err(err) if err.kind() == io::ErrorKind::NotFound => true,
        Err(err) => return Err(Error::Io(err)),
    };
    if should_write {
        fs::write(&path, FYXC_BINARY)?;
    }

    #[cfg(unix)]
    {
        let mut permissions = fs::metadata(&path)?.permissions();
        permissions.set_mode(0o755);
        fs::set_permissions(&path, permissions)?;
    }

    Ok(path)
}

pub fn temp_binary_dir() -> PathBuf {
    std::env::temp_dir()
        .join("fyxc-host")
        .join(env!("CARGO_PKG_VERSION"))
}

pub fn extract_temp_binary() -> Result<PathBuf, Error> {
    extract_binary(temp_binary_dir())
}

pub fn command_in<P: AsRef<Path>>(dir: P) -> Result<Command, Error> {
    Ok(Command::new(extract_binary(dir)?))
}

pub fn command() -> Result<Command, Error> {
    Ok(Command::new(extract_temp_binary()?))
}

pub fn run<I, S>(args: I) -> Result<Output, Error>
where
    I: IntoIterator<Item = S>,
    S: AsRef<OsStr>,
{
    let mut command = command()?;
    command.args(args);
    run_command(&mut command)
}

pub fn run_in<P, I, S>(dir: P, args: I) -> Result<Output, Error>
where
    P: AsRef<Path>,
    I: IntoIterator<Item = S>,
    S: AsRef<OsStr>,
{
    let mut command = command_in(dir)?;
    command.args(args);
    run_command(&mut command)
}

pub fn run_command(command: &mut Command) -> Result<Output, Error> {
    let output = command.output()?;
    if output.status.success() {
        return Ok(output);
    }
    Err(Error::CommandFailed(CommandFailure {
        status: output.status,
        stdout: String::from_utf8_lossy(&output.stdout).into_owned(),
        stderr: String::from_utf8_lossy(&output.stderr).into_owned(),
    }))
}

pub mod build_support {
    use super::{run_in, Error};
    use std::path::{Path, PathBuf};

    fn tool_cache_dir(out_dir: &Path) -> PathBuf {
        out_dir.join(".fyxc-host")
    }

    pub fn build_fyx<P: AsRef<Path>, Q: AsRef<Path>>(fyx_dir: P, out_dir: Q) -> Result<(), Error> {
        let fyx_dir = fyx_dir.as_ref();
        let out_dir = out_dir.as_ref();
        let args = [
            OsString::from("build"),
            fyx_dir.as_os_str().to_os_string(),
            OsString::from("--out"),
            out_dir.as_os_str().to_os_string(),
        ];
        run_in(tool_cache_dir(out_dir), args)?;
        Ok(())
    }

    use std::ffi::OsString;
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::{SystemTime, UNIX_EPOCH};

    fn unique_temp_dir() -> PathBuf {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("time")
            .as_nanos();
        std::env::temp_dir().join(format!("fyxc-host-test-{nanos}"))
    }

    #[test]
    fn extracts_embedded_binary() {
        let dir = unique_temp_dir();
        let path = extract_binary(&dir).expect("extract binary");
        assert!(path.exists(), "embedded fyxc should be written to disk");
        assert!(
            !binary_bytes().is_empty(),
            "embedded fyxc bytes should not be empty"
        );
        fs::remove_dir_all(dir).ok();
    }
}
