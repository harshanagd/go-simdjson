# AGENTS.md

## Custom Instructions

### Comments and documentation

- Keep comments lean. State the contract or the non-obvious reason, then stop.
- Do not narrate what the code already says, restate a name in prose, or record
  investigation history in a comment — that belongs in the commit message or the issue.
- A doc comment gets one short paragraph. Add a second only for a caller-facing contract
  or a real hazard.
- Prefer one line over three. If a comment is longer than the code it describes, cut it.

### Tests

- Put tests in the existing by-area files (`tape_test.go`, `iter_test.go`,
  `serialize_test.go`, `mutation_test.go`, `marshal_test.go`, `ndjson_test.go`) next to
  the code they exercise. No catch-all test file.
- Test the exact edge a change introduces, and verify the test fails against the unfixed
  code before trusting it.
- `compat_test.go` is deliberately not gofmt-clean. Never run `gofmt -w *.go`; format
  named files, and exclude it from `gofmt -l` checks.

### Tape invariants

- `Tape.Validate` is the trust boundary. `Parse`/`ParseND` output always satisfies it;
  `Serializer.Deserialize` calls it because its input is arbitrary bytes.
- Do not add bounds checks to walkers to guard against a malformed tape. Establish the
  invariant in `Validate` instead — a per-element check measured +24% on
  `BenchmarkForEachArray`.
- Before adding a guard, check whether `Validate` already covers it. If it does, the guard
  is dead code.
