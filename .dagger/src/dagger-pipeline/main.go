package main

import (
	"context"
	"sync"

	"dagger.io/dagger"
	"go.uber.org/multierr"
)

// DaggerPipeline is the main struct for the CI pipeline.
type DaggerPipeline struct{}

// Ci is the main entry point for the CI pipeline.
func (m *DaggerPipeline) Ci(ctx context.Context) (string, error) {
	sonarToken := dag.Host().EnvVariable("SONAR_TOKEN").Secret()

	var (
		wg      sync.WaitGroup
		allErrs error
		mu      sync.Mutex // to protect allErrs
	)

	wg.Add(2)

	// Run Spring Boot scan in a goroutine
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

	// Run Nuxt.js scan in a goroutine
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

	wg.Wait()

	if allErrs != nil {
		return "", allErrs
	}

	return "CI pipeline finished successfully", nil
}

// SpringBoot returns a new SpringBoot component.
func (m *DaggerPipeline) SpringBoot() *SpringBoot {
	return &SpringBoot{
		Src: dag.Host().Directory("./spring-boot-app"),
	}
}

// SpringBoot is a component for building and scanning the Spring Boot application.
type SpringBoot struct {
	Src *dagger.Directory
}

func (sb *SpringBoot) builder() *dagger.Container {
	mavenCache := dag.CacheVolume("maven-cache")
	return dag.Container().From("maven:3.9.8-eclipse-temurin-21").
		WithMountedCache("/root/.m2", mavenCache).
		WithMountedDirectory("/app", sb.Src).
		WithWorkdir("/app")
}

// Build builds the Spring Boot application and returns a container with the application.
func (sb *SpringBoot) Build(ctx context.Context) (*dagger.Container, error) {
	builder := sb.builder().WithExec([]string{"mvn", "clean", "package"})
	app := dag.Container().From("eclipse-temurin:21-jre-alpine").
		WithDirectory("/app", builder.Directory("/app/target")).
		WithWorkdir("/app").
		WithExposedPort(8080).
		WithEntrypoint([]string{"java", "-jar", "demo-0.0.1-SNAPSHOT.jar"})
	return app, nil
}

// Scan scans the Spring Boot application using SonarCloud.
func (sb *SpringBoot) Scan(ctx context.Context, sonarToken *dagger.Secret) (string, error) {
	scanner := sb.builder().
		WithSecretVariable("SONAR_TOKEN", sonarToken).
		WithExec([]string{"mvn", "verify", "-Psonar", "sonar:sonar", "-Dsonar.login=${SONAR_TOKEN}"})
	return scanner.Stdout(ctx)
}

// NuxtJs returns a new NuxtJs component.
func (m *DaggerPipeline) NuxtJs() *NuxtJs {
	return &NuxtJs{
		Src: dag.Host().Directory("./nuxt-app"),
	}
}

// NuxtJs is a component for building and scanning the Nuxt.js application.
type NuxtJs struct {
	Src *dagger.Directory
}

func (nj *NuxtJs) builder() *dagger.Container {
	pnpmCache := dag.CacheVolume("pnpm-cache")
	return dag.Container().From("node:22.19.0-slim").
		WithExec([]string{"corepack", "enable"}).
		WithMountedCache("/root/.local/share/pnpm/store", pnpmCache).
		WithMountedDirectory("/app", nj.Src).
		WithWorkdir("/app").
		WithExec([]string{"pnpm", "install"})
}

// Build builds the Nuxt.js application and returns the build artifacts.
func (nj *NuxtJs) Build(ctx context.Context) (*dagger.Directory, error) {
	return nj.builder().WithExec([]string{"pnpm", "build"}).Directory("/app/.output"), nil
}

// Scan scans the Nuxt.js application using SonarCloud.
func (nj *NuxtJs) Scan(ctx context.Context, sonarToken *dagger.Secret) (string, error) {
	scanner := nj.builder().
		WithSecretVariable("SONAR_TOKEN", sonarToken).
		WithExec([]string{"pnpm", "run", "sonar:scan"})
	return scanner.Stdout(ctx)
}
