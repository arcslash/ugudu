#!/usr/bin/env node

/**
 * Postinstall script for @arcslash/ugudu
 * Downloads the correct binary for the current platform
 */

const https = require('https');
const http = require('http');
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const os = require('os');

const VERSION = require('../package.json').version;
const REPO = 'arcslash/ugudu';

// Map Node.js platform/arch to Go GOOS/GOARCH
const PLATFORM_MAP = {
  darwin: 'darwin',
  linux: 'linux',
  win32: 'windows',
};

const ARCH_MAP = {
  x64: 'amd64',
  arm64: 'arm64',
};

function getPlatform() {
  const platform = PLATFORM_MAP[os.platform()];
  const arch = ARCH_MAP[os.arch()];

  if (!platform || !arch) {
    throw new Error(`Unsupported platform: ${os.platform()}-${os.arch()}`);
  }

  return { platform, arch };
}

function getBinaryName(platform) {
  return platform === 'windows' ? 'ugudu.exe' : 'ugudu';
}

function getDownloadUrl(platform, arch, version) {
  const ext = platform === 'windows' ? '.zip' : '.tar.gz';
  const binaryName = `ugudu_${version}_${platform}_${arch}${ext}`;
  return `https://github.com/${REPO}/releases/download/v${version}/${binaryName}`;
}

function download(url) {
  return new Promise((resolve, reject) => {
    const client = url.startsWith('https') ? https : http;

    client.get(url, (response) => {
      // Handle redirects
      if (response.statusCode >= 300 && response.statusCode < 400 && response.headers.location) {
        return download(response.headers.location).then(resolve).catch(reject);
      }

      if (response.statusCode !== 200) {
        reject(new Error(`Download failed: ${response.statusCode}`));
        return;
      }

      const chunks = [];
      response.on('data', (chunk) => chunks.push(chunk));
      response.on('end', () => resolve(Buffer.concat(chunks)));
      response.on('error', reject);
    }).on('error', reject);
  });
}

async function extract(buffer, platform, binDir) {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ugudu-'));
  const archivePath = path.join(tmpDir, platform === 'windows' ? 'ugudu.zip' : 'ugudu.tar.gz');

  fs.writeFileSync(archivePath, buffer);

  try {
    if (platform === 'windows') {
      // Use PowerShell to extract zip
      execSync(`powershell -Command "Expand-Archive -Path '${archivePath}' -DestinationPath '${tmpDir}'"`, { stdio: 'ignore' });
    } else {
      // Use tar to extract
      execSync(`tar -xzf "${archivePath}" -C "${tmpDir}"`, { stdio: 'ignore' });
    }

    // Find and copy the binary
    const binaryName = getBinaryName(platform);
    const extractedBinary = path.join(tmpDir, binaryName);
    const targetBinary = path.join(binDir, binaryName);

    if (!fs.existsSync(extractedBinary)) {
      // Binary might be in a subdirectory
      const files = fs.readdirSync(tmpDir, { recursive: true });
      const binaryFile = files.find(f => f.endsWith(binaryName));
      if (binaryFile) {
        fs.copyFileSync(path.join(tmpDir, binaryFile), targetBinary);
      } else {
        throw new Error('Binary not found in archive');
      }
    } else {
      fs.copyFileSync(extractedBinary, targetBinary);
    }

    // Make executable on Unix
    if (platform !== 'windows') {
      fs.chmodSync(targetBinary, 0o755);
    }

    return targetBinary;
  } finally {
    // Cleanup
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
}

async function installFromGithub() {
  const { platform, arch } = getPlatform();
  const binDir = path.join(__dirname, '..', 'bin');

  // Create bin directory
  fs.mkdirSync(binDir, { recursive: true });

  const url = getDownloadUrl(platform, arch, VERSION);
  console.log(`Downloading Ugudu ${VERSION} for ${platform}-${arch}...`);
  console.log(`URL: ${url}`);

  try {
    const buffer = await download(url);
    const binaryPath = await extract(buffer, platform, binDir);
    console.log(`Installed to: ${binaryPath}`);
    console.log('Ugudu installed successfully!');
  } catch (err) {
    console.error(`Failed to download from GitHub: ${err.message}`);
    console.error('\nYou can install manually:');
    console.error('  1. Download from: https://github.com/arcslash/ugudu/releases');
    console.error('  2. Extract and add to your PATH');
    process.exit(1);
  }
}

async function main() {
  // Check if binary already exists (e.g., installed via Go)
  const binDir = path.join(__dirname, '..', 'bin');
  const { platform } = getPlatform();
  const binaryPath = path.join(binDir, getBinaryName(platform));

  if (fs.existsSync(binaryPath)) {
    console.log('Ugudu binary already exists, skipping download.');
    return;
  }

  // Check if installed globally via Go
  try {
    execSync('which ugudu', { stdio: 'ignore' });
    console.log('Ugudu is already installed globally.');

    // Create symlink in bin directory
    fs.mkdirSync(binDir, { recursive: true });
    const globalBinary = execSync('which ugudu', { encoding: 'utf8' }).trim();
    fs.symlinkSync(globalBinary, binaryPath);
    return;
  } catch {
    // Not installed globally, continue with download
  }

  await installFromGithub();
}

main().catch((err) => {
  console.error('Installation failed:', err);
  process.exit(1);
});
