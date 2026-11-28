# SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
# SPDX-License-Identifier: Apache-2.0
#
# Reads one JSON line per document on stdin, {"src": "<base64>"}, parses it with the YAML reference parser, and writes
# one JSON line per document on stdout: {"ok": true|false, "events": "<test.event lines>", "error": "<why>"}.
#
# The parser loads once for the whole run: loading it costs about 65 ms, more than parsing most documents. Run it as
# perl -I<home>/lib -I<home>/ext/perl/lib/perl5 batch.pl, where <home> is the reference parser's checkout.
use v5.12;
use Encode;
use JSON::PP;
use MIME::Base64;
use Prelude;
use Parser;
use TestReceiver;

$| = 1;
# utf8: the output is UTF-8 bytes. A code point past U+10FFFF, which an escape such as "\U00110023" yields, is written
# as Perl writes it in its own UTF-8 extension, as the one-process-per-document yaml-parser did.
my $json = JSON::PP->new->utf8->canonical;

while (my $line = <STDIN>) {
  my $in = $json->decode($line);
  # An empty document arrives as null: Go writes an empty []byte that way.
  my $yaml = decode_utf8(decode_base64($in->{src} // ''));
  my $parser = Parser->new(TestReceiver->new);
  my $ok = eval { $parser->parse($yaml); 1 };
  my $error = $ok ? '' : "$@";
  my $events = eval { $parser->{receiver}->output() } // '';
  print $json->encode({ok => $ok ? JSON::PP::true : JSON::PP::false, events => $events, error => $error}), "\n";
}
