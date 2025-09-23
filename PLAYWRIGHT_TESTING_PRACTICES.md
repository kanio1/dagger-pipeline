# Profesjonalne Metody Uruchamiania Testów Playwright w Kontenerze

Poniższy dokument przedstawia analizę i trzy profesjonalne, zgodne z dobrymi praktykami, alternatywy dla uruchamiania testów Playwright w kontenerze, wykraczające poza osadzanie logiki w pliku `docker-compose.yml`.

### Analiza Problemu

Obecne rozwiązanie w `docker-compose.yml` używa `sh -c "if ..."` do warunkowego uruchamiania testów.
```yaml
command: >
  sh -c "
    if [ \"$HEADED_MODE\" = \"true\" ]; then
      pnpm run test:e2e --headed;
    else
      pnpm run test:e2e;
    fi
  "
```
Chociaż to działa, osadzanie skryptów shellowych w plikach YAML ma swoje wady:
*   **Niska czytelność:** Skomplikowana logika jest trudna do odczytania i debugowania.
*   **Problemy z formatowaniem:** Wymaga ucieczki znaków (np. `\"`), co jest podatne na błędy.
*   **Słaba rozszerzalność:** Dodanie kolejnych kroków (np. generowanie raportu, czyszczenie) staje się bardzo niepraktyczne.

Oto trzy profesjonalne alternatywy.

---

### Propozycja 1: Entrypoint Script w Obrazie Docker (Najbardziej zalecana)

Jest to najczęstsza i najbardziej rekomendowana praktyka. Przenosimy całą logikę startową do dedykowanego skryptu `.sh`, który staje się częścią obrazu Docker.

**Jak to działa:**
1.  Tworzymy skrypt `run-tests.sh` w katalogu `nuxt-app`.
2.  W skrypcie umieszczamy logikę `if/else` do obsługi trybu `headed`.
3.  Modyfikujemy `Dockerfile.playwright`, aby skopiował ten skrypt, nadał mu uprawnienia do wykonania (`chmod +x`) i ustawił go jako domyślne polecenie (`CMD` lub `ENTRYPOINT`).
4.  Znacząco upraszczamy plik `docker-compose.yml`.

**Implementacja:**

1.  **Stwórz plik `nuxt-app/run-tests.sh`:**
    ```sh
    #!/bin/sh
    # Zabezpieczenie: zakończ skrypt, jeśli którekolwiek polecenie się nie powiedzie
    set -e

    echo "Starting Playwright tests..."

    # Sprawdź zmienną środowiskową HEADED_MODE
    if [ "$HEADED_MODE" = "true" ]; then
      echo "Running in headed mode."
      # Użyj --, aby oddzielić opcje pnpm od opcji playwright
      pnpm run test:e2e -- --headed
    else
      echo "Running in headless mode."
      pnpm run test:e2e
    fi

    echo "Playwright tests finished."
    ```

2.  **Zmodyfikuj `nuxt-app/Dockerfile.playwright`:**
    ```dockerfile
    FROM mcr.microsoft.com/playwright:v1.45.0-jammy

    WORKDIR /app

    COPY package.json pnpm-lock.yaml* ./
    RUN pnpm install

    COPY . .

    # Skopiuj skrypt i nadaj mu uprawnienia do wykonania
    COPY run-tests.sh .
    RUN chmod +x run-tests.sh

    # Ustaw skrypt jako domyślne polecenie
    CMD ["./run-tests.sh"]
    ```

3.  **Uprość `docker-compose.yml`:**
    Sekcja `command` w usłudze `playwright` zostaje całkowicie usunięta.
    ```yaml
    playwright:
      profiles:
        - playwright
      build:
        context: ./nuxt-app
        dockerfile: Dockerfile.playwright
      depends_on:
        - nuxt-app
      # ... reszta bez zmian
      # 'command' jest już zdefiniowane w obrazie Docker
    ```

---

### Propozycja 2: Rozbudowa Skryptów w `package.json`

To podejście wykorzystuje wbudowane możliwości menedżera pakietów (`pnpm`) do zarządzania logiką.

**Jak to działa:**
1.  Definiujemy w `package.json` dwa osobne skrypty: jeden dla trybu `headless`, drugi dla `headed`.
2.  W `docker-compose.yml` używamy zmiennej środowiskowej, aby dynamicznie wybrać, który skrypt `pnpm` ma zostać uruchomiony.

**Implementacja:**

1.  **Zmodyfikuj `nuxt-app/package.json`:**
    ```json
    "scripts": {
      "test:e2e": "playwright test",
      "test:e2e:headed": "playwright test --headed",
      // ... inne skrypty
    }
    ```

2.  **Zmodyfikuj `docker-compose.yml`:**
    Używamy zmiennej `E2E_SCRIPT` do określenia, który skrypt uruchomić.
    ```yaml
    playwright:
      # ...
      environment:
        # Domyślnie uruchamiamy skrypt headless
        - E2E_SCRIPT=${E2E_SCRIPT:-test:e2e}
      # Uruchamiamy skrypt zdefiniowany w zmiennej
      command: pnpm run ${E2E_SCRIPT}
    ```
    Aby uruchomić w trybie `headed`, wywołalibyśmy:
    `E2E_SCRIPT=test:e2e:headed docker compose --profile playwright up`

---

### Propozycja 3: Natywne Uruchomienie Testów w Dagger

To najbardziej zaawansowane i "Dagger-native" podejście. Zamiast polegać na `docker-compose` do uruchamiania testów, cały proces orkiestrujemy bezpośrednio w potoku Dagger.

**Jak to działa:**
1.  Całkowicie usuwamy profil `playwright` z `docker-compose.yml`. Służy on tylko do uruchamiania aplikacji.
2.  Tworzymy nową funkcję w potoku Dagger, np. `TestE2E`.
3.  W tej funkcji Dagger:
    a. Uruchamia usługę `nuxt-app` w tle.
    b. Buduje kontener Playwright (z tego samego `Dockerfile.playwright`).
    c. Łączy oba kontenery w tej samej sieci wirtualnej.
    d. Uruchamia testy z kontenera Playwright, kierując je na usługę `nuxt-app`.

**Implementacja (koncepcyjny fragment w `main.go`):**
```go
// ... (wewnątrz DaggerPipeline)

// TestE2E runs the Playwright end-to-end tests.
//
// Parameters:
//   headed: If true, runs the tests in headed mode.
func (m *DaggerPipeline) TestE2E(ctx context.Context,
    // +optional
    headed bool,
) (string, error) {
    // 1. Zbuduj i uruchom aplikację Nuxt jako usługę
    nuxtSvc := m.NuxtJs(m.Checkout(ctx, "")).BuildContainer().AsService()

    // 2. Zbuduj kontener Playwright
    playwrightCtr := m.NuxtJs(m.Checkout(ctx, "")).PlaywrightContainer()

    // 3. Dołącz usługę Nuxt do kontenera Playwright (aby były w tej samej sieci)
    playwrightCtr = playwrightCtr.WithServiceBinding("nuxt-app", nuxtSvc)

    // 4. Zbuduj i wykonaj polecenie testowe
    cmd := []string{"pnpm", "run", "test:e2e"}
    if headed {
        cmd = append(cmd, "--", "--headed")
    }

    return playwrightCtr.WithExec(cmd).Stdout(ctx)
}

// ... (potrzebna byłaby też funkcja PlaywrightContainer w NuxtJs)
```

### Podsumowanie i Rekomendacja

| Kryterium               | Propozycja 1 (Entrypoint Script)                    | Propozycja 2 (`package.json`)                    | Propozycja 3 (Natywnie w Dagger)                        |
| ----------------------- | --------------------------------------------------- | ------------------------------------------------ | ------------------------------------------------------- |
| **Separacja logiki**    | **Doskonała** (logika w `.sh`, konfiguracja w `.yml`) | Dobra (logika w `package.json`)                  | **Najlepsza** (wszystko w kodzie potoku)                |
| **Czytelność `yml`**      | **Najlepsza** (brak logiki)                         | Bardzo dobra                                     | Nie dotyczy (`docker-compose` nie jest używany do testów) |
| **Złożoność**           | Niska                                               | Bardzo niska                                     | Wysoka (wymaga dobrej znajomości Dagger)                 |
| **Integracja z CI/CD**  | Dobra                                               | Dobra                                            | **Najlepsza** (pełna kontrola i powtarzalność)          |

**Rekomendacja:**

*   **Najlepsza praktyka ogólnego zastosowania:** **Propozycja 1 (Entrypoint Script)**. Jest to złoty standard w świecie Dockera. Oddziela logikę od konfiguracji, jest łatwa w utrzymaniu i zrozumiała dla każdego, kto zna Dockera.
*   **Szybkie i proste rozwiązanie:** **Propozycja 2 (`package.json`)** jest również bardzo dobra, jeśli logika jest prosta (np. tylko wybór między dwoma poleceniami).
*   **Docelowe rozwiązanie dla CI/CD:** **Propozycja 3 (Natywnie w Dagger)** to kierunek, w którym warto podążać, jeśli Dagger jest centralnym elementem waszego CI. Daje to największą kontrolę, szybkość (dzięki cache'owaniu Daggera) i spójność między środowiskiem lokalnym a CI.

Dla Twojego obecnego przypadku, polecam zaimplementowanie **Propozycji 1** jako natychmiastowego ulepszenia. Następnie, w miarę rozwoju waszego potoku CI, możecie rozważyć migrację do **Propozycji 3**.
