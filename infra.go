package main

import "dagger.io/dagger"

// NewCaddyService creates a new Caddy reverse proxy service.
func NewCaddyService(client *dagger.Client) *dagger.Service {
	caddyfile := client.Host().Directory(".").File("Caddyfile")

	return client.Container().From("caddy:2.8").
		WithMountedFile("/etc/caddy/Caddyfile", caddyfile).
		WithMountedCache("/data", client.CacheVolume("caddy_data")).
		WithMountedCache("/config", client.CacheVolume("caddy_config")).
		AsService()
}

// NewPostgresService creates a new PostgreSQL service.
func NewPostgresService(client *dagger.Client) *dagger.Service {
	return client.Container().From("postgres:16-alpine").
		WithEnvVariable("POSTGRES_DB", "keycloak").
		WithEnvVariable("POSTGRES_USER", "keycloak").
		WithEnvVariable("POSTGRES_PASSWORD", "password").
		WithMountedCache("/var/lib/postgresql/data", client.CacheVolume("postgres_data")).
		WithExposedPort(5432).
		AsService()
}

// NewKeycloakService creates the Keycloak service. It depends on Postgres.
func NewKeycloakService(client *dagger.Client, postgres *dagger.Service) *dagger.Service {
	providerSrc := client.Host().Directory("../acc-user-provider")

	return client.Container().From("quay.io/keycloak/keycloak:25.0").
		WithMountedDirectory("/opt/keycloak/providers", providerSrc).
		WithServiceBinding("postgres", postgres).
		WithEnvVariable("KC_DB", "postgres").
		WithEnvVariable("KC_DB_URL_HOST", "postgres").
		WithEnvVariable("KC_DB_USERNAME", "keycloak").
		WithEnvVariable("KC_DB_PASSWORD", "password").
		WithEnvVariable("KEYCLOAK_ADMIN", "admin").
		WithEnvVariable("KEYCLOAK_ADMIN_PASSWORD", "admin").
		WithEnvVariable("KC_HEALTH_ENABLED", "true").
		WithExec([]string{"/opt/keycloak/bin/kc.sh", "start-dev", "--import-realm"}).
		WithExposedPort(8080).
		AsService()
}
