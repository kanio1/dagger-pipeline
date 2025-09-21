# Dagger Pipeline Guide

## 1. Overview

This guide explains how to use our Dagger-based CI/CD pipeline. The pipeline is written in Go and is designed to build, test, and scan our Spring Boot and Nuxt.js applications.

It provides a set of modular and reusable functions that can be chained together to perform various development and CI tasks.

## 2. Prerequisites

Before using the pipeline, make sure you have the following installed:

*   **Dagger CLI:** [Installation Guide](https://docs.dagger.io/cli/465057/install)
*   **Docker:** The Dagger engine runs on Docker.

For running SonarCloud scans, you must have a `SONAR_TOKEN` environment variable set:

```shell
export SONAR_TOKEN="your_sonar_cloud_token"
```

## 3. Pipeline Structure

The pipeline is organized into modular components to ensure reusability and clarity.

*   `DaggerPipeline`: The main entry point for the pipeline. It contains the top-level `ci` function and factory methods for the application components.
*   `SpringBoot`: A component dedicated to the Spring Boot application. It contains methods to `build` and `scan` the app.
*   `NuxtJs`: A component dedicated to the Nuxt.js application, also with its own `build` and `scan` methods.

This structure allows you to interact with each part of the pipeline independently.

## 4. How to Use the Pipeline (7 Examples)

All commands should be run from the root of the repository.

### Example 1: Run the Full CI Pipeline

This is the main function to simulate a full CI run. It builds and scans both applications in parallel.

```shell
dagger call ci
```

### Example 2: Build Only the Spring Boot Application

This command builds the Spring Boot application and produces a runnable OCI container. This is useful for development when you only need to build one part of the system.

```shell
dagger call spring-boot build
```

### Example 3: Scan Only the Spring Boot Application

This command runs the SonarCloud scan for the Spring Boot app. It requires the `SONAR_TOKEN` to be set.

```shell
dagger call spring-boot scan
```

### Example 4: Build Only the Nuxt.js Application

This command builds the Nuxt.js application and returns the build artifacts from the `.output` directory.

```shell
dagger call nuxt-js build
```

### Example 5: Scan Only the Nuxt.js Application

This command runs the SonarCloud scan for the Nuxt.js app. It requires the `SONAR_TOKEN` to be set.

```shell
dagger call nuxt-js scan
```

### Example 6: Build and Export the Spring Boot Container

This example shows how to chain commands to export the built container image to your local filesystem. You can then load it into Docker to run it locally.

```shell
# 1. Build and export the container to a file
dagger call spring-boot build export --path build/spring-boot-app.tar

# 2. Load the image into Docker
docker load -i build/spring-boot-app.tar

# 3. Run the container (replace with the actual image name/ID from the load output)
docker run -p 8080:8080 your-image-name:tag
```

### Example 7: Build and Export the Nuxt.js Static Assets

This example shows how to get the production-ready static files from the Nuxt.js build.

```shell
# 1. Build the app and export the contents of the .output directory
dagger call nuxt-js build export --path ./nuxt-dist

# 2. The `nuxt-dist` directory now contains your built application.
ls ./nuxt-dist
```
