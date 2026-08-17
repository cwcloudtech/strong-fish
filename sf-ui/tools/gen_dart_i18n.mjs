import en from '../src/i18n/translations/en.js';
import fr from '../src/i18n/translations/fr.js';
import { writeFileSync } from 'fs';

const q = (s) => "'" + String(s).replace(/\\/g, '\\\\').replace(/'/g, "\\'").replace(/\$/g, '\\$').replace(/\n/g, '\\n') + "'";

function emit(value, indent) {
  const pad = '  '.repeat(indent);
  const inner = '  '.repeat(indent + 1);
  const entries = Object.entries(value).map(([k, v]) =>
    inner + q(k) + ': ' + (typeof v === 'object' && v !== null ? emit(v, indent + 1) : q(v)) + ','
  );
  return '{\n' + entries.join('\n') + '\n' + pad + '}';
}

for (const [name, dict] of [['en', en], ['fr', fr]]) {
  const header = `// Generated from sf-ui/src/i18n/translations/${name}.js - the two clients share
// one set of keys on purpose, so a string added for the web is available here
// too. Regenerate with sf-ui's tools/gen_dart_i18n.mjs rather than editing by
// hand, or the dictionaries drift.
// ignore_for_file: prefer_single_quotes

const Map<String, dynamic> ${name} = `;
  writeFileSync(new URL(`../../sf-mobile/lib/i18n/${name}.dart`, import.meta.url), header + emit(dict, 0) + ';\n');
  console.log(name, 'written');
}
