// Private IPC peer, never a WebView import. Go owns the public envelope and terminal.
import { execute, createUserMessage } from '@ampcode/sdk';
import { once } from 'node:events';
import { accessSync, constants, readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const maxMessage = 64 * 1024;
type Start = {
  type: 'start'; requestId: string; cwd: string; prompt: string; threadId?: string;
};
const controller = new AbortController();
let active: Start | undefined;
let done = false;
delete process.env.AMP_DEBUG;

async function send(type: string, data: Record<string, unknown>): Promise<void> {
  const line = JSON.stringify({ type, data }) + '\n';
  if (Buffer.byteLength(line) > maxMessage) throw new Error('MESSAGE_LIMIT');
  if (!process.stdout.write(line)) await once(process.stdout, 'drain');
}

async function run(start: Start): Promise<void> {
  let result: string | undefined;
  let session: string | undefined;
  try {
    // Refuse SDK fallback to AMP_CLI_PATH/PATH if the locked local CLI is absent.
    const expected = JSON.parse(readFileSync(new URL('../package.json', import.meta.url), 'utf8'));
    const sdkURL = new URL(import.meta.resolve('@ampcode/sdk'));
    const sdk = JSON.parse(readFileSync(new URL('../package.json', sdkURL), 'utf8'));
    const cliPath = createRequire(sdkURL).resolve('@ampcode/cli/package.json');
    const cli = JSON.parse(readFileSync(cliPath, 'utf8'));
    if (process.version !== `v${expected.engines.node}` || sdk.version !== expected.dependencies['@ampcode/sdk'] || cli.version !== expected.dependencies['@ampcode/cli']) {
      throw new Error('RUNTIME_MISMATCH');
    }
    accessSync(resolve(dirname(cliPath), cli.bin.amp), constants.X_OK);
    // Streaming input is real SDK serialization, including request_id. It is not
    // publication-transaction deduplication and does not create an interactive pool.
    async function* prompt() {
      yield createUserMessage(start.prompt, { requestId: start.requestId });
    }
    for await (const message of execute({
      prompt: prompt(),
      signal: controller.signal,
      options: {
        cwd: start.cwd,
        visibility: 'private',
        executor: 'local',
        mode: 'ultra',
        settingsFile: fileURLToPath(new URL('../runtime-settings.json', import.meta.url)),
        ...(start.threadId ? { continue: start.threadId } : {}),
      },
    })) {
      // The SDK does not enforce its StreamMessage types at runtime or bound its
      // raw JSONL reader. This is a post-parse bound, NOT a raw transport bound.
      if (Buffer.byteLength(JSON.stringify(message)) > maxMessage) throw new Error('MESSAGE_LIMIT');
      if (!message || typeof message !== 'object') throw new Error('INVALID_MESSAGE');
      if (result !== undefined) throw new Error('AFTER_RESULT');
      if (typeof message.session_id !== 'string') throw new Error('INVALID_MESSAGE');
      if (session && session !== message.session_id) throw new Error('SESSION_MISMATCH');
      session = message.session_id;
      if (start.threadId && session !== start.threadId) throw new Error('SESSION_MISMATCH');
      if (message.type === 'assistant') {
        if (!Array.isArray(message.message?.content)) throw new Error('INVALID_MESSAGE');
        for (const block of message.message.content) {
          if (block.type === 'text' && typeof block.text === 'string') {
            await send('assistant', { text: block.text, threadId: session });
          } else if (block.type === 'tool_use') {
            await send('tool', { phase: 'use', id: block.id, name: block.name, input: block.input });
          }
        }
      } else if (message.type === 'user') {
        if (!Array.isArray(message.message?.content)) throw new Error('INVALID_MESSAGE');
        for (const block of message.message.content) {
          if (block.type === 'tool_result') await send('tool', { phase: 'result', ...block });
        }
      } else if (message.type === 'result') {
        if (message.is_error !== false || message.subtype !== 'success' || typeof message.result !== 'string') {
          throw new Error('RESULT_ERROR');
        }
        result = message.result;
      }
      // Unknown message types/fields are tolerated, but cannot establish success.
    }
    if (controller.signal.aborted) throw new Error('ABORTED');
    if (result === undefined) throw new Error('MISSING_RESULT');
    await send('sdk_end', {
      result, threadId: session,
      // Source-audited SDK waitForProcess/throwIfProcessFailed path, not a raw
      // OS wait status or evidence that any descendants have exited.
      cliExitEvidence: 'sdk-iterator-validated-zero',
    });
  } catch (error) {
    const known = new Set(['RUNTIME_MISMATCH', 'MESSAGE_LIMIT', 'INVALID_MESSAGE', 'AFTER_RESULT', 'SESSION_MISMATCH', 'RESULT_ERROR', 'ABORTED', 'MISSING_RESULT']);
    const code = error instanceof Error && known.has(error.message) ? error.message : 'SDK_EXECUTION';
    // SDK errors may contain raw CLI output, book content or credentials. Do not
    // serialize those strings as diagnostics.
    await send('sdk_error', { code, aborted: controller.signal.aborted });
    process.exitCode = 1;
  } finally {
    done = true;
    process.stdin.destroy();
  }
}

let pending = Buffer.alloc(0);
process.stdin.on('data', (chunk: Buffer) => {
  if (done) return;
  pending = Buffer.concat([pending, chunk]);
  let end: number;
  while ((end = pending.indexOf(10)) >= 0) {
    const line = pending.subarray(0, end);
    pending = pending.subarray(end + 1);
    if (line.length > maxMessage) { controller.abort(); process.exitCode = 1; process.stdin.destroy(); return; }
    try {
      const message = JSON.parse(line.toString('utf8'));
      if (message.type === 'start' && !active) {
        active = message;
        void run(message);
      } else if (message.type === 'cancel' && message.requestId === active?.requestId) {
        controller.abort();
      } else {
        throw new Error('invalid control');
      }
    } catch {
      controller.abort();
      process.exitCode = 1;
      process.stdin.destroy();
      return;
    }
  }
  if (pending.length > maxMessage) { controller.abort(); process.exitCode = 1; process.stdin.destroy(); }
});
process.stdin.on('end', () => { if (!done) controller.abort(); });
