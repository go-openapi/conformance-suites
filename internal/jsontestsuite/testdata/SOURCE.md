# Vendored JSON Test Suite

These fixtures are vendored unmodified from Nicolas Seriot's **JSONTestSuite**:

- Upstream: https://github.com/nst/JSONTestSuite
- Companion article: https://seriot.ch/projects/parsing_json.html
- Vendored revision: `1ef36fa`, the directory `test_parsing` only
- License: MIT (see `LICENSE` in this directory)

## File names

- `y_*` must be accepted by a conforming parser.
- `n_*` must be rejected by a conforming parser.
- `i_*` are implementation-defined: either outcome conforms.

## Updating

Copy `test_parsing` from a fresh checkout of the upstream repository and update the revision above.
Do not edit the fixtures in place.
