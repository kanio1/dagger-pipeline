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

// The default git repository URL.
// PLEASE REPLACE THIS with your actual Gerrit repository URL.
const DefaultGitURL = "ssh://gerrit.example.com/your-repo.git"

// Checkout clones the git repository using SSH.
//
// It uses the host's SSH agent to authenticate with the git server.
// By default, it checks out the "develop" branch.
//
// Parameters:
//   url: The SSH URL of the git repository to clone. Defaults to DefaultGitURL.
//
// Returns:
//   A Dagger directory containing the checked-out source code.
func (m *DaggerPipeline) Checkout(
	ctx context.Context,
	// +optional
	url string,
) (*dagger.Directory, error) {
	if url == "" {
		url = DefaultGitURL
	}
	// The SSH_AUTH_SOCK environment variable is automatically used by Dagger
	// to forward the host's SSH agent to the container.
	// We pass it as a secret to ensure it's handled securely.
	sshSocket := dag.Host().EnvVariable("SSH_AUTH_SOCK").Secret()
	return dag.Git(url, dagger.GitOpts{
		SSHAuthSocket: sshSocket,
	}).Branch("develop").Tree(), nil
}

// Ci runs the full continuous integration pipeline.
//
// It first checks out the source code from git, then executes the SonarCloud scans
// for both the Spring Boot and Nuxt.js applications in parallel.
//
// This function requires a SONAR_TOKEN environment variable to be set on the host
// for authenticating with SonarCloud, and a running SSH agent for git checkout.
//
// Example usage from the command line:
//   export SONAR_TOKEN="your_sonar_cloud_token"
//   dagger call ci
func (m *DaggerPipeline) Ci(ctx context.Context) (string, error) {
	// Checkout the source code first.
	src, err := m.Checkout(ctx, "")
	if err != nil {
		return "", err
	}

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
		// Pass the checked-out source code to the SpringBoot component.
		springBoot := m.SpringBoot(src)
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
		// Pass the checked-out source code to the NuxtJs component.
		nuxtJs := m.NuxtJs(src)
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
func (m *DaggerPipeline) SpringBoot(src *dagger.Directory) *SpringBoot {
	return &SpringBoot{
		// Get a reference to the source code of the Spring Boot application from the provided directory.
		Src: src.Directory("./spring-boot-app"),
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
func (m *DaggerPipeline) NuxtJs(src *dagger.Directory) *NuxtJs {
	return &NuxtJs{
		// Get a reference to the source code of the Nuxt.js application from the provided directory.
		Src: src.Directory("./nuxt-app"),
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

// builder creates a base Node.js container with dependencies installed.
//
// This function optimizes caching by separating dependency installation from
// source code mounting. It only mounts the 'package.json' and 'pnpm-lock.yaml' files,
// runs 'pnpm install', and then returns a container with the node_modules directory
// fully cached. This container can be used as a base for subsequent build and scan steps.
//
// This is a Dagger best practice because this step will only be re-run if the
// dependency files change, not on every source code change.
//
// Returns:
//   A Dagger container with all Node.js dependencies pre-installed.
func (nj *NuxtJs) builder() *dagger.Container {
	// Use a named cache volume to persist the pnpm store between pipeline runs.
	pnpmCache := dag.CacheVolume("pnpm-cache")

	// Create a base container with only the necessary dependency files.
	// By explicitly selecting only package.json and pnpm-lock.yaml, we ensure
	// that the dependency installation step is only re-run when these files change.
	deps := dag.Container().From("node:22.19.0-slim").
		WithExec([]string{"corepack", "enable"}).
		WithMountedCache("/root/.local/share/pnpm/store", pnpmCache).
		WithWorkdir("/app").
		WithFile("/app/package.json", nj.Src.File("package.json")).
		WithFile("/app/pnpm-lock.yaml", nj.Src.File("pnpm-lock.yaml")).
		WithExec([]string{"pnpm", "install"})

	// Mount the rest of the source code onto the container with pre-installed dependencies.
	// This creates the final builder container for running build or scan commands.
	return deps.WithMountedDirectory("/app", nj.Src)
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
