# Playwright's official image ships with all browsers preinstalled, so we
# don't need to run `playwright install` at runtime — image is self-contained.
FROM mcr.microsoft.com/playwright:v1.47.2-jammy

WORKDIR /e2e

# Install npm deps into the image so every `docker compose run` reuses the
# prebuilt node_modules instead of fetching it each time.
COPY package.json ./
RUN npm install --no-audit --no-fund

COPY . .

# Default command mirrors `npm test` inside the e2e/ directory.
CMD ["npx", "playwright", "test"]
