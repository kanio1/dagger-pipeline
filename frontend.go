package main

import "dagger.io/dagger"

// NewFrontendService creates the Nuxt.js frontend service.
func NewFrontendService(client *dagger.Client) *dagger.Service {
	src := client.Host().Directory("../frontend-app")
	pnpmCache := client.CacheVolume("pnpm-cache")

	return client.Container().From("node:22.4.0").
		WithMountedDirectory("/app", src).
		WithWorkdir("/app").
		WithMountedCache("/root/.local/share/pnpm/store/v3", pnpmCache).
		WithExec([]string{"corepack", "enable"}).
		WithExec([]string{"pnpm", "install", "--frozen-lockfile"}).
		WithExposedPort(3000).
		WithExec([]string{"pnpm", "run", "dev", "--host", "0.0.0.0"}).
		AsService()
}
