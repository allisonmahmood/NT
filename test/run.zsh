#!/bin/zsh
# Run the whole test suite; exit nonzero if anything fails.
emulate -L zsh
DIR="${${(%):-%x}:A:h}"
rc=0
for t in "$DIR"/test_*.zsh; do
  print "\n######## ${t:t} ########"
  /bin/zsh "$t" || rc=1
done
print "\n######## overall: $([[ $rc -eq 0 ]] && print OK || print FAILED) ########"
exit $rc
