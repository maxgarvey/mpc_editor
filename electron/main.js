'use strict';

const { app, BrowserWindow, shell, Menu } = require('electron');
const { spawn } = require('child_process');
const net = require('net');
const http = require('http');
const path = require('path');

let goProcess = null;
let serverUrl = null;
let mainWindow = null;

// ---------------------------------------------------------------------------
// Port + server helpers
// ---------------------------------------------------------------------------

function findFreePort() {
    return new Promise((resolve, reject) => {
        const srv = net.createServer();
        srv.listen(0, '127.0.0.1', () => {
            const port = srv.address().port;
            srv.close(() => resolve(port));
        });
        srv.on('error', reject);
    });
}

function waitForServer(url, timeoutMs = 15000) {
    return new Promise((resolve, reject) => {
        const deadline = Date.now() + timeoutMs;
        function attempt() {
            http.get(url, (res) => {
                res.resume(); // drain
                resolve();
            }).on('error', () => {
                if (Date.now() >= deadline) {
                    reject(new Error('Go server did not start within ' + timeoutMs + 'ms'));
                } else {
                    setTimeout(attempt, 150);
                }
            });
        }
        attempt();
    });
}

// ---------------------------------------------------------------------------
// Go binary
// ---------------------------------------------------------------------------

function goBinaryPath() {
    const name = process.platform === 'win32' ? 'mpc_editor.exe' : 'mpc_editor';
    if (app.isPackaged) {
        // electron-builder copies the binary into Resources via extraResources
        return path.join(process.resourcesPath, name);
    }
    // Development: binary is in the project root (one directory up from electron/)
    return path.join(__dirname, '..', name);
}

async function startGoServer() {
    const port = await findFreePort();
    const bin = goBinaryPath();

    goProcess = spawn(bin, [], {
        env: Object.assign({}, process.env, {
            PORT: String(port),
            NO_BROWSER: '1',
        }),
    });

    goProcess.stdout.on('data', (d) => process.stdout.write('[go] ' + d));
    goProcess.stderr.on('data', (d) => process.stderr.write('[go] ' + d));
    goProcess.on('exit', (code, sig) => {
        console.log(`[go] process exited  code=${code} signal=${sig}`);
        goProcess = null;
    });

    serverUrl = `http://127.0.0.1:${port}`;
    await waitForServer(serverUrl);
    return serverUrl;
}

function stopGoServer() {
    if (goProcess) {
        goProcess.kill();
        goProcess = null;
    }
}

// ---------------------------------------------------------------------------
// Window
// ---------------------------------------------------------------------------

function createWindow(url) {
    mainWindow = new BrowserWindow({
        width: 1400,
        height: 900,
        minWidth: 900,
        minHeight: 600,
        title: 'MPC Editor',
        // Use the app icon if it exists; electron-builder sets this automatically
        // for packaged builds via the build.icon field in package.json.
        webPreferences: {
            nodeIntegration: false,
            contextIsolation: true,
            // No preload needed — the app talks to a local HTTP server
        },
    });

    // Keep address bar out of sight; open external <a target="_blank"> links in
    // the system browser rather than inside the Electron window.
    mainWindow.webContents.setWindowOpenHandler(({ url: href }) => {
        if (href.startsWith('http://127.0.0.1') || href.startsWith('http://localhost')) {
            return { action: 'allow' };
        }
        shell.openExternal(href);
        return { action: 'deny' };
    });

    mainWindow.loadURL(url);

    mainWindow.on('closed', () => {
        mainWindow = null;
    });

    buildMenu();
}

// Minimal native menu — keeps Cmd+Q, Cmd+W, Cmd+R working.
function buildMenu() {
    const template = [
        {
            label: 'MPC Editor',
            submenu: [
                { label: 'About MPC Editor', role: 'about' },
                { type: 'separator' },
                { label: 'Hide MPC Editor', role: 'hide' },
                { label: 'Hide Others', role: 'hideOthers' },
                { label: 'Show All', role: 'unhide' },
                { type: 'separator' },
                { label: 'Quit', role: 'quit' },
            ],
        },
        {
            label: 'Edit',
            submenu: [
                { role: 'undo' }, { role: 'redo' }, { type: 'separator' },
                { role: 'cut' }, { role: 'copy' }, { role: 'paste' },
                { role: 'selectAll' },
            ],
        },
        {
            label: 'View',
            submenu: [
                { role: 'reload' },
                { role: 'forceReload' },
                { role: 'toggleDevTools' },
                { type: 'separator' },
                { role: 'resetZoom' },
                { role: 'zoomIn' },
                { role: 'zoomOut' },
                { type: 'separator' },
                { role: 'togglefullscreen' },
            ],
        },
        {
            label: 'Window',
            submenu: [
                { role: 'minimize' },
                { role: 'zoom' },
                { type: 'separator' },
                { role: 'front' },
            ],
        },
    ];

    Menu.setApplicationMenu(Menu.buildFromTemplate(template));
}

// ---------------------------------------------------------------------------
// App lifecycle
// ---------------------------------------------------------------------------

// Skip the GPU process entirely. This app renders plain HTML/CSS — hardware
// acceleration buys nothing and causes Chromium to spam EGL init errors on macOS.
app.disableHardwareAcceleration();

app.whenReady().then(async () => {
    try {
        const url = await startGoServer();
        createWindow(url);
    } catch (err) {
        console.error('Startup failed:', err);
        app.quit();
    }
});

// macOS: re-open window when clicking the dock icon with no windows open.
app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0 && serverUrl) {
        createWindow(serverUrl);
    }
});

// Quit when all windows are closed (except on macOS where the app stays in dock).
app.on('window-all-closed', () => {
    if (process.platform !== 'darwin') {
        app.quit();
    }
});

app.on('before-quit', () => {
    stopGoServer();
});
