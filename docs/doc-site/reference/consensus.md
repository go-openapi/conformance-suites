---
title: Consensus report
weight: 30
description: |
  Five YAML parsers read the compact YAML corpus: where they agree with it, and with each other.
---

The consensus report compares five YAML parsers on every case of the compact YAML corpus. For each parser it
scores the verdict (accept or reject), the loaded value and the round trip against the corpus, and lists the cases
on which the parsers disagree, by tag and by feature.

{{< consensus >}}
