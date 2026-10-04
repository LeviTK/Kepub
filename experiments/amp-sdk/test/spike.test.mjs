import test from 'node:test';
import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, writeFileSync, existsSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { resolve, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';

const root = fileURLToPath(new URL('..', import.meta.url));
const threadId = 'T-11111111-2222-4333-8444-555555555555';
const fixturePrompt = '只读临时 fixture；literal $(touch injected) --dangerously-allow-all\n中文🙂';

async function until(predicate, timeout = 3000) {
  const deadline = Date.now() + timeout;
  while (!predicate()) {
    if (Date.now() > deadline) throw new Error('fixture readiness timeout');
    await delay(10);
  }
}

function launch(t, scenario, changes = {}, bindingChange, extraArgs = []) {
  const cwd = mkdtempSync(join(tmpdir(), 'kepub-sdk-'));
  writeFileSync(join(cwd, 'book-sentinel.txt'), 'never uploaded');
  const start = { schemaVersion: 1, type: 'start', requestId: 'sdk-spike-9', workspaceId: 'ws-2', taskId: 'task-3',
    baseRevision: 'rev-5', cwd, prompt: fixturePrompt, generation: 7,
    selectedText: 'not implicitly added to prompt', bookPath: 'Text/章.xhtml', ...changes };
  const args = ['-node', process.execPath, '-helper', resolve(root, 'dist/helper.js'), '-timeout', '1800ms', ...extraArgs];
  if (bindingChange) {
    const binding = { threadId, workspaceId: start.workspaceId, taskId: start.taskId, cwd, baseRevision: start.baseRevision, ...bindingChange };
    writeFileSync(join(cwd, 'bindings.json'), JSON.stringify({ schemaVersion: 1, threads: [binding] }));
    args.push('--bindings', join(cwd, 'bindings.json'));
  }
  const env = { ...process.env, NODE_OPTIONS: `--import=${resolve(root, 'test/redirect-cli.mjs')}`, SPIKE_SCENARIO: scenario };
  const proc = spawn(resolve(root, 'dist/supervisor'), args, { cwd: root, env, stdio: ['pipe', 'pipe', 'pipe'] });
  proc.stdin.on('error', () => {});
  let stdout = '', stderr = '';
  proc.stdout.setEncoding('utf8').on('data', data => { stdout += data; });
  proc.stderr.setEncoding('utf8').on('data', data => { stderr += data; });
  const closed = new Promise((resolve, reject) => {
    proc.once('error', reject);
    proc.once('close', (code, signal) => resolve({ code, signal }));
  });
  proc.stdin.write(JSON.stringify(start) + '\n');
  const cancel = () => proc.stdin.write(JSON.stringify({ schemaVersion: 1, type: 'cancel', requestId: start.requestId }) + '\n');
  t.after(async () => {
    if (proc.exitCode === null && !proc.stdin.writableEnded) { cancel(); proc.stdin.end(); }
    await closed;
    rmSync(cwd, { recursive: true, force: true });
  });
  const file = name => join(cwd, name);
  async function finish(type, code) {
    proc.stdin.end();
    const status = await closed;
    const events = stdout.trim().split('\n').map(line => JSON.parse(line));
    const terminals = events.filter(e => ['completed', 'failed', 'cancelled'].includes(e.type));
    assert.equal(terminals.length, 1, stdout + stderr);
    const terminal = terminals[0];
    assert.equal(events.at(-1), terminal);
    assert.equal(terminal.type, type, stdout + stderr);
    assert.equal(terminal.data.code, code, stdout + stderr);
    assert.equal(terminal.data.reviewRequired, type === 'completed');
    assert.deepEqual(terminal.data.cleanup, { scope: 'process-group', confirmed: true });
    assert.equal(status.code, type === 'completed' ? 0 : 1);
    assert.equal(status.signal, null);
    for (const [index, event] of events.entries()) {
      assert.equal(event.sequence, index + 1);
      for (const key of ['requestId', 'workspaceId', 'taskId']) assert.equal(event[key], start[key]);
      assert.equal(event.schemaVersion, 1);
      assert.equal(event.generation, start.generation ?? null);
    }
    // Must be absent, not merely zombies or a stopped heartbeat at one instant.
    for (const name of ['cli.pid', 'writer.pid']) if (existsSync(file(name))) {
      const pid = Number(readFileSync(file(name), 'utf8'));
      assert.throws(() => process.kill(pid, 0), { code: 'ESRCH' }, `unreaped ${name}: ${pid}`);
    }
    if (existsSync(file('spawn.jsonl'))) {
      const { helperPid } = JSON.parse(readFileSync(file('spawn.jsonl'), 'utf8'));
      assert.throws(() => process.kill(helperPid, 0), { code: 'ESRCH' }, 'unreaped helper');
    }
    assert.equal(readFileSync(file('book-sentinel.txt'), 'utf8'), 'never uploaded');
    assert.equal(existsSync(file('injected')), false);
    return { events, terminal, stderr };
  }
  return { start, cwd, file, cancel, finish, proc };
}

test('fixed SDK loads and resolves the pinned real CLI (version only)', () => {
  const output = execFileSync(process.execPath, ['scripts/versions.mjs'], { cwd: root, encoding: 'utf8' });
  assert.equal(JSON.parse(output).sdkResolvesPinnedCLI, true);
});

test('blocked helper stdin: Go deadline does not wait for startup pipe drain', async t => {
  const run = launch(t, 'blocked_helper', { prompt: 'x'.repeat(60000) }, undefined,
    ['-helper', resolve(root, 'test/blocked-helper.mjs')]);
  await run.finish('failed', 'TIMEOUT');
});

for (const scenario of ['stream_input', 'chunked_utf8', 'unknown_fields']) {
  test(scenario, async t => {
    const run = launch(t, scenario, scenario === 'unknown_fields' ? { generation: undefined } : {});
    const { events, terminal } = await run.finish('completed', null);
    assert.equal(events.find(e => e.type === 'assistant').data.text, '中文🙂 café');
    assert.equal(events.find(e => e.type === 'tool').data.name, 'fixture_read');
    assert.equal(terminal.data.threadId, threadId);
    assert.equal(terminal.data.helperExitCode, 0);
    assert.equal(terminal.data.cliExitEvidence, 'sdk-iterator-validated-zero');
    const mapped = JSON.parse(readFileSync(run.file('spawn.jsonl'), 'utf8'));
    assert.deepEqual(mapped.args, ['--execute', '--stream-json', '--visibility', 'private', '--settings-file', resolve(root, 'runtime-settings.json'), '--mode', 'ultra', '--stream-json-input']);
    assert.deepEqual(JSON.parse(readFileSync(mapped.args[5], 'utf8')), {
      'amp.updates.mode': 'disabled', 'amp.runner.autoUpdate.enabled': false, 'amp.dangerouslyAllowAll': false,
    });
    assert.equal(mapped.cwd, run.cwd);
    assert.equal(mapped.hasAbortSignal, true);
    assert.equal(mapped.sdkVersion, '0.1.0-20260918210405-g81edbf0');
    assert.deepEqual(JSON.parse(readFileSync(run.file('input.jsonl'), 'utf8')), {
      type: 'user', request_id: run.start.requestId, message: { role: 'user', content: [{ type: 'text', text: fixturePrompt }] },
    });
  });
}

for (const [scenario, code] of [
  ['message_limit', 'MESSAGE_LIMIT'], ['no_result', 'MISSING_RESULT'], ['success_nonzero', 'SDK_EXECUTION'],
  ['truncated_json', 'SDK_EXECUTION'], ['success_writer', 'WRITERS_AFTER_SDK'],
]) {
  test(scenario, async t => {
    const run = launch(t, scenario);
    await run.finish('failed', code);
    if (scenario === 'success_writer') {
      await delay(750);
      assert.equal(existsSync(run.file('late-write')), false);
    }
  });
}

test('stderr_parallel: real SDK backpressure gap is contained, NOT a passing transport gate', async t => {
  const run = launch(t, 'stderr_parallel');
  await run.finish('failed', 'TIMEOUT');
  assert.equal(existsSync(run.file('stderr-started')), true);
  assert.equal(existsSync(run.file('stderr-finished')), false);
});

test('unterminated_oversize: SDK raw line exceeds 64KiB; only timeout contains it', async t => {
  const run = launch(t, 'unterminated_oversize');
  await run.finish('failed', 'TIMEOUT');
  assert.equal(existsSync(run.file('oversize-sent')), true);
});

test('exit_before_eof: real SDK misses already-fired CLI exit (fails closed)', async t => {
  const run = launch(t, 'exit_before_eof');
  await run.finish('failed', 'TIMEOUT');
  assert.deepEqual(JSON.parse(readFileSync(run.file('observed-exit.jsonl'), 'utf8')), { code: 23, signal: null });
});

test('cancel_before: immediate start/cancel, no completed', async t => {
  const run = launch(t, 'cancel_before');
  run.cancel();
  await run.finish('cancelled', 'CANCELLED');
});

for (const scenario of ['cancel_after', 'repeat_cancel', 'delayed_writer']) {
  test(scenario, async t => {
    const run = launch(t, scenario);
    await until(() => existsSync(run.file('heartbeat')));
    run.cancel();
    if (scenario === 'repeat_cancel') { run.cancel(); run.cancel(); }
    await run.finish('cancelled', 'CANCELLED');
    const heartbeat = readFileSync(run.file('heartbeat'), 'utf8');
    await delay(750);
    assert.equal(readFileSync(run.file('heartbeat'), 'utf8'), heartbeat);
    assert.equal(existsSync(run.file('late-write')), false);
    assert.equal(existsSync(run.file('writer-term')), true);
    if (scenario === 'delayed_writer') assert.equal(existsSync(run.file('cli-term')), true);
  });
}

test('cancel_after_result: protocol success is not terminal before process exit', async t => {
  const run = launch(t, 'cancel_after_result');
  await until(() => existsSync(run.file('result-sent')));
  run.cancel();
  await run.finish('cancelled', 'CANCELLED');
});

test('AbortSignal boundary: CLI dies but writer continues until Go group escalation', async t => {
  const run = launch(t, 'abort_boundary', {}, undefined, ['-grace', '500ms']);
  await until(() => existsSync(run.file('heartbeat')));
  run.cancel();
  await until(() => existsSync(run.file('observed-exit.jsonl')));
  assert.deepEqual(JSON.parse(readFileSync(run.file('observed-exit.jsonl'), 'utf8')), { code: null, signal: 'SIGTERM' });
  const before = readFileSync(run.file('heartbeat'), 'utf8');
  await delay(100);
  assert.ok(readFileSync(run.file('heartbeat'), 'utf8').length > before.length, 'AbortSignal alone must be shown insufficient');
  assert.equal(existsSync(run.file('writer-term')), false, 'observation must precede Go TERM grace');
  await run.finish('cancelled', 'CANCELLED');
  const frozen = readFileSync(run.file('heartbeat'), 'utf8');
  await delay(100);
  assert.equal(readFileSync(run.file('heartbeat'), 'utf8'), frozen);
});

test('explicit threadId: trusted binding and real SDK CLI mapping', async t => {
  const run = launch(t, 'stream_input', { threadId }, {});
  await run.finish('completed', null);
  const { args } = JSON.parse(readFileSync(run.file('spawn.jsonl'), 'utf8'));
  assert.deepEqual(args.slice(0, 3), ['threads', 'continue', threadId]);
  assert.equal(args.includes('--dangerously-allow-all'), false);
  assert.equal(args.includes('--orb-execute'), false);
});

test('explicit threadId: missing binding rejects before SDK process', async t => {
  const run = launch(t, 'stream_input', { threadId });
  await run.finish('failed', 'BINDING_MISMATCH');
  assert.equal(existsSync(run.file('spawn.jsonl')), false);
});

for (const field of ['workspaceId', 'taskId', 'baseRevision', 'cwd']) {
  test(`explicit threadId: rejects ${field} mismatch`, async t => {
    const run = launch(t, 'stream_input', { threadId }, { [field]: 'wrong' });
    await run.finish('failed', 'BINDING_MISMATCH');
    assert.equal(existsSync(run.file('spawn.jsonl')), false);
  });
}
