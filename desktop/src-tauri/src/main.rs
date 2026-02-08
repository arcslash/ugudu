// Prevents additional console window on Windows in release
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::process::Command;
use std::time::Duration;
use tauri::Manager;

// Check if the daemon is running by attempting to connect
async fn check_daemon() -> bool {
    match reqwest::get("http://localhost:8080/api/health").await {
        Ok(resp) => resp.status().is_success(),
        Err(_) => false,
    }
}

// Start the daemon process
fn start_daemon() -> Result<(), String> {
    // Try to find ugudu binary
    let binary = if cfg!(target_os = "windows") {
        "ugudu.exe"
    } else {
        "ugudu"
    };

    // Try common locations
    let paths = vec![
        format!("/usr/local/bin/{}", binary),
        format!("{}/.local/bin/{}", std::env::var("HOME").unwrap_or_default(), binary),
        format!("{}/go/bin/{}", std::env::var("HOME").unwrap_or_default(), binary),
        binary.to_string(),
    ];

    for path in paths {
        if let Ok(_) = Command::new(&path)
            .arg("daemon")
            .spawn()
        {
            return Ok(());
        }
    }

    Err("Could not find or start ugudu daemon".to_string())
}

#[tauri::command]
async fn get_daemon_status() -> Result<String, String> {
    if check_daemon().await {
        Ok("running".to_string())
    } else {
        Err("Daemon is not running".to_string())
    }
}

#[tauri::command]
async fn ensure_daemon() -> Result<String, String> {
    if check_daemon().await {
        return Ok("already_running".to_string());
    }

    // Try to start the daemon
    start_daemon()?;

    // Wait for daemon to be ready
    for _ in 0..30 {
        tokio::time::sleep(Duration::from_millis(500)).await;
        if check_daemon().await {
            return Ok("started".to_string());
        }
    }

    Err("Daemon failed to start".to_string())
}

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .invoke_handler(tauri::generate_handler![get_daemon_status, ensure_daemon])
        .setup(|app| {
            // Get the main window
            let window = app.get_webview_window("main").unwrap();

            // Spawn a task to ensure daemon is running
            tauri::async_runtime::spawn(async move {
                if !check_daemon().await {
                    let _ = start_daemon();
                    // Wait a bit for daemon to start
                    tokio::time::sleep(Duration::from_secs(2)).await;
                }
            });

            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
