#!/usr/bin/env node

import { spawn } from 'node:child_process';
import { createHash, randomUUID } from 'node:crypto';
import { open, readFile, stat, unlink, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(scriptDir, '..');
const configPath = path.join(rootDir, 'wails.json');
const lockName = createHash('sha256').update(rootDir).digest('hex');
const lockPath = path.join(tmpdir(), `foghorn-wails-version-${lockName}.lock`);
const lockToken = randomUUID();

const wait = (milliseconds) => new Promise((resolve) => setTimeout(resolve, milliseconds));

async function lockOwnerIsRunning() {
  try {
    const owner = JSON.parse(await readFile(lockPath, 'utf8'));
    if (!Number.isInteger(owner.pid) || owner.pid <= 0) return true;
    process.kill(owner.pid, 0);
    return true;
  } catch (error) {
    if (error?.code === 'ENOENT' || error?.code === 'ESRCH') return false;
    if (error instanceof SyntaxError) {
      try {
        return Date.now() - (await stat(lockPath)).mtimeMs < 5000;
      } catch (statError) {
        return statError?.code !== 'ENOENT';
      }
    }
    return true;
  }
}

async function acquireConfigLock() {
  const deadline = Date.now() + 5 * 60 * 1000;

  while (true) {
    try {
      const handle = await open(lockPath, 'wx');
      try {
        await handle.writeFile(JSON.stringify({ pid: process.pid, token: lockToken }));
      } catch (error) {
        await handle.close();
        await unlink(lockPath).catch(() => {});
        throw error;
      }
      return handle;
    } catch (error) {
      if (error?.code !== 'EEXIST') throw error;

      if (!(await lockOwnerIsRunning())) {
        await unlink(lockPath).catch((unlinkError) => {
          if (unlinkError?.code !== 'ENOENT') throw unlinkError;
        });
        continue;
      }

      if (Date.now() >= deadline) {
        throw new Error(`Timed out waiting for another versioned Wails build: ${lockPath}`);
      }
      await wait(200);
    }
  }
}

async function releaseConfigLock(handle) {
  await handle.close();
  try {
    const owner = JSON.parse(await readFile(lockPath, 'utf8'));
    if (owner.token === lockToken) await unlink(lockPath);
  } catch (error) {
    if (error?.code !== 'ENOENT') throw error;
  }
}

function productVersion(raw) {
  const match = String(raw).match(/^(\d+)\.(\d+)\.(\d+)/);
  return match ? `${match[1]}.${match[2]}.${match[3]}` : '0.0.0';
}

function usage() {
  console.error('Usage: run-versioned-wails-build.mjs [--print] <version> [-- <command> <args...>]');
}

const args = process.argv.slice(2);
const printOnly = args[0] === '--print';
const rawVersion = args[printOnly ? 1 : 0];

if (!rawVersion) {
  usage();
  process.exit(2);
}

const resolvedProductVersion = productVersion(rawVersion);
if (printOnly) {
  process.stdout.write(`${resolvedProductVersion}\n`);
  process.exit(0);
}

const separator = args.indexOf('--', 1);
const commandArgs = separator >= 0 ? args.slice(separator + 1) : args.slice(1);
if (commandArgs.length === 0) {
  usage();
  process.exit(2);
}

const lockHandle = await acquireConfigLock();
let originalConfig;
let temporaryConfig;
let exitCode = 1;
try {
  originalConfig = await readFile(configPath, 'utf8');
  const config = JSON.parse(originalConfig);
  config.info ??= {};
  config.info.productVersion = resolvedProductVersion;
  temporaryConfig = `${JSON.stringify(config, null, 2)}\n`;

  await writeFile(configPath, temporaryConfig);
  console.log(`Wails product version: ${resolvedProductVersion}`);

  const [command, ...commandArguments] = commandArgs;
  const child = spawn(command, commandArguments, {
    cwd: rootDir,
    env: process.env,
    stdio: 'inherit',
    shell: false,
  });

  for (const signal of ['SIGINT', 'SIGTERM']) {
    process.on(signal, () => child.kill(signal));
  }

  exitCode = await new Promise((resolve, reject) => {
    child.once('error', reject);
    child.once('exit', (code, signal) => {
      if (signal) resolve(128 + (signal === 'SIGINT' ? 2 : 15));
      else resolve(code ?? 1);
    });
  });
} finally {
  try {
    if (temporaryConfig) {
      const currentConfig = await readFile(configPath, 'utf8');
      if (currentConfig === temporaryConfig) {
        await writeFile(configPath, originalConfig);
      } else {
        console.error('wails.json changed during the build; leaving the newer contents untouched.');
      }
    }
  } finally {
    await releaseConfigLock(lockHandle);
  }
}

process.exitCode = exitCode;
