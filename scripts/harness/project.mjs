#!/usr/bin/env node
// Project adapter. No dependencies, package install, network calls or shell interpolation.
import { existsSync, readFileSync, writeFileSync, mkdirSync, readdirSync, lstatSync, renameSync } from 'node:fs';
import { resolve, dirname, join, relative, isAbsolute, extname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { parseArgs } from 'node:util';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const hash = (bytes) => createHash('sha256').update(bytes).digest('hex');
function local(path) {
  const absolute = resolve(root, path);
  const rel = relative(root, absolute);
  if (isAbsolute(rel) || rel === '..' || rel.startsWith('..' + '/') || rel.startsWith('..' + '\\')) throw new Error('Path escapes project');
  for (let part = absolute; ; part = dirname(part)) {
    if (existsSync(part) && lstatSync(part).isSymbolicLink()) throw new Error('Symlink/junction path is not allowed');
    if (part === root) break;
  }
  return absolute;
}
const read = (path) => readFileSync(local(path), 'utf8').replace(/^\uFEFF/, '');
const json = (path) => JSON.parse(read(path));
const present = (path) => existsSync(local(path));
function write(path, text) {
  const target = local(path);
  mkdirSync(dirname(target), { recursive: true });
  const temporary = local(path + '.tmp-' + process.pid);
  writeFileSync(temporary, text, { encoding: 'utf8', mode: 0o600 });
  renameSync(temporary, target);
}
const config = json('.harness/project.json');
if (config.schemaVersion !== 1) throw new Error('Unsupported harness schema');
const skipped = new Set(['node_modules', 'dist', 'wailsjs', 'build', 'secrets', 'logs', '.git', '.tmp', '.artifacts', 'old_snowden', 'archive', 'evidence']);
const privateArtifact = (name) => /(?:\.env$|^\.env(?:\.|$)|\.local\.json$|^vault(?:[.-]|$)|^secrets(?:[.-]|$)|^runtime[-.]config|^inventory[-.]secrets|\.(?:pem|key|pfx|p12|log)$)/i.test(name);
const extensions = new Set(['.go', '.mod', '.sum', '.ts', '.vue', '.css', '.html', '.json', '.mjs', '.md', '.ps1', '.nsi', '.sh', '.yml']);
function digest(scope) {
  const files = new Map();
  function collect(path, explicit = false) {
    if (!present(path)) { files.set(path, 'missing'); return; }
    const stat = lstatSync(local(path));
    if (stat.isDirectory()) {
      for (const entry of readdirSync(local(path), { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name))) {
        if (entry.isSymbolicLink() || skipped.has(entry.name) || privateArtifact(entry.name)) continue;
        collect(join(path, entry.name));
      }
    } else if (stat.isFile() && (explicit || extensions.has(extname(path)))) {
      if (files.size > 10000) throw new Error('Source inventory unexpectedly large');
      files.set(path.replaceAll('\\', '/'), hash(readFileSync(local(path))));
    }
  }
  for (const path of config.profiles[scope].inputs) collect(path, true);
  // These public generated inputs affect embedded Go and frontend compilation.
  if (scope === 'backend' && present('windows/frontend/dist/index.html')) collect('windows/frontend/dist/index.html', true);
  if (scope === 'frontend' && present('windows/frontend/wailsjs')) {
    for (const dir of ['windows/frontend/wailsjs/go', 'windows/frontend/wailsjs/runtime']) if (present(dir)) collect(dir);
  }
  const installed = [];
  if (scope !== 'harness') for (const pkg of ['vite', 'vue-tsc', 'typescript', 'vue']) {
    const path = `windows/frontend/node_modules/${pkg}/package.json`;
    if (present(path)) installed.push([pkg, json(path).version]);
  }
  return { sha256: hash(JSON.stringify([...files].sort()) + JSON.stringify(installed)), files: files.size, installed };
}
function commandSpec(command) {
  return { ...command, exe: command.exe === '$node' ? process.execPath : command.exe === '$task' ? local('.tools/task/task.exe') : command.exe,
    args: command.args.map((arg) => arg === '$tags' ? config.tags : arg) };
}
function map() {
  const pkg = json('windows/frontend/package.json');
  const goVersion = read('windows/go.mod').match(/^go\s+(.+)$/m)?.[1] ?? 'см. go.mod';
  const lines = ['# MAP — активное дерево Snowden System', '',
    '> Генерируется: `node scripts/harness/project.mjs map`. Не редактировать вручную.', '',
    '## Быстрый вход', '',
    '- Продолжение: [STATE](STATE.md) → активный пункт [PLAN](PLAN.md) → исходники.',
    '- Правила: корневой AGENTS + ближайший scoped AGENTS; справочник docs/AGENTS.md.',
    '- Подтверждённые уроки: поиск по docs/LESSONS.md; аудит: docs/OFFLINE-AUDIT-2026-10-18.md.',
    '- История не загружается по умолчанию: old_snowden/, docs/archive/, docs/memory/, SESSION-STATE.', '',
    '## Стек / источники команд', '',
    `- Go ${goVersion}, embedded sing-box/Wails: windows/go.mod + windows/tools/package.ps1.`,
    `- Vue ${pkg.dependencies.vue}, TypeScript ${pkg.devDependencies.typescript}, Vite: windows/frontend/package.json.`,
    `- Production tags: \`${config.tags}\`.`,
      '- Корневой package.json — граница Node tooling и команды тестов; зависимости в windows/frontend. PostgreSQL нет; Android не начат.',
    '- Команды проекта: Taskfile.yml (готовый Task); pinned инструменты: .harness/tools.lock.json.', '',
    '## Где искать', '', '| Область | Реальные входы |', '|---|---|'];
  for (const zone of config.zones) lines.push(`| ${zone.name} | ${zone.paths.map((path) => '`' + path + '`').join(', ')} |`);
  lines.push('', '## Владение и reuse', '',
    '- Текущее: UI → App → Manager → Engine. Наблюдатели передают решения через Manager.',
    '- Целевые ProtectionController/EngineSession — planned FR-009/010, не существующий API.',
    '- UI: views/composition → существующие composables → src/api/backend.ts; contract.ts — типы.',
    '- Компоненты: NavRail/ContextTabs/TopBar/SectionHeader/ProbeReportView/HintBox; сначала проверить их API.',
    '- Wails bindings генерируются; runtime config строится backend/render, не копией assets/configs.', '',
    '## Работа с обвязкой', '',
    '```powershell', 'node scripts/harness/project.mjs doctor', 'node scripts/harness/project.mjs next',
    'node scripts/harness/project.mjs check --scope harness', '```', '',
    '`check` без `--run` ничего не выполняет. Scope frontend/backend — по запросу проверки.',
    'Doctor — статический, не подтверждает VPN, приложение, VPS или сборку.',
    'Release/admin/live не запускаются этим runner. Полный порядок — docs/HARNESS.md.', '',
    '## Данные и артефакты', '',
    '- Public descriptors: windows/backend/render/descriptors.json; server templates: configs/singbox/.',
    '- Не индексировать значения секретов, env, vault, raw logs. MAP читает только перечисленные публичные манифесты.',
    '- .artifacts/harness/last-<scope>.json — локальные итоги; docs/evidence/ — только redacted acceptance.',
    '- Source digest — фильтр устаревания, не доказательство совпадения всей ОС/сети/инструментария.', '');
  write('docs/MAP.md', lines.join('\n'));
  return { ok: true, file: 'docs/MAP.md', lines: lines.length, scope: 'Explicit active paths; no legacy tree traversal' };
}
function doctor() {
  const checks = [];
  const add = (id, status, detail) => checks.push({ id, status, detail });
  add('node', Number(process.versions.node.split('.')[0]) >= 22 && (Number(process.versions.node.split('.')[0]) > 22 || Number(process.versions.node.split('.')[1]) >= 13) ? 'pass' : 'error', process.version);
  for (const path of config.required) add(path, present(path) ? 'pass' : 'error', 'Required project entrypoint');
  for (const zone of config.zones) for (const path of zone.paths) if (!present(path)) add('map:' + path, 'error', 'Configured code path missing');
  for (const [path, max] of Object.entries(config.contextBudgets)) if (present(path)) {
    const text = read(path), lines = text.trimEnd().split(/\r?\n/).length;
    add('budget:' + path, lines <= max ? 'pass' : 'warn', { lines, max, utf8Bytes: Buffer.byteLength(text) });
  }
  const plan = read(config.plan);
  for (let i = 1; i <= 9; i++) {
    const id = 'OFF-0' + i;
    add('task:' + id, plan.split('\n').filter((line) => line.startsWith('| ' + id + ' |')).length === 1 ? 'pass' : 'error', 'Exactly one canonical task row');
  }
  add('next-task', plan.includes('| ' + config.next.id + ' |') && present(config.next.spec) && read(config.state).includes(config.next.id) ? 'pass' : 'error', config.next);
  add('production-tags', read('windows/tools/package.ps1').includes(config.tags) ? 'pass' : 'error', 'Tags match package.ps1');
  add('task-tags', read('Taskfile.yml').includes('PRODUCTION_TAGS: ' + config.tags) ? 'pass' : 'error', 'Task tags match package.ps1');
  add('task-dry-run', !/^\s*(?:sh|dotenv|includes):/m.test(read('Taskfile.yml')) ? 'pass' : 'error', 'No dynamic sh vars, dotenv or remote includes in this Taskfile');
  const roots = read('AGENTS.md');
  add('managed-block', roots.split('<!-- project-harness:start -->').length === 2 && roots.split('<!-- project-harness:end -->').length === 2 ? 'pass' : 'error', 'Portable block preserved');
  add('offline-mode', config.mode === 'offline' ? 'pass' : 'warn', 'No live/admin checks registered');
  const native = config.profiles.frontend.commands;
  for (const step of native) for (const path of step.requires ?? []) add('prerequisite:' + path, present(path) ? 'pass' : 'warn', 'No automatic dependency install');
  add('frontend-script', json('windows/frontend/package.json').scripts.build === 'vue-tsc --noEmit && vite build' ? 'pass' : 'warn', 'Runner must be reviewed when native build script changes');
  const git = spawnSync('git', ['rev-parse', '--verify', 'HEAD'], { cwd: root, encoding: 'utf8', windowsHide: true, timeout: 5000 });
  add('revision', git.status === 0 ? 'pass' : 'warn', git.status === 0 ? git.stdout.trim() : 'No commit baseline; source digest available, release reproducibility pending OFF-07');
  const installed = present('.tools/installed.json') ? json('.tools/installed.json') : { tools: [] };
  const locked = json('.harness/tools.lock.json');
  for (const tool of [...locked.downloads, ...locked.goTools]) {
    const path = tool.entry ? `.tools/${tool.name}/${tool.entry}` : `.tools/go/${tool.name}.exe`;
    const record = installed.tools.find((entry) => entry.name === tool.name);
    const valid = present(path) && record?.version === tool.version && record?.executableSha256 === hash(readFileSync(local(path)));
    add('tool:' + tool.name, valid ? 'pass' : 'warn', { version: tool.version, present: present(path), note: 'Local receipt; download hash is pinned in tools.lock.json. No global install.' });
  }
  add('product-gates', 'info', 'P0/P1 remain open; prior application results are in V2-094, not rerun by doctor');
  add('efficiency', 'info', 'Token benefit not measured; use explicit exported/manual eval data, no private session scraping');
  return { ok: !checks.some((check) => check.status === 'error'), checks, limits: 'Static file/manifest checks only; no application tests, network or OS policy changes' };
}
function secrets() {
  const exe = local('.tools/gitleaks/gitleaks.exe');
  if (!present('.tools/gitleaks/gitleaks.exe')) return { ok: false, status: 'blocked', reason: 'Install pinned Gitleaks explicitly; no automatic install' };
  const names = spawnSync('git', ['diff', '--cached', '--name-only', '-z', '--no-ext-diff'], { cwd: root, encoding: 'utf8', windowsHide: true, timeout: 10000 });
  if (names.status !== 0 || !names.stdout?.length) return { ok: false, status: 'blocked', reason: 'No staged files or Git unavailable; empty scan is not a security PASS' };
  const diff = spawnSync('git', ['diff', '--cached', '--no-ext-diff', '--no-textconv', '--unified=0'], { cwd: root, encoding: 'utf8', windowsHide: true, timeout: 20000, maxBuffer: 16 * 1024 * 1024 });
  if (diff.status !== 0 || !diff.stdout?.length) return { ok: false, status: 'blocked', reason: 'Staged text diff unavailable; binary contents are not covered' };
  const scan = spawnSync(exe, ['stdin', '--config', local('.harness/gitleaks.toml'), '--redact=100', '--no-banner', '--no-color', '--ignore-gitleaks-allow', '--timeout', '60'],
    { cwd: root, input: diff.stdout, encoding: 'utf8', windowsHide: true, timeout: 70000, maxBuffer: 1024 * 1024 });
  return { ok: scan.status === 0, status: scan.status === 0 ? 'pass' : scan.status === 1 ? 'findings' : 'blocked',
    stagedFiles: names.stdout.split('\0').filter(Boolean).length, exitCode: scan.status, rawLogsSaved: false,
    note: 'Staged text diff only. Secret matches and source contents are not printed or saved. No provider validation or revocation.' };
}
function check(scope, run) {
  const profile = config.profiles[scope];
  if (!profile) throw new Error('Scope must be harness, frontend or backend; live/admin/release are intentionally excluded');
  const commands = profile.commands.map(commandSpec);
  if (!run) return { ok: true, executed: false, scope, commands, note: 'Use --run only for an authorized verification task. No dependency installation.' };
  const startedAt = new Date().toISOString(), before = digest(scope), results = [];
  for (const command of commands) {
    const missing = (command.requires ?? []).filter((path) => !present(path));
    if (missing.length) { results.push({ id: command.id, status: 'blocked', missing }); break; }
    const start = performance.now();
    const result = spawnSync(command.exe, command.args, { cwd: local(command.cwd), encoding: 'utf8',
      windowsHide: true, timeout: command.timeoutMs ?? 120000, maxBuffer: 1024 * 1024, shell: false, stdio: ['ignore', 'pipe', 'pipe'] });
    const row = { id: command.id, status: result.status === 0 ? 'pass' : result.error?.code === 'ENOENT' ? 'blocked' : 'fail',
      exitCode: result.status, signal: result.signal, seconds: Math.round((performance.now() - start) / 10) / 100,
      outputBytes: Buffer.byteLength(result.stdout ?? '') + Buffer.byteLength(result.stderr ?? '') };
    if (result.error) row.errorCode = result.error.code ?? 'execution-error';
    if (row.status !== 'pass' && command.onFailure) row.hint = command.onFailure;
    results.push(row);
    // Emit progress without persisting or echoing potentially sensitive raw logs.
    process.stderr.write(`${command.id}: ${row.status} (${row.seconds}s)\n`);
    if (row.status !== 'pass') break;
  }
  const after = digest(scope);
  const report = { ok: results.length === commands.length && results.every((row) => row.status === 'pass'),
    executed: true, scope, startedAt, completedAt: new Date().toISOString(), node: process.version,
    sourceBefore: before, sourceAfter: after, sourceChangedDuringRun: before.sha256 !== after.sha256,
    commands, results, rawLogsSaved: false, note: 'Source identity is bounded; compare toolchain, external config and requested scenario before reuse.' };
  const file = `.artifacts/harness/last-${scope}.json`;
  write(file, JSON.stringify(report, null, 2) + '\n');
  return { ...report, file };
}
try {
  const { positionals, values } = parseArgs({ allowPositionals: true, options: { scope: { type: 'string', default: 'harness' }, run: { type: 'boolean', default: false }, details: { type: 'boolean', default: false } } });
  if (positionals.length !== 1) throw new Error('Usage: project.mjs map|doctor|next|secrets|check [--scope harness|frontend|backend] [--run]');
  const action = positionals[0];
  if ((values.run || values.scope !== 'harness') && action !== 'check') throw new Error('--scope/--run apply only to check');
  if (values.details && action !== 'doctor') throw new Error('--details applies only to doctor');
  let result;
  switch (action) {
    case 'map': result = map(); break;
    case 'doctor': {
      const full = doctor();
      result = values.details ? full : { ok: full.ok,
        counts: Object.fromEntries(['pass', 'warn', 'error', 'info'].map((status) => [status, full.checks.filter((item) => item.status === status).length])),
        attention: full.checks.filter((item) => item.status === 'warn' || item.status === 'error'),
        limits: full.limits, details: 'Use doctor --details only when individual checks matter' };
      break;
    }
    case 'check': result = check(values.scope, values.run); break;
    case 'secrets': result = secrets(); break;
    case 'next': result = { ok: present(config.next.spec), task: config.next, state: config.state, plan: config.plan,
      note: 'Read STATE and the canonical task row before continuing. This does not authorize product implementation or run commands.' }; break;
    default: throw new Error('Unknown action');
  }
  console.log(JSON.stringify(result, null, 2));
  process.exitCode = result.ok ? 0 : 1;
} catch (error) {
  console.log(JSON.stringify({ ok: false, error: 'project-harness', message: error.message }));
  process.exitCode = 1;
}
