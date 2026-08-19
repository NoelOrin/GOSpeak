#!/usr/bin/env node
import fs from 'node:fs';
import { execSync as ex } from 'node:child_process';

function sh(cmd) { return ex(cmd, { encoding: 'utf-8' }).trim(); }

const version = process.argv[2] || '0.2.1';
const date = process.argv[3] || new Date().toISOString().slice(0,10);
const compareTag = 'v0.2.0';
const currentTag = `v${version}`;

let commits = [];
try {
  const log = sh(`git log ${compareTag}..HEAD --pretty=format:'%H%x1f%s%x1f%b%x1e' `);
  if (log) {
    for (const entry of log.split('\x1e').filter(Boolean)) {
      const [hash, subject, body] = entry.split('\x1f');
      commits.push({ hash: hash.slice(0,7), fullHash: hash, subject: subject.trim(), body: body.trim() });
    }
  }
} catch (e) {}

let bullets = [];
for (const c of commits) {
  if (c.body) {
    for (const line of c.body.split('\n')) {
      const t = line.trim();
      if (t.startsWith('- ')) bullets.push(t.slice(2).trim());
    }
  }
}
if (bullets.length === 0) {
  bullets = commits.map(c => c.subject);
}

const typeMap = { feat: [], fix: [], docs: [], chore: [], ci: [], refactor: [], perf: [], other: [] };
for (const b of bullets) {
  const m = b.match(/^([\w\/]+):\s*(.*)/);
  let scope = '', desc = b, type = 'fix';
  if (m) { scope = m[1]; desc = m[2]; }
  const low = b.toLowerCase();
  const baseScope = scope.split('/')[0];
  if (scope === 'feat' || low.includes('feat')) type = 'feat';
  else if (scope === 'fix' || low.includes('fix') || ['audit','handler','domain','sfu','bus','server','web','bot'].includes(baseScope)) type = 'fix';
  else if (scope === 'docs' || low.startsWith('docs')) type = 'docs';
  else if (baseScope === 'chore') type = 'chore';
  else if (baseScope === 'ci' || low.startsWith('ci:')) type = 'ci';
  else if (scope === 'refactor') type = 'refactor';
  else if (scope === 'perf') type = 'perf';
  else type = 'fix';
  if (['audit','handler','domain','sfu','bus','server','web','bot'].includes(baseScope)) type = 'fix';
  if (baseScope === 'chore') type = 'chore';
  if (baseScope === 'ci') type = 'ci';
  typeMap[type].push({ scope, desc, raw: b });
}

const totalCommits = commits.length;
const totalBullets = bullets.length;
let counts = [];
for (const k of ['feat','fix','perf','docs','chore','ci','refactor']) {
  if (typeMap[k].length) counts.push(`${k} ${typeMap[k].length}`);
}
if (counts.length === 0) counts.push(`chore ${totalCommits}`);
const summary = `> 自 ${compareTag} 以来的发布，共 ${totalBullets} 个变更点（${counts.join(' / ')})，基于 ${totalCommits} 个提交。`;

let lines = [];
lines.push(`## [${version}](https://github.com/NoelOrin/GOSpeak/compare/${compareTag}...${currentTag}) (${date})`);
lines.push('');
lines.push(summary);
lines.push('');

const sections = [
  { title: 'Features', key: 'feat' },
  { title: 'Bug Fixes', key: 'fix' },
  { title: 'Performance', key: 'perf' },
  { title: 'Documentation', key: 'docs' },
  { title: 'Chore', key: 'chore' },
  { title: 'CI', key: 'ci' },
  { title: 'Refactor', key: 'refactor' },
  { title: 'Other', key: 'other' },
];
for (const sec of sections) {
  const items = typeMap[sec.key];
  if (!items.length) continue;
  lines.push(`### ${sec.title}`);
  lines.push('');
  for (const it of items) {
    let bullet;
    if (sec.key === 'feat') {
      bullet = `* ${it.raw}`;
      if (!bullet.startsWith('feat')) bullet = `* feat(${it.scope}): ${it.desc}`;
    } else if (sec.key === 'fix') {
      if (it.scope) bullet = `* fix(${it.scope}): ${it.desc}`;
      else bullet = `* fix: ${it.desc}`;
    } else {
      if (it.scope) bullet = `* ${it.scope}: ${it.desc}`;
      else bullet = `* ${it.raw}`;
    }
    lines.push(bullet);
  }
  lines.push('');
}

const newSection = lines.join('\n');
console.log(newSection);
console.log('--- counts', counts, 'bullets', totalBullets);

const changelogPath = 'CHANGELOG.md';
let content = fs.readFileSync(changelogPath, 'utf-8');
const oldSectionRegex = new RegExp(`# Changelog / 更新日志\\n\\n## \\[0\\.2\\.1[\\s\\S]*?(?=\\n## 0\\.2\\.0)`);
if (oldSectionRegex.test(content)) {
  content = content.replace(oldSectionRegex, `# Changelog / 更新日志\n\n${newSection}`);
} else {
  const altRegex = /# Changelog \/ 更新日志\n\n## \[0\.2\.1[\s\S]*?\n## 0\.2\.0/;
  if (altRegex.test(content)) {
    content = content.replace(altRegex, `# Changelog / 更新日志\n\n${newSection}## 0.2.0`);
  } else {
    console.error('Failed to locate 0.2.1 section');
    process.exit(1);
  }
}
fs.writeFileSync(changelogPath, content);
console.log('Updated CHANGELOG.md');
