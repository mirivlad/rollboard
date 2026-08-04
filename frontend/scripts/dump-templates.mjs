// dump-templates.mjs — print every bundled game template as JSON.
//
// The template definitions live in TypeScript because that is where the editor
// reads them from. scripts/validate-templates.sh pipes this into the Go
// validator, so the definitions a new author actually starts from are the ones
// checked, rather than a copy of them.
//
// scripts/validate-demos.sh used to hold its own transcription of two of these
// boards in a shell variable. It could only ever drift from the real thing, and
// it did: it was still describing a board with no colour groups after the
// template had grown them.
//
// Node loads the .ts sources directly (type stripping, Node 22.6+), so this
// needs no bundler and no node_modules.

import { createMonopolyDemo } from '../src/lib/monopoly.ts';
import { createRpgDemo } from '../src/lib/rpg.ts';
import {
  createDefaultGame,
  createMiniMonopolyDemo,
  createDungeonRaceDemo,
  createBranchingDemo,
  createManualBranchDemo,
} from '../src/lib/defaults.ts';

const templates = {
  'Blank board': createDefaultGame(),
  'Monopoly': createMonopolyDemo(),
  'Dungeon Crawl': createRpgDemo(),
  'Mini-Monopoly': createMiniMonopolyDemo(),
  'Dungeon Race': createDungeonRaceDemo(),
  'Branching paths': createBranchingDemo(),
  'Manual branch': createManualBranchDemo(),
};

// A template carries no ID until it is saved; the validator requires one, so
// each gets a stable stand-in that names it in any error message.
for (const [name, definition] of Object.entries(templates)) {
  definition.id = `template-${name.toLowerCase().replace(/[^a-z0-9]+/g, '-')}`;
}

process.stdout.write(JSON.stringify(templates, null, 2));
