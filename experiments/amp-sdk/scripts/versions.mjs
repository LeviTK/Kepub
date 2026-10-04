import { createRequire } from 'node:module';
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { dirname, resolve } from 'node:path';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import assert from 'node:assert/strict';

const require = createRequire(import.meta.url);
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const expected = JSON.parse(readFileSync(resolve(root, 'package.json')));
assert.equal(process.version, `v${expected.engines.node}`);
const sdk = JSON.parse(readFileSync(resolve(root, 'node_modules/@ampcode/sdk/package.json')));
const cliPath = require.resolve('@ampcode/cli/package.json');
const cli = JSON.parse(readFileSync(cliPath));
assert.equal(sdk.version, expected.dependencies['@ampcode/sdk']);
assert.equal(cli.version, expected.dependencies['@ampcode/cli']);
const fromSDK = createRequire(import.meta.resolve('@ampcode/sdk'));
assert.equal(fromSDK.resolve('@ampcode/cli/package.json'), cliPath, 'SDK must resolve locked CLI');
const bin = resolve(dirname(cliPath), cli.bin.amp);
// Version/help only; never execute a prompt, authenticate, or create a thread.
const version = execFileSync(bin, ['--version'], { encoding: 'utf8', timeout: 10000 }).trim();
assert.ok(version.includes(cli.version));
await import('@ampcode/sdk');
console.log(JSON.stringify({
  node: process.version, platform: `${process.platform}/${process.arch}`, sdk: sdk.version,
  cliPackage: cli.version, cliBinaryVersion: version,
  cliSHA256: createHash('sha256').update(readFileSync(bin)).digest('hex'),
  sdkLoaded: true, sdkResolvesPinnedCLI: true,
}, null, 2));
