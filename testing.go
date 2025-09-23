package main

import "dagger.io/dagger"

// RunPlaywrightTests runs the Playwright tests against the running environment.
func RunPlaywrightTests(client *dagger.Client, endpoint *dagger.Service) *dagger.Container {
	src := client.Host().Directory("../playwright-tests")

	// Use the official Playwright image which has browsers pre-installed.
	// The image also includes Node.js, so we can use corepack to enable pnpm.
	return client.Container().From("mcr.microsoft.com/playwright:v1.44.0-jammy").
		// Enable pnpm using corepack
		WithExec([]string{"corepack", "enable"}).
		// Mount source and run tests
		WithMountedDirectory("/app", src).
		WithWorkdir("/app").
		WithMountedCache("/root/.local/share/pnpm/store/v3", client.CacheVolume("pnpm-cache-tests")).
		WithExec([]string{"pnpm", "install", "--frozen-lockfile"}).
		WithServiceBinding("caddy", endpoint).
		WithExec([]string{"pnpm", "test", "--base-url=https://caddy"})
}
