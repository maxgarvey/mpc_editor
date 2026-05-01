use std::net::TcpListener;
use std::process::Command;
use std::sync::{Mutex, OnceLock};
use std::time::{Duration, Instant};
use tauri::{Manager, WebviewUrl, WebviewWindowBuilder};

static GO_PROCESS: OnceLock<Mutex<std::process::Child>> = OnceLock::new();

fn find_free_port() -> u16 {
    TcpListener::bind("127.0.0.1:0")
        .expect("bind random port")
        .local_addr()
        .unwrap()
        .port()
}

fn wait_for_port(port: u16, timeout: Duration) -> bool {
    let deadline = Instant::now() + timeout;
    loop {
        if std::net::TcpStream::connect(("127.0.0.1", port)).is_ok() {
            return true;
        }
        if Instant::now() >= deadline {
            return false;
        }
        std::thread::sleep(Duration::from_millis(150));
    }
}

fn go_binary_path(app: &tauri::AppHandle) -> std::path::PathBuf {
    let name = if cfg!(target_os = "windows") {
        "mpc_editor.exe"
    } else {
        "mpc_editor"
    };
    if tauri::is_dev() {
        // CARGO_MANIFEST_DIR = tauri/src-tauri; go up twice to reach project root
        std::path::PathBuf::from(env!("CARGO_MANIFEST_DIR"))
            .parent()
            .unwrap()
            .parent()
            .unwrap()
            .join(name)
    } else {
        app.path().resource_dir().unwrap().join(name)
    }
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .setup(|app| {
            let port = find_free_port();
            let bin = go_binary_path(app.handle());

            let child = Command::new(&bin)
                .env("PORT", port.to_string())
                .env("NO_BROWSER", "1")
                .spawn()
                .map_err(|e| format!("Failed to start {}: {e}", bin.display()))?;

            GO_PROCESS.set(Mutex::new(child)).ok();

            if !wait_for_port(port, Duration::from_secs(15)) {
                if let Some(m) = GO_PROCESS.get() {
                    let _ = m.lock().unwrap().kill();
                }
                return Err("mpc_editor server did not start in time".into());
            }

            let url: url::Url = format!("http://127.0.0.1:{port}").parse().unwrap();
            WebviewWindowBuilder::new(app, "main", WebviewUrl::External(url))
                .title("MPC Editor")
                .inner_size(1400.0, 900.0)
                .min_inner_size(900.0, 600.0)
                .build()?;

            Ok(())
        })
        .build(tauri::generate_context!())
        .expect("error building tauri app")
        .run(|_app, event| {
            if let tauri::RunEvent::Exit = event {
                if let Some(m) = GO_PROCESS.get() {
                    if let Ok(mut child) = m.lock() {
                        let _ = child.kill();
                    }
                }
            }
        });
}
