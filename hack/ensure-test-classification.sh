#!/usr/bin/env bash

# get the dir of this scipt
dir=$(dirname "$0")

declare -a unclassified

while IFS= read -r -d '' file
do
  [ -z "$(sed -n '/^\/\/ +build/p;q' "$file")" ] && unclassified+=("$file")
done < <(find ${dir}/.. -name '*_test.go' -print0)

if [ "${#unclassified[@]}" -eq "0" ]; then
  echo "OK - all test files contain a build tag"
  exit 0
fi

echo "The following ${#unclassified[@]} test files are not classified with a valid Go build tag [unit|integration]"
for i in "${unclassified[@]}"
do
  echo "$i"
done
exit 1