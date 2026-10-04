// Test-only OS spawn boundary. execute(), JSONL parsing, AbortSignal, argument
// building and wait logic all run from the unmodified, installed SDK package.
// AMP_CLI_PATH is NOT sufficient: this SDK prefers its local CLI dependency.
import childProcess from 'node:child_process';
import { createRequire, syncBuiltinESMExports } from 'node:module';
import { appendFileSync, readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const packagePath = require.resolve('@ampcode/cli/package.json');
const cli = JSON.parse(readFileSync(packagePath));
const pinnedBin = resolve(dirname(packagePath), cli.bin.amp);
const spawn = childProcess.spawn;
const fake = fileURLToPath(new URL('./fake-cli.mjs', import.meta.url));
childProcess.spawn = (command, args, options) => {
  if (command !== pinnedBin) throw new Error('test blocked unexpected child process');
  appendFileSync(resolve(options.cwd, 'spawn.jsonl'), JSON.stringify({
    command, args, cwd: options.cwd, sdkVersion: options.env.AMP_SDK_VERSION,
    hasAbortSignal: options.signal instanceof AbortSignal, helperPid: process.pid,
  }) + '\n');
  const env = { ...options.env, NODE_OPTIONS: '' };
  // No inherited credentials are needed by the fake or its tool writer.
  for (const key of Object.keys(env)) if (/TOKEN|SECRET|KEY|PASSWORD/i.test(key)) delete env[key];
  const child = spawn(process.execPath, [fake, ...args], { ...options, env });
  child.on('exit', (code, signal) => {
    appendFileSync(resolve(options.cwd, 'observed-exit.jsonl'), JSON.stringify({ code, signal }) + '\n');
    // Expose AbortSignal's boundary before normal Node-exit group cleanup. This
    // holds only the test helper's event loop; SDK and child signals are intact.
    if (process.env.SPIKE_SCENARIO === 'abort_boundary') setTimeout(() => {}, 400);
  });
  return child;
};
childProcess.spawnSync = () => { throw new Error('test blocked synchronous subprocess'); };
syncBuiltinESMExports();
