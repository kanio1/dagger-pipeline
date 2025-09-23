package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"dagger.io/dagger"
)

func main() {
	ctx := context.Background()

	// --- CLI Setup ---
	withTesting := flag.Bool("with-testing", false, "Run Playwright tests instead of starting the environment.")
	withMonitoring := flag.Bool("with-monitoring", false, "Include the Spring Boot Admin monitor.")
	flag.Parse()

	// --- Dagger Client ---
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		panic(err)
	}
	defer client.Close()

	// --- Service Definitions ---
	var adminSvc *dagger.Service
	if *withMonitoring {
		adminSvc = NewSpringBootAdminService(client)
	}

	postgres := NewPostgresService(client)
	keycloak := NewKeycloakService(client, postgres)
	backend := NewBackendService(client, adminSvc) // Pass optional admin service
	frontend := NewFrontendService(client)

	// --- Terminal Service (Caddy) ---
	// Caddy binds all other services, making them available by hostname.
	caddySvc := NewCaddyService(client).
		WithServiceBinding("postgres", postgres).
		WithServiceBinding("keycloak", keycloak).
		WithServiceBinding("backend", backend).
		WithServiceBinding("frontend", frontend)

	if adminSvc != nil {
		caddySvc = caddySvc.WithServiceBinding("admin", adminSvc)
	}

	// --- Execution Logic ---
	if *withTesting {
		fmt.Println("Running Playwright tests...")
		test := RunPlaywrightTests(client, caddySvc)
		out, err := test.Stdout(ctx)
		if err != nil {
			log.Fatalf("Failed to get test output: %v", err)
		}
		fmt.Println(out)

		exitCode, err := test.ExitCode(ctx)
		if err != nil {
			log.Fatalf("Failed to get test exit code: %v", err)
		}
		if exitCode != 0 {
			log.Fatalf("Tests failed with exit code %d", exitCode)
		}
		fmt.Println("Tests passed successfully!")
		return
	}

	// Default: bring up the full environment
	fmt.Println("Starting all services... Environment will be available at https://localhost")
	if *withMonitoring {
		fmt.Println("Spring Boot Admin will be available at https://localhost/admin/")
	}

	// The Up call on the terminal service is blocking and will keep the environment running.
	if err := caddySvc.Up(ctx, dagger.ServiceUpOpts{
		Ports: []dagger.PortForward{
			{Frontend: 80, Backend: 80},
			{Frontend: 443, Backend: 443},
		},
	}); err != nil {
		log.Fatalf("Failed to bring up services: %v", err)
	}
}
