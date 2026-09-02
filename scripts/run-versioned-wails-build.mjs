#!/usr/bin/env node

import { spawn } from 'node:child_process';
import { readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(scriptDir, '..');
const configPath = path.join(rootDir, 'wails.json');

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

const originalConfig = await readFile(configPath, 'utf8');
const config = JSON.parse(originalConfig);
config.info ??= {};
config.info.productVersion = resolvedProductVersion;

let restored = false;
async function restoreConfig() {
  if (restored) return;
  restored = true;
  await writeFile(configPath, originalConfig);
}

await writeFile(configPath, `${JSON.stringify(config, null, 2)}\n`);
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

let exitCode = 1;
try {
  exitCode = await new Promise((resolve, reject) => {
    child.once('error', reject);
    child.once('exit', (code, signal) => {
      if (signal) resolve(128 + (signal === 'SIGINT' ? 2 : 15));
      else resolve(code ?? 1);
    });
  });
} finally {
  await restoreConfig();
}

process.exitCode = exitCode;
