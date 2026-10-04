import { writeFileSync, appendFileSync } from 'node:fs';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { setTimeout as delay } from 'node:timers/promises';
import { fileURLToPath } from 'node:url';

const scenario = process.env.SPIKE_SCENARIO;
const thread = process.argv.includes('continue') ? process.argv[process.argv.indexOf('continue') + 1] : 'T-11111111-2222-4333-8444-555555555555';
writeFileSync('cli.pid', `${process.pid}`);
let input = '';
for await (const chunk of process.stdin) input += chunk;
writeFileSync('input.jsonl', input);

async function output(message, chunked = false) {
  const bytes = Buffer.from(JSON.stringify({ session_id: thread, ...message }) + '\n');
  if (chunked) {
    // Split each UTF-8 code point across separate writes, including astral text.
    for (const byte of bytes) { process.stdout.write(Buffer.from([byte])); await delay(1); }
  } else if (!process.stdout.write(bytes)) await once(process.stdout, 'drain');
}

function writer() {
  const child = spawn(process.execPath, [fileURLToPath(new URL('./writer.mjs', import.meta.url))], { stdio: 'ignore' });
  child.unref();
}

await output({ type: 'system', subtype: 'init', cwd: process.cwd(), tools: [], mcp_servers: [] });
if (['cancel_after', 'repeat_cancel', 'delayed_writer', 'success_writer', 'abort_boundary', 'epipe_writer'].includes(scenario)) {
  writer();
}
if (['cancel_before', 'cancel_after', 'repeat_cancel', 'delayed_writer', 'abort_boundary', 'epipe_writer'].includes(scenario)) {
  if (['delayed_writer', 'epipe_writer'].includes(scenario)) process.on('SIGTERM', () => appendFileSync('cli-term', 'TERM\n'));
  await output({ type: 'assistant', message: { content: [{ type: 'text', text: 'ready' }] } });
  if (scenario === 'epipe_writer') {
    setInterval(() => { void output({ type: 'assistant', message: { content: [{ type: 'text', text: 'still writing' }] } }); }, 25);
  } else {
    setInterval(() => {}, 1000);
  }
} else if (scenario === 'unterminated_oversize') {
  if (!process.stdout.write('x'.repeat(1024 * 1024))) await once(process.stdout, 'drain');
  writeFileSync('oversize-sent', 'yes');
  setInterval(() => {}, 1000);
} else {
  if (scenario === 'stderr_parallel' || scenario === 'stderr_190000') {
    writeFileSync('stderr-started', 'yes');
    let remaining = scenario === 'stderr_190000' ? 190000 : 2883584;
    while (remaining > 0) {
      const size = Math.min(remaining, 8192);
      if (!process.stderr.write('d'.repeat(size))) await once(process.stderr, 'drain');
      remaining -= size;
    }
    writeFileSync('stderr-finished', 'yes');
  } else {
    process.stderr.write('small diagnostic\n');
  }
  await output({
    type: 'assistant', future_field: 'ignored',
    message: { content: [{ type: 'text', text: scenario === 'message_limit' ? 'x'.repeat(70000) : '中文🙂 café' },
      { type: 'tool_use', id: 'tool-7', name: 'fixture_read', input: { bookPath: 'Text/章.xhtml' } }] },
  }, scenario === 'chunked_utf8');
  if (scenario === 'unknown_fields') await output({ type: 'future_progress', extra: 123 });
  if (scenario === 'truncated_json') {
    process.stdout.write('{"type":"res');
  } else if (scenario !== 'no_result') {
    await output({ type: 'result', subtype: 'success', is_error: false, result: 'fixture only', duration_ms: 3, num_turns: 1 });
    writeFileSync('result-sent', 'yes');
  }
  if (scenario === 'exit_before_eof') {
    // A valid Unix child keeps stdout open beyond the CLI's nonzero exit. The
    // SDK must not attach its exit listener only after the child's EOF.
    const holder = spawn(process.execPath, ['-e', 'setTimeout(() => {}, 250)'], { stdio: ['ignore', 'inherit', 'ignore'] });
    holder.unref();
    process.exit(23);
  }
  process.stdout.end();
  // Make the ordinary success/nonzero cases deterministic; the dedicated race
  // scenario above explicitly tests exit preceding EOF rather than hiding it.
  await delay(scenario === 'cancel_after_result' ? 1000 : 80);
  process.exit(scenario === 'success_nonzero' ? 23 : 0);
}
