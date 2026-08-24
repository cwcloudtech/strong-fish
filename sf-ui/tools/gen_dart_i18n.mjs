import ar from '../src/i18n/translations/ar.js';
import en from '../src/i18n/translations/en.js';
import fr from '../src/i18n/translations/fr.js';
import { readFileSync, writeFileSync } from 'fs';

const q = (s) => "'" + String(s).replace(/\\/g, '\\\\').replace(/'/g, "\\'").replace(/\$/g, '\\$').replace(/\n/g, '\\n') + "'";

function emit(value, indent) {
  const pad = '  '.repeat(indent);
  const inner = '  '.repeat(indent + 1);
  const entries = Object.entries(value).map(([k, v]) =>
    inner + q(k) + ': ' + (typeof v === 'object' && v !== null ? emit(v, indent + 1) : q(v)) + ','
  );
  return '{\n' + entries.join('\n') + '\n' + pad + '}';
}

/**
 * Keys the phone app has and the web does not.
 *
 * The mobile dictionaries carry a handful of strings with no web equivalent -
 * "no app on this phone can open this file", the voice-recorder's messages -
 * because those screens only exist here. Regenerating from the web alone used
 * to delete them silently, which is exactly the kind of loss a generator
 * should not cause: what is in the .dart and not in the .js is kept.
 */
function readExisting(path) {
  let source;
  try {
    source = readFileSync(path, 'utf8');
  } catch {
    return {};
  }

  // A small reader for what this generator itself writes: nested maps of
  // single-quoted strings, one entry per line.
  const root = {};
  const stack = [root];
  for (const line of source.split('\n')) {
    const trimmed = line.trim();
    const opening = trimmed.match(/^'([\w.]+)': \{$/);
    if (opening) {
      const child = {};
      stack[stack.length - 1][opening[1]] = child;
      stack.push(child);
      continue;
    }
    if (trimmed === '},' || trimmed === '};') {
      if (stack.length > 1) stack.pop();
      continue;
    }
    const entry = trimmed.match(/^'([\w.]+)': '(.*)',$/);
    if (entry) stack[stack.length - 1][entry[1]] = entry[2];
  }
  return root;
}

/** The web's values, plus anything only the phone has. */
function merge(generated, existing) {
  const merged = { ...generated };
  for (const [key, value] of Object.entries(existing)) {
    if (!(key in merged)) {
      merged[key] = value;
    } else if (typeof merged[key] === 'object' && typeof value === 'object') {
      merged[key] = merge(merged[key], value);
    }
  }
  return merged;
}

for (const [name, dict] of [['en', en], ['fr', fr], ['ar', ar]]) {
  const header = `// Generated from sf-ui/src/i18n/translations/${name}.js - the two clients share
// one set of keys on purpose, so a string added for the web is available here
// too. Regenerate with sf-ui's tools/gen_dart_i18n.mjs rather than editing by
// hand, or the dictionaries drift.
// ignore_for_file: prefer_single_quotes

const Map<String, dynamic> ${name} = `;
  const target = new URL(`../../sf-mobile/lib/i18n/${name}.dart`, import.meta.url);
  // The existing file is read back first: its escaped strings are re-emitted
  // as they are, so a preserved key round-trips unchanged.
  const merged = merge(dict, readExisting(target));
  writeFileSync(target, header + emit(merged, 0) + ';\n');
  console.log(name, 'written');
}
