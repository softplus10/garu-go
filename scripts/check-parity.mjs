// Usage: node scripts/check-parity.mjs <matching garu-ko package dir> <Go parity executable>
import assert from 'node:assert/strict';
import { pathToFileURL, fileURLToPath } from 'node:url';
import { readFile } from 'node:fs/promises';
import { spawnSync } from 'node:child_process';
import path from 'node:path';
const [packageDir, executable] = process.argv.slice(2);
if (!packageDir || !executable) throw new Error('Expected npm package directory and parity executable');
const pkg = JSON.parse(await readFile(path.join(packageDir, 'package.json'), 'utf8'));
const versionFile = await readFile(new URL('../version.go', import.meta.url), 'utf8');
const version = versionFile.match(/const Version = "([^"]+)"/)[1];
assert.equal(pkg.version, version, 'npm package must match pinned upstream tag');
const texts = ['', ' ', '도서관에서 새로운 책을 빌린다', 'AI 문서와 BM25 검색', '도서관', '컴퓨터', '연필과 공책', '도서관의 책을 읽는다', '😀 한국어 문장', '한국\0문장', '오늘은 도서관에서 책을 읽는다', '여러 사람이 함께 읽을 수 있는 책'];
const input = texts.join('\n') + '\n';
function execute(command, args) {
  const run = spawnSync(command, args, {input, encoding:'utf8'});
  assert.equal(run.status, 0, run.stderr || String(run.error));
  const rows = run.stdout.trim().split('\n').map(JSON.parse);
  assert.equal(rows.length, texts.length);
  return rows;
}
const rows = execute(executable, []);
function compare(actual, expected) {
  assert.equal(actual.text, expected.text);
  const tokens = (r) => r.result.tokens.map(({text,pos,start,end})=>({text,pos,start,end}));
  assert.deepEqual(tokens(actual), tokens(expected), actual.text);
  assert.ok(Math.abs(actual.result.score - expected.result.score) < 0.0001, actual.text);
  assert.deepEqual(actual.nouns, expected.nouns, actual.text);
}
const entry = pkg.exports['.'].node?.import ?? pkg.exports['.'].node;
const { Garu } = await import(pathToFileURL(path.resolve(packageDir, entry)).href);
let analyzer;
try {
  analyzer = await Garu.load();
} catch (error) {
  // Some historical npm releases contain an invalid WASM binary. Do not hide
  // runtime/API/parity failures; this fallback is only for WASM compilation errors.
  if (!(error instanceof WebAssembly.CompileError)) throw error;
  console.warn(`garu-ko@${version} WASM cannot compile: ${error.message}`);
  const manifest = fileURLToPath(new URL('../native/Cargo.toml', import.meta.url));
  const expected = execute('cargo', ['run', '--quiet', '--locked', '--release', '--manifest-path', manifest, '--example', 'reference']);
  rows.forEach((row, i) => compare(row, expected[i]));
  console.log(`Matched upstream Rust core v${version}: ${rows.length} inputs; npm WASM comparison unavailable.`);
}
if (analyzer) {
  for (const row of rows) {
    compare(row, {text:row.text, result:analyzer.analyze(row.text), nouns:analyzer.nouns(row.text, {includeSL:true})});
  }
  analyzer.destroy();
  console.log(`Matched garu-ko@${version}: ${rows.length} inputs (tokens, POS, offsets, scores, nouns).`);
}
