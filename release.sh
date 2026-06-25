#!/bin/sh
set -e

# Determine next tag: YY.MM.NN-stable
today=$(date +%y.%m)
last=$(git tag --sort=-version:refname | grep -E "^${today}\.[0-9]+-stable$" | head -1)

if [ -n "$last" ]; then
    n=$(echo "$last" | sed "s/^${today}\.\([0-9]*\)-stable$/\1/")
    next="${today}.$((n + 1))-stable"
else
    next="${today}.01-stable"
fi

echo "Next tag: $next"
printf "Continue? [Y/n] "
read -r ans
case "$ans" in
    n|N) echo "Aborted."; exit 0 ;;
esac

git tag -a "$next" -m "$next"
echo "Created tag $next"

printf "Push to origin? [Y/n] "
read -r ans
case "$ans" in
    n|N) echo "Tag created locally only."; exit 0 ;;
esac

git push origin "$next"
echo "Pushed $next — release workflow started."
