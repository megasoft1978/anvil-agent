// Host-only grading subprocess. Never calls a model or exposes oracles to its worktree.
import { readFileSync, writeFileSync } from "node:fs";
import { gradeDirectory } from "../grade.mjs";
const [input, output] = process.argv.slice(2);
if (!input || !output) throw new Error("expected grading input and output paths");
const spec = JSON.parse(readFileSync(input, "utf8"));
const result = gradeDirectory(spec.scenario, spec.root, spec.oracleRoot, spec.scratchDir, spec.options);
writeFileSync(output, JSON.stringify(result), { mode: 0o600 });
