#!/usr/bin/env -S bash -e

# Copyright 2019 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Use the Google Closure Compiler to minimize and concatenate
# our JavasScript.
# With -check, it checks that the source files are not newer
# than the compiled ones instead.

source devtools/lib.sh || { echo "Are you at repo root?"; exit 1; }

JSDIR=content/static/js

# compile OUTFILE INFILE1 INFILE2 ...
compile() {
  local outfile=$1
  shift
  rm -f $outfile
  docker run --rm -i femtopixel/google-closure-compiler-app:v20200112 < <(cat $@) > $outfile
  echo "wrote $outfile"
}

check() {
  local outfile=$1
  shift
  for infile in $@; do
    if [[ $infile -nt $outfile ]]; then
      echo "$infile is newer than $outfile; run devtools/compile_js.sh"
      exit 1
    fi
  done
  echo "checked $outfile"
}

main() {
  local cmd=compile
  if [[ $1 = "-check" ]]; then
    cmd=check
  fi
  $cmd $JSDIR/base.min.js       $JSDIR/{site,analytics}.js
  # TODO: once this is not an experiment, add it to the line above.
  $cmd $JSDIR/completion.min.js $JSDIR/completion.js
  $cmd $JSDIR/jump.min.js       third_party/dialog-polyfill/dialog-polyfill.js $JSDIR/jump.js
}

main $@
