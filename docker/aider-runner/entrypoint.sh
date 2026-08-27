#!/usr/bin/env bash
# Entrypoint for the aider-runner image. Clones REPO_URL, checks out
# PUSH_BRANCH (from BASE_BRANCH if set, else the repo's default branch),
# runs Aider non-interactively against the given task, and pushes the
# result. Exit code is the sole success signal grey-seal's SessionRunner
# reads (see lib/repo/aiderrunner) — 0 means "satisfied", anything else
# means "failed", including the case where Aider made no commits at all.
set -euo pipefail

: "${REPO_URL:?REPO_URL is required}"
: "${PUSH_BRANCH:?PUSH_BRANCH is required}"
# file:// repos are beaver's local-only mode (a bare repo on a bind-mounted
# host directory, not a real GitHub remote) — no token needed to clone/push
# a local path, and the AUTH_URL substitution below is already a no-op for
# non-https:// URLs.
case "$REPO_URL" in
  file://*) ;;
  *) : "${GITHUB_TOKEN:?GITHUB_TOKEN is required}" ;;
esac
: "${TASK_DESCRIPTION:?TASK_DESCRIPTION is required}"
: "${OPENAI_API_BASE:?OPENAI_API_BASE is required}"
: "${OPENAI_API_KEY:?OPENAI_API_KEY is required}"
: "${AIDER_MODEL:?AIDER_MODEL is required}"

export OPENAI_API_BASE OPENAI_API_KEY

# Inject the token into the clone/push URL rather than passing it to aider
# or leaving it in a git remote config that might get logged.
AUTH_URL=$(printf '%s' "$REPO_URL" | sed -E "s#https://#https://x-access-token:${GITHUB_TOKEN}@#")

git config --global user.email "grey-seal-agent@localhost"
git config --global user.name "grey-seal agent"
git config --global advice.detachedHead false

echo "cloning ${REPO_URL}"
if [ -n "${BASE_BRANCH:-}" ]; then
  git clone --branch "$BASE_BRANCH" --single-branch "$AUTH_URL" /repo
else
  git clone "$AUTH_URL" /repo
fi

cd /repo
git checkout -b "$PUSH_BRANCH"
START_SHA=$(git rev-parse HEAD)

echo "running aider (model=openai/${AIDER_MODEL})"
# --edit-format whole: local/quantized models struggle to follow the default
# diff (SEARCH/REPLACE) format, which requires reproducing exact original
# file text before replacing it — aider's own troubleshooting docs recommend
# "whole" for exactly this case (the LLM returns the full updated file
# instead). Observed live without this flag: the model wrote plausible code
# into brand-new files named after the TODO description text instead of
# editing the real file, so "aider made a commit" looked like success while
# the actual TODO(agent) markers were untouched.
aider --yes-always --no-check-update --no-gitignore \
  --model "openai/${AIDER_MODEL}" \
  --edit-format whole \
  --message "$TASK_DESCRIPTION"

END_SHA=$(git rev-parse HEAD)
if [ "$START_SHA" = "$END_SHA" ]; then
  echo "aider made no commits — nothing to push" >&2
  exit 1
fi

echo "pushing ${PUSH_BRANCH}"
git push "$AUTH_URL" "HEAD:$PUSH_BRANCH"
