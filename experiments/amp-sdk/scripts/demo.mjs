// Safe offline fixture demo: the real SDK is loaded, but its CLI spawn is always
// redirected by the test-only preload. No model, account or book is required.
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawn } from 'node:child_process';
import { once } from 'node:events';

const root = fileURLToPath(new URL('..', import.meta.url));
const cwd = mkdtempSync(join(tmpdir(), 'kepub-sdk-demo-'));
try {
  const child = spawn(resolve(root, 'dist/supervisor'), [
    '-node', process.execPath, '-helper', resolve(root, 'dist/helper.js'), '-timeout', '1800ms',
  ], {
    cwd: root,
    env: { ...process.env, NODE_OPTIONS: `--import=${resolve(root, 'test/redirect-cli.mjs')}`, SPIKE_SCENARIO: process.argv[2] ?? 'stream_input' },
    stdio: ['pipe', 'inherit', 'inherit'],
  });
  const closed = once(child, 'close');
  child.stdin.end(JSON.stringify({
    schemaVersion: 1, type: 'start', requestId: 'demo-1', workspaceId: 'ws-1', taskId: 'task-1',
    baseRevision: 'rev-1', cwd, prompt: 'Synthetic fixture only. Do not read a book.',
  }) + '\n');
  const [code] = await closed;
  process.exitCode = code ?? 1;
} finally {
  rmSync(cwd, { recursive: true, force: true });
}
