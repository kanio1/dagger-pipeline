package main

import "dagger.io/dagger"

// NewBackendService creates the Spring Boot backend service.
// It now accepts an optional, nillable admin service for monitoring.
func NewBackendService(client *dagger.Client, admin *dagger.Service) *dagger.Service {
	src := client.Host().Directory("../backend-app")
	mavenCache := client.CacheVolume("maven-cache")

	ctr := client.Container().From("maven:3.9.8-eclipse-temurin-21").
		WithMountedDirectory("/app", src).
		WithWorkdir("/app").
		WithMountedCache("/root/.m2", mavenCache).
		WithEnvVariable("SPRING_SECURITY_OAUTH2_RESOURCESERVER_JWT_ISSUER-URI", "https://localhost/realms/your-realm")

	// Conditionally add the Spring Boot Admin configuration if the service is provided.
	if admin != nil {
		ctr = ctr.
			WithServiceBinding("admin", admin).
			WithEnvVariable("SPRING_BOOT_ADMIN_CLIENT_URL", "http://admin:8080")
	}

	return ctr.
		WithExposedPort(8080).
		WithExec([]string{"mvn", "spring-boot:run"}).
		AsService()
}
