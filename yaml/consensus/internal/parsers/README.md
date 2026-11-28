# Parser drivers

`consensus run` asks each source about each document through a driver: a `consensus drive <id>` process that
reads one request per line on its stdin and writes one response per line on its stdout, in the order of the
requests. The runner starts the driver on the first case and restarts it after a crash or a timeout.

```
request   {"sha":"<hex sha256 of src>","src":"<base64 document>"}
response  {"sha":"<same sha>","answer":{"source":"go-yaml","verdict":"accept","docs":1,"json":["{\"a\":1}"],"elapsedMs":0.08},"seconds":0.0001}
```

`answer` is a `consensus.Answer`. `seconds` is how long the driver took, from reading the request to writing the
response.

## Failures

- A panic while answering one document becomes an answer with verdict `error` and the message `panic: ...`; the
  driver goes on.
- A driver that writes no answer within `-timeout` plus 5 s is killed. The case answers `error: timed out`.
- A driver that exits, or writes something other than one JSON line, answers `error: crashed: <reason>`. The reason
  is the last `fatal error:` or `panic:` line of its stderr, or else its last line.
- After 10 crashes the runner drops the source. The results header lists it under `dropped`.

## The parsers

Each slow parser keeps one process of its own for the whole corpus, behind the driver, and speaks the same kind of
JSON lines (see `internal/jsonlines`):

| directory | process | request | response |
|---|---|---|---|
| `perlref` | `perl -e batch.pl` | `{"src"}` | `{"ok","events","error"}` |
| `libfyaml` | `python3 -c batch.py` | `{"src","mode"}` | `{"docs"}` or `{"error"}` |
| `goopenapi` | `goyaml-json -lines` | `{"src"}` | the answer `goyaml-json` writes for one stream |

`goopenapi/goyaml-json` is a module of its own, so that the consensus module does not require go-openapi/go-yaml.
The consensus image builds it from a pinned version, or from a local checkout.

yaml.v3 runs inside the driver. YAMLStar runs its `yaml` command for each document, because the command reads one
stream and its library has no event API.
