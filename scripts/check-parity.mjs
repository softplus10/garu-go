// Usage: npm install --prefix /tmp/garu-parity garu-ko@<matching version>
// node scripts/check-parity.mjs /tmp/garu-parity/node_modules/garu-ko /tmp/garu-parity-cli
import assert from 'node:assert/strict';
import { pathToFileURL } from 'node:url';
import { readFile } from 'node:fs/promises';
import { spawnSync } from 'node:child_process';
import path from 'node:path';
const [packageDir, executable] = process.argv.slice(2);
if (!packageDir || !executable) throw new Error('Expected npm package directory and parity executable');
const pkg = JSON.parse(await readFile(path.join(packageDir, 'package.json'), 'utf8'));
const versionFile = await readFile(new URL('../version.go', import.meta.url), 'utf8');
const version = versionFile.match(/const Version = "([^"]+)"/)[1];
assert.equal(`v${pkg.version}`, `v${version}`, 'npm package must match vendored upstream tag');
const entry = pkg.exports['.'].node?.import ?? pkg.exports['.'].node;
const { Garu } = await import(pathToFileURL(path.resolve(packageDir, entry)).href);
const analyzer = await Garu.load();
const texts = ['', ' ', '도서관에서 새로운 책을 빌린다', 'AI 문서와 BM25 검색', '도서관', '컴퓨터', '연필과 공책', '도서관의 책을 읽는다', '😀 한국어 문장', '한국\0문장', '오늘은 도서관에서 책을 읽는다', '여러 사람이 함께 읽을 수 있는 책'];
const run = spawnSync(executable, [], {input: texts.join('\n') + '\n', encoding:'utf8'});
assert.equal(run.status, 0, run.stderr);
const rows = run.stdout.trim().split('\n').map(JSON.parse);
assert.equal(rows.length, texts.length);
for (const row of rows) {
  const expected = analyzer.analyze(row.text);
  assert.deepEqual(row.result.tokens.map(({text,pos,start,end})=>({text,pos,start,end})), expected.tokens.map(({text,pos,start,end})=>({text,pos,start,end})), row.text);
  assert.ok(Math.abs(row.result.score - expected.score) < 0.0001, row.text);
  assert.deepEqual(row.nouns, analyzer.nouns(row.text, {includeSL:true}), row.text);
}
analyzer.destroy();
console.log(`Matched garu-ko@${pkg.version}: ${rows.length} inputs (tokens, POS, offsets, scores, nouns).`);
