package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"dagger.io/dagger"
	"gopkg.in/yaml.v3"
)

// Main CLI logic
func main() {
	// Subkomendy
	generateCertsCmd := flag.NewFlagSet("generate-certs", flag.ExitOnError)
	generateComposeCmd := flag.NewFlagSet("generate-compose", flag.ExitOnError)

	// Flagi dla subkomendy generate-compose
	backendPath := generateComposeCmd.String("backend-path", "../backend-app", "Ścieżka do kodu źródłowego backendu")
	frontendPath := generateComposeCmd.String("frontend-path", "../frontend-app", "Ścieżka do kodu źródłowego frontendu")
	providerPath := generateComposeCmd.String("provider-path", "../acc-user-provider", "Ścieżka do katalogu z providerem Keycloak")
	playwrightPath := generateComposeCmd.String("playwright-path", "../playwright-tests", "Ścieżka do testów Playwright")
	backendLogLevel := generateComposeCmd.String("backend-log-level", "INFO", "Poziom logowania dla backendu (np. DEBUG, INFO)")
	keycloakLogLevel := generateComposeCmd.String("keycloak-log-level", "INFO", "Poziom logowania dla Keycloak (np. DEBUG, INFO)")

	if len(os.Args) < 2 {
		log.Fatal("Oczekiwano subkomendy 'generate-certs' lub 'generate-compose'")
	}

	ctx := context.Background()
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		panic(err)
	}
	defer client.Close()

	switch os.Args[1] {
	case "generate-certs":
		generateCertsCmd.Parse(os.Args[2:])
		if err := generateCerts(ctx, client); err != nil {
			log.Fatalf("Błąd podczas generowania certyfikatów: %v", err)
		}
		fmt.Println("Certyfikaty zostały pomyślnie wygenerowane w ./.local/certs/")
	case "generate-compose":
		generateComposeCmd.Parse(os.Args[2:])
		config := &ComposeConfig{
			Paths: &ComposePaths{
				Backend:    *backendPath,
				Frontend:   *frontendPath,
				Provider:   *providerPath,
				Playwright: *playwrightPath,
			},
			LogLevels: &LogLevels{
				Backend:  *backendLogLevel,
				Keycloak: *keycloakLogLevel,
			},
		}
		if err := generateCompose(config); err != nil {
			log.Fatalf("Błąd podczas generowania plików konfiguracyjnych: %v", err)
		}
		fmt.Println("Pliki docker-compose.yaml i nginx.conf zostały pomyślnie wygenerowane.")
	default:
		log.Fatalf("Nieznana komenda: %s. Użyj 'generate-certs' lub 'generate-compose'.", os.Args[1])
	}
}

// generateCerts uses a mkcert container to create locally trusted certificates.
func generateCerts(ctx context.Context, client *dagger.Client) error {
	outputDir := "./.local/certs"
	caCache := client.CacheVolume("mkcert-ca")

	certGenerator := client.Container().From("ldez/mkcert:v1.4.4").
		WithMountedCache("/root/.local/share/mkcert", caCache).
		WithWorkdir("/work").
		WithExec([]string{"mkcert", "-install"}).
		WithExec([]string{"mkcert", "localhost", "127.0.0.1", "::1"})

	_, err := certGenerator.Directory("/work").Export(ctx, outputDir)
	if err != nil {
		return fmt.Errorf("nie udało się wyeksportować certyfikatów: %w", err)
	}
	return nil
}

type ComposePaths struct {
	Backend, Frontend, Provider, Playwright string
}
type LogLevels struct {
	Backend, Keycloak string
}
type ComposeConfig struct {
	Paths     *ComposePaths
	LogLevels *LogLevels
}

// generateCompose creates the docker-compose.yaml and nginx.conf files.
func generateCompose(config *ComposeConfig) error {
	if err := generateNginxConf(); err != nil {
		return err
	}

	compose := DockerCompose{
		Version: "3.9",
		Services: map[string]Service{
			"postgres": {
				Image:       "postgres:16-alpine",
				Environment: map[string]string{"POSTGRES_DB": "keycloak", "POSTGRES_USER": "keycloak", "POSTGRES_PASSWORD": "password"},
				Volumes:     []string{"postgres_data:/var/lib/postgresql/data"},
				Networks:    []string{"dev-net"},
				HealthCheck: &HealthCheck{Test: []string{"CMD-SHELL", "pg_isready -U keycloak -d keycloak"}, Interval: "10s", Timeout: "5s", Retries: 5},
			},
			"keycloak": {
				Image:   "quay.io/keycloak/keycloak:25.0",
				Command: "start-dev --import-realm",
				Environment: map[string]string{
					"KC_DB": "postgres", "KC_DB_URL_HOST": "postgres", "KC_DB_USERNAME": "keycloak", "KC_DB_PASSWORD": "password",
					"KEYCLOAK_ADMIN": "admin", "KEYCLOAK_ADMIN_PASSWORD": "admin", "KC_LOG_LEVEL": config.LogLevels.Keycloak,
				},
				Volumes:   []string{fmt.Sprintf("%s:/opt/keycloak/providers", config.Paths.Provider)},
				Networks:  []string{"dev-net"},
				DependsOn: map[string]any{"postgres": map[string]string{"condition": "service_healthy"}},
			},
			"backend": {
				Image:      "maven:3.9.8-eclipse-temurin-21",
				WorkingDir: "/app",
				Command:    "mvn spring-boot:run",
				Environment: map[string]string{
					"LOGGING_LEVEL_ROOT": config.LogLevels.Backend,
					"SPRING_SECURITY_OAUTH2_RESOURCESERVER_JWT_ISSUER-URI": "https://localhost/realms/your-realm",
					"SPRING_BOOT_ADMIN_CLIENT_URL":                         "http://spring-boot-admin:9090",
					"MANAGEMENT_ENDPOINTS_WEB_EXPOSURE_INCLUDE":            "*",
				},
				Volumes:  []string{fmt.Sprintf("%s:/app", config.Paths.Backend), "maven-cache:/root/.m2"},
				Networks: []string{"dev-net"},
			},
			"frontend": {
				Image:      "node:22.19.0", // Changed from -alpine
				WorkingDir: "/app",
				Command:    "sh -c 'corepack enable && pnpm install --frozen-lockfile && pnpm run dev --host 0.0.0.0'",
				Volumes:    []string{fmt.Sprintf("%s:/app", config.Paths.Frontend), "pnpm-cache:/root/.local/share/pnpm/store/v3"},
				Networks:   []string{"dev-net"},
			},
			"nginx": {
				Image:     "nginx:1.27-alpine",
				Ports:     []string{"80:80", "443:443"},
				Volumes:   []string{"./nginx.conf:/etc/nginx/nginx.conf", "./.local/certs:/etc/nginx/certs"},
				Networks:  []string{"dev-net"},
				DependsOn: []string{"backend", "frontend"},
			},
			"spring-boot-admin": {
				Image:    "codecentric/spring-boot-admin-server:3.3.1",
				Ports:    []string{"9090:9090"},
				Networks: []string{"dev-net"},
				Profiles: []string{"monitoring"},
			},
			"playwright": {
				Image:      "mcr.microsoft.com/playwright:v1.44.0-jammy",
				WorkingDir: "/app",
				Command:    "pnpm test --base-url=https://nginx",
				Volumes:    []string{fmt.Sprintf("%s:/app", config.Paths.Playwright)},
				Networks:   []string{"dev-net"},
				Profiles:   []string{"testing"},
			},
		},
		Volumes:  map[string]any{"postgres_data": nil, "maven-cache": nil, "pnpm-cache": nil},
		Networks: map[string]any{"dev-net": nil},
	}

	yamlData, err := yaml.Marshal(&compose)
	if err != nil {
		return fmt.Errorf("nie udało się zserializować danych do YAML: %w", err)
	}

	return os.WriteFile("docker-compose.yaml", yamlData, 0644)
}

func generateNginxConf() error { /* ... function unchanged ... */
	nginxConf := `
worker_processes 1;
events { worker_connections 1024; }
http {
    upstream backend { server backend:8080; }
    upstream frontend { server frontend:3000; }
    server {
        listen 80;
        server_name localhost;
        return 301 https://$host$request_uri;
    }
    server {
        listen 443 ssl;
        server_name localhost;
        ssl_certificate /etc/nginx/certs/localhost.pem;
        ssl_certificate_key /etc/nginx/certs/localhost-key.pem;
        location / {
            proxy_pass http://frontend;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            proxy_http_version 1.1;
            proxy_set_header Upgrade $http_upgrade;
            proxy_set_header Connection "upgrade";
        }
        location /api/ {
            proxy_pass http://backend/api/;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
        }
    }
}`
	return os.WriteFile("nginx.conf", []byte(nginxConf), 0644)
}

// --- YAML Structs ---
type DockerCompose struct {
	Version  string             `yaml:"version"`
	Services map[string]Service `yaml:"services"`
	Volumes  map[string]any     `yaml:"volumes,omitempty"`
	Networks map[string]any     `yaml:"networks,omitempty"`
}
type Service struct {
	Image       string            `yaml:"image,omitempty"`
	WorkingDir  string            `yaml:"working_dir,omitempty"`
	Command     string            `yaml:"command,omitempty"`
	Ports       []string          `yaml:"ports,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty"`
	Volumes     []string          `yaml:"volumes,omitempty"`
	Networks    []string          `yaml:"networks,omitempty"`
	DependsOn   any               `yaml:"depends_on,omitempty"`
	HealthCheck *HealthCheck      `yaml:"healthcheck,omitempty"`
	Profiles    []string          `yaml:"profiles,omitempty"`
}
type HealthCheck struct {
	Test     []string `yaml:"test"`
	Interval string   `yaml:"interval,omitempty"`
	Timeout  string   `yaml:"timeout,omitempty"`
	Retries  int      `yaml:"retries,omitempty"`
}
