// This package is a Dagger module for a comprehensive CI/CD pipeline.
//
// It provides functionalities to build, test, and scan a Spring Boot application
// and a Nuxt.js application in parallel. The pipeline is designed to be modular,
// reusable, and efficient, leveraging Dagger's caching capabilities.
package main

import (
	"context"
	"sync"

	"dagger.io/dagger"
	"go.uber.org/multierr"
)

// DaggerPipeline is the main struct for the CI pipeline. It serves as the top-level
// entry point for all pipeline operations.
type DaggerPipeline struct{}

// Ci runs the full continuous integration pipeline.
//
// It executes the SonarCloud scans for both the Spring Boot and Nuxt.js applications
// in parallel to optimize for speed. If any of the scans fail, it aggregates the
// errors and returns them all at the end, providing a complete report of the pipeline status.
//
// This function requires a SONAR_TOKEN environment variable to be set on the host
// for authenticating with SonarCloud.
//
// Example usage from the command line:
//   export SONAR_TOKEN="your_sonar_cloud_token"
//   dagger call ci
func (m *DaggerPipeline) Ci(ctx context.Context) (string, error) {
	// It's a Dagger best practice to get secrets from the host environment.
	// This prevents secrets from being hardcoded in the pipeline.
	sonarToken := dag.Host().EnvVariable("SONAR_TOKEN").Secret()

	var (
		wg      sync.WaitGroup
		allErrs error
		mu      sync.Mutex // A mutex is used to safely write to the allErrs variable from multiple goroutines.
	)

	// We use a WaitGroup to run the scans for both applications in parallel.
	wg.Add(2)

	// Run Spring Boot scan in a goroutine for parallel execution.
	go func() {
		defer wg.Done()
		springBoot := m.SpringBoot()
		_, err := springBoot.Scan(ctx, sonarToken)
		if err != nil {
			mu.Lock()
			allErrs = multierr.Append(allErrs, err)
			mu.Unlock()
		}
	}()

	// Run Nuxt.js scan in a goroutine for parallel execution.
	go func() {
		defer wg.Done()
		nuxtJs := m.NuxtJs()
		_, err := nuxtJs.Scan(ctx, sonarToken)
		if err != nil {
			mu.Lock()
			allErrs = multierr.Append(allErrs, err)
			mu.Unlock()
		}
	}()

	// Wait for all parallel jobs to complete.
	wg.Wait()

	if allErrs != nil {
		return "", allErrs
	}

	return "CI pipeline finished successfully", nil
}

// SpringBoot returns a new SpringBoot component.
//
// This function acts as a factory for creating a new SpringBoot component,
// which encapsulates all the logic for handling the Spring Boot application.
func (m *DaggerPipeline) SpringBoot() *SpringBoot {
	return &SpringBoot{
		// Get a reference to the source code of the Spring Boot application on the host.
		Src: dag.Host().Directory("./spring-boot-app"),
	}
}

// SpringBoot is a component for building and scanning the Spring Boot application.
//
// It encapsulates the source code and provides methods for common CI tasks
// related to the Spring Boot application.
type SpringBoot struct {
	// Src holds a reference to the source code of the Spring Boot application.
	Src *dagger.Directory
}

// builder sets up a base Maven container for building and scanning.
//
// This private helper method creates a container with the correct Maven and Java versions,
// mounts the application source code, and configures a cache for Maven dependencies.
// This ensures that dependencies are not re-downloaded on every run, speeding up the pipeline.
//
// Returns:
//   A configured Dagger container ready for building or scanning.
func (sb *SpringBoot) builder() *dagger.Container {
	// Use a named cache volume to persist the Maven repository between pipeline runs.
	mavenCache := dag.CacheVolume("maven-cache")
	return dag.Container().From("maven:3.9.8-eclipse-temurin-21").
		WithMountedCache("/root/.m2", mavenCache).
		WithMountedDirectory("/app", sb.Src).
		WithWorkdir("/app")
}

// Build builds the Spring Boot application and returns a runnable OCI container.
//
// It uses a multi-stage build approach. First, it builds the application using Maven,
// then it creates a new, minimal JRE container and copies only the built JAR file into it.
// This results in a small, secure, and production-ready container image.
//
// Returns:
//   A Dagger container with the built Spring Boot application.
func (sb *SpringBoot) Build(ctx context.Context) (*dagger.Container, error) {
	builder := sb.builder().WithExec([]string{"mvn", "clean", "package"})
	// The final application container is built from a minimal JRE image.
	app := dag.Container().From("eclipse-temurin:21-jre-alpine").
		WithDirectory("/app", builder.Directory("/app/target")).
		WithWorkdir("/app").
		WithExposedPort(8080).
		WithEntrypoint([]string{"java", "-jar", "demo-0.0.1-SNAPSHOT.jar"})
	return app, nil
}

// Scan runs a SonarCloud scan on the Spring Boot application.
//
// It uses the same base builder as the Build method to ensure a consistent environment.
// The SonarCloud analysis is triggered by running the `verify` and `sonar:sonar` Maven goals.
//
// It requires a SonarCloud token, which is passed as a Dagger secret to ensure it's not exposed in logs.
//
// Parameters:
//   sonarToken: A Dagger secret containing the SonarCloud authentication token.
//
// Returns:
//   The stdout from the SonarCloud scanner.
func (sb *SpringBoot) Scan(ctx context.Context, sonarToken *dagger.Secret) (string, error) {
	scanner := sb.builder().
		// The SonarCloud token is securely passed to the container as a secret environment variable.
		WithSecretVariable("SONAR_TOKEN", sonarToken).
		WithExec([]string{"mvn", "verify", "-Psonar", "sonar:sonar", "-Dsonar.login=${SONAR_TOKEN}"})
	return scanner.Stdout(ctx)
}

// NuxtJs returns a new NuxtJs component.
//
// This function acts as a factory for creating a new NuxtJs component,
// which encapsulates all the logic for handling the Nuxt.js application.
func (m *DaggerPipeline) NuxtJs() *NuxtJs {
	return &NuxtJs{
		// Get a reference to the source code of the Nuxt.js application on the host.
		Src: dag.Host().Directory("./nuxt-app"),
	}
}

// NuxtJs is a component for building and scanning the Nuxt.js application.
//
// It encapsulates the source code and provides methods for common CI tasks
// related to the Nuxt.js application.
type NuxtJs struct {
	// Src holds a reference to the source code of the Nuxt.js application.
	Src *dagger.Directory
}

// builder sets up a base Node.js container for building and scanning.
//
// This private helper method creates a container with the correct Node.js version,
// enables corepack for pnpm support, mounts the application source code, and configures
// a cache for pnpm dependencies.
//
// Returns:
//   A configured Dagger container ready for building or scanning.
func (nj *NuxtJs) builder() *dagger.Container {
	// Use a named cache volume to persist the pnpm store between pipeline runs.
	pnpmCache := dag.CacheVolume("pnpm-cache")
	return dag.Container().From("node:22.19.0-slim").
		WithExec([]string{"corepack", "enable"}).
		WithMountedCache("/root/.local/share/pnpm/store", pnpmCache).
		WithMountedDirectory("/app", nj.Src).
		WithWorkdir("/app").
		// We run pnpm install in the builder to ensure all dependencies are available
		// for subsequent build and scan steps.
		WithExec([]string{"pnpm", "install"})
}

// Build builds the Nuxt.js application and returns the build artifacts.
//
// It reuses the base builder and runs the `pnpm build` command.
//
// Returns:
//   A Dagger directory containing the built Nuxt.js application artifacts.
func (nj *NuxtJs) Build(ctx context.Context) (*dagger.Directory, error) {
	return nj.builder().WithExec([]string{"pnpm", "build"}).Directory("/app/.output"), nil
}

// Scan scans the Nuxt.js application using SonarCloud.
//
// It reuses the base builder and runs the `sonar:scan` script from `package.json`.
//
// It requires a SonarCloud token, which is passed as a Dagger secret.
//
// Parameters:
//   sonarToken: A Dagger secret containing the SonarCloud authentication token.
//
// Returns:
//   The stdout from the SonarCloud scanner.
func (nj *NuxtJs) Scan(ctx context.Context, sonarToken *dagger.Secret) (string, error) {
	scanner := nj.builder().
		// The SonarCloud token is securely passed to the container as a secret environment variable.
		WithSecretVariable("SONAR_TOKEN", sonarToken).
		WithExec([]string{"pnpm", "run", "sonar:scan"})
	return scanner.Stdout(ctx)
}
