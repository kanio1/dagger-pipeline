package main

import "dagger.io/dagger"

// NewSpringBootAdminService creates the Spring Boot Admin monitoring service.
func NewSpringBootAdminService(client *dagger.Client) *dagger.Service {
	return client.Container().From("codecentric/spring-boot-admin-server:3.3.1").
		WithExposedPort(8080).
		AsService()
}
