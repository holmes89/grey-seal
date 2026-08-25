# Disposable image grey-seal's aiderrunner.SessionRunner spins up per agent
# run: clones a repo, runs Aider non-interactively against a self-hosted
# LiteLLM endpoint, and pushes the result. See
# lib/repo/aiderrunner/client.go and entrypoint.sh.
FROM python:3.12-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends git ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir aider-chat

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
