# SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
# SPDX-License-Identifier: Apache-2.0
#
# Reads one JSON line per document on stdin, {"src": "<base64>", "mode": "1.2"}, loads every document in it with
# libfyaml's Python binding, and writes one JSON line per input on stdout: {"docs": ["<json>", ...]} when libfyaml
# reads the stream, {"error": "<the last line of the traceback>"} when it raises.
#
# The interpreter and the binding load once for the whole run: that costs about 18 ms, as much as reading a document.
import base64
import json
import sys
import traceback

import libfyaml

for line in sys.stdin:
    request = json.loads(line)
    # An empty document arrives as null: Go writes an empty []byte that way.
    src = base64.b64decode(request.get("src") or "").decode("utf-8", "surrogateescape")
    try:
        docs = [libfyaml.json_dumps(doc) for doc in libfyaml.loads_all(src, mode=request.get("mode", "1.2"))]
        answer = {"docs": docs}
    except Exception as e:  # every failure of the binding is an answer about this document
        answer = {"error": traceback.format_exception_only(e)[-1].strip()}
    sys.stdout.write(json.dumps(answer) + "\n")
    sys.stdout.flush()
