#!/bin/sh
# Zabezpieczenie: zakończ skrypt, jeśli którekolwiek polecenie się nie powiedzie
set -e

echo "Starting Playwright tests..."

# Sprawdź zmienną środowiskową HEADED_MODE
if [ "$HEADED_MODE" = "true" ]; then
  echo "Running in headed mode."
  # Użyj --, aby oddzielić opcje pnpm od opcji playwright
  pnpm run test:e2e -- --headed
else
  echo "Running in headless mode."
  pnpm run test:e2e
fi

echo "Playwright tests finished."
