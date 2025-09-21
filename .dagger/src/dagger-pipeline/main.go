package main

import (
	"context"
	"os"

	"dagger.io/dagger"
)

// DaggerPipeline is the main struct for the CI pipeline.
type DaggerPipeline struct{}

// Ci is the main entry point for the CI pipeline.
func (m *DaggerPipeline) Ci(ctx context.Context) (string, error) {
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stderr))
	if err != nil {
		return "", err
	}
	defer client.Close()

	sonarToken := client.Host().EnvVariable("SONAR_TOKEN").Secret()

	// Spring Boot
	springBoot := m.SpringBoot(client)
	_, err = springBoot.Scan(ctx, sonarToken)
	if err != nil {
		return "", err
	}

	// Nuxt.js
	nuxtJs := m.NuxtJs(client)
	_, err = nuxtJs.Scan(ctx, sonarToken)
	if err != nil {
		return "", err
	}

	return "CI pipeline finished successfully", nil
}

// SpringBoot returns a new SpringBoot component.
func (m *DaggerPipeline) SpringBoot(client *dagger.Client) *SpringBoot {
	return &SpringBoot{
		Client: client,
		Src:    client.Host().Directory("./spring-boot-app"),
	}
}

// SpringBoot is a component for building and scanning the Spring Boot application.
type SpringBoot struct {
	Client *dagger.Client
	Src    *dagger.Directory
}

func (sb *SpringBoot) builder() *dagger.Container {
	mavenCache := sb.Client.CacheVolume("maven-cache")
	return sb.Client.Container().From("maven:3.9.8-eclipse-temurin-21").
		WithMountedCache("/root/.m2", mavenCache).
		WithMountedDirectory("/app", sb.Src).
		WithWorkdir("/app")
}

// Build builds the Spring Boot application and returns a container with the application.
func (sb *SpringBoot) Build(ctx context.Context) (*dagger.Container, error) {
	builder := sb.builder().WithExec([]string{"mvn", "clean", "package"})
	app := sb.Client.Container().From("eclipse-temurin:21-jre-alpine").
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
func (m *DaggerPipeline) NuxtJs(client *dagger.Client) *NuxtJs {
	return &NuxtJs{
		Client: client,
		Src:    client.Host().Directory("./nuxt-app"),
	}
}

// NuxtJs is a component for building and scanning the Nuxt.js application.
type NuxtJs struct {
	Client *dagger.Client
	Src    *dagger.Directory
}

func (nj *NuxtJs) builder() *dagger.Container {
	pnpmCache := nj.Client.CacheVolume("pnpm-cache")
	return nj.Client.Container().From("node:22.19.0-slim").
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
