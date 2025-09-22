# Analiza Połączenia z Zdalnym Systemem Legacy

Poniższy dokument przedstawia analizę i trzy propozycje dotyczące sposobu integracji lokalnego środowiska Docker Compose z zdalnym systemem legacy.

### Analiza Problemu

Głównym wyzwaniem jest to, że kontenery w Docker Compose działają w swojej własnej, izolowanej sieci wirtualnej. Domyślnie nie "widzą" one zdalnych maszyn w taki sam sposób, jak komputer hosta (maszyna dewelopera). Musimy w jawny sposób przekazać informację o adresie systemu legacy do wnętrza kontenera, który ma się z nim komunikować. Rozwiązanie powinno unikać "hardkodowania" adresów IP bezpośrednio w kodzie aplikacji.

Oto trzy propozycje, od najprostszej i najczęściej stosowanej, po bardziej zaawansowane.

---

### Propozycja 1: Użycie zmiennych środowiskowych i pliku `.env` (Najbardziej zalecana)

To standardowe i najbardziej elastyczne podejście, które idealnie pasuje do opisu "w pliku properties". Docker Compose automatycznie wczytuje plik o nazwie `.env` z głównego katalogu projektu i udostępnia zdefiniowane w nim zmienne.

**Jak to działa:**
1.  Tworzymy plik `.env` w głównym katalogu projektu, obok `docker-compose.yml`.
2.  W pliku `.env` definiujemy adres i port systemu legacy.
3.  W pliku `docker-compose.yml` przekazujemy te zmienne do środowiska odpowiedniego kontenera.
4.  Aplikacja wewnątrz kontenera odczytuje te zmienne środowiskowe i używa ich do nawiązania połączenia.

**Implementacja:**

1.  **Stwórz plik `.env`:**
    ```ini
    # Adres IP lub nazwa domenowa serwera z systemem legacy
    LEGACY_API_HOST=legacy.system.com
    # Port, na którym działa system legacy
    LEGACY_API_PORT=8080
    ```

2.  **Zmodyfikuj `docker-compose.yml`:**
    W definicji serwisu, który ma się łączyć z systemem legacy (np. `spring-boot-app`), dodaj sekcję `environment`.

    ```yaml
    services:
      spring-boot-app:
        build: ./spring-boot-app
        ports:
          - "8081:8081"
        environment:
          # Przekazujemy zmienne do kontenera. Możemy je złożyć w pełny URL.
          - LEGACY_API_URL=http://${LEGACY_API_HOST}:${LEGACY_API_PORT}
          # Lub przekazać osobno
          - LEGACY_HOST=${LEGACY_API_HOST}
          - LEGACY_PORT=${LEGACY_API_PORT}
    ```

**Zalety:**
*   **Elastyczność:** Każdy deweloper może mieć własny plik `.env` z innym adresem (np. do testowania z różnymi środowiskami legacy).
*   **Bezpieczeństwo:** Plik `.env` nie powinien być dodawany do systemu kontroli wersji (należy go dodać do `.gitignore`), co zapobiega wyciekowi wrażliwych adresów.
*   **Standard:** Jest to powszechnie przyjęta praktyka konfiguracji aplikacji w konteneryzacji.

**Wady:**
*   Wymaga, aby aplikacja była napisana tak, by odczytywać konfigurację ze zmiennych środowiskowych (co jest dobrą praktyką).

---

### Propozycja 2: Użycie `extra_hosts` w Docker Compose

To podejście jest przydatne, gdy aplikacja ma w kodzie zaszytą konkretną nazwę hosta (np. `legacy.api.local`), a my chcemy przekierować ten ruch na konkretny adres IP bez modyfikacji kodu aplikacji.

**Jak to działa:**
Opcja `extra_hosts` w `docker-compose.yml` dodaje wpis do pliku `/etc/hosts` wewnątrz kontenera. Działa to jak statyczny wpis DNS tylko dla tego kontenera.

**Implementacja:**

1.  **Zmodyfikuj `docker-compose.yml`:**
    W definicji serwisu dodaj sekcję `extra_hosts`. Można to połączyć z plikiem `.env` dla elastyczności.

    *Plik `.env`*
    ```ini
    LEGACY_SYSTEM_IP=192.168.1.100
    ```

    *Plik `docker-compose.yml`*
    ```yaml
    services:
      spring-boot-app:
        build: ./spring-boot-app
        ports:
          - "8081:8081"
        extra_hosts:
          # Aplikacja łącząc się z 'legacy.api.local', w rzeczywistości połączy się z adresem IP podanym w zmiennej
          - "legacy.api.local:${LEGACY_SYSTEM_IP}"
    ```

**Zalety:**
*   **Przezroczystość dla aplikacji:** Nie wymaga żadnych zmian w kodzie aplikacji, jeśli używa ona stałej nazwy domenowej.
*   **Prostota konfiguracji:** Wszystko jest zdefiniowane w jednym pliku `docker-compose.yml`.

**Wady:**
*   **Mniejsza elastyczność:** Dotyczy tylko mapowania nazwy hosta na IP. Jeśli zmieniają się też porty lub protokoły (http/https), to nie rozwiązuje problemu.
*   Może być mylące, jeśli nie jest dobrze udokumentowane, dlaczego dana nazwa hosta działa.

---

### Propozycja 3: Użycie tunelu SSH (Rozwiązanie zaawansowane)

To rozwiązanie jest idealne, gdy system legacy nie jest wystawiony publicznie w internecie, a dostęp do niego jest możliwy tylko przez SSH (np. z serwera bastionowego).

**Jak to działa:**
1.  Deweloper tworzy tunel SSH ze swojej lokalnej maszyny na serwer legacy. Tunel ten "przekierowuje" port ze zdalnej maszyny na port lokalny.
2.  Kontener Docker łączy się ze specjalnym adresem `host.docker.internal`, który zawsze wskazuje na maszynę hosta (komputer dewelopera).
3.  Ruch z kontenera trafia na lokalny port hosta, a następnie przez tunel SSH jest bezpiecznie przesyłany do systemu legacy.

**Implementacja:**

1.  **Deweloper uruchamia tunel SSH w terminalu:**
    ```bash
    # Przekieruj port 8080 z serwera legacy na lokalny port 9000
    ssh -L 9000:localhost:8080 user@remote-legacy-server.com
    ```
    *   `9000`: lokalny port na maszynie dewelopera.
    *   `localhost:8080`: adres i port systemu legacy *z perspektywy serwera, z którym się łączymy*.
    *   `user@remote-legacy-server.com`: dane logowania SSH.

2.  **Skonfiguruj aplikację (np. przez plik `.env` z Propozycji 1):**
    *Plik `.env`*
    ```ini
    # Używamy specjalnej nazwy DNS Docker, która wskazuje na hosta
    LEGACY_API_HOST=host.docker.internal
    # Używamy lokalnego portu, na który tunelujemy
    LEGACY_API_PORT=9000
    ```
    Plik `docker-compose.yml` pozostaje taki sam jak w Propozycji 1.

**Zalety:**
*   **Bardzo bezpieczne:** Cała komunikacja jest szyfrowana przez SSH.
*   **Omija problemy sieciowe:** Nie wymaga, aby system legacy miał publiczny adres IP; wystarczy dostęp przez SSH.
*   Nie wymaga zmian w `docker-compose.yml` (poza konfiguracją w `.env`).

**Wady:**
*   **Skomplikowane dla dewelopera:** Wymaga ręcznego uruchomienia i zarządzania tunelem SSH.
*   `host.docker.internal` może mieć różne implementacje w zależności od systemu (Linux vs. Windows/Mac), choć w nowszych wersjach Dockera jest to ustandaryzowane.

### Podsumowanie i Rekomendacja

| Kryterium               | Propozycja 1 (`.env`)                               | Propozycja 2 (`extra_hosts`)                     | Propozycja 3 (Tunel SSH)                               |
| ----------------------- | --------------------------------------------------- | ------------------------------------------------ | ------------------------------------------------------ |
| **Łatwość implementacji** | **Wysoka**                                          | Wysoka                                           | Niska (wymaga dodatkowych kroków)                      |
| **Elastyczność**          | **Bardzo wysoka**                                   | Średnia (tylko mapowanie hosta)                  | Wysoka (ale wymaga manualnej konfiguracji)             |
| **Bezpieczeństwo**        | Dobre (nie przechowuje sekretów w repo)             | Dobre                                            | **Bardzo wysokie** (szyfrowanie end-to-end)            |
| **Kiedy używać**          | **W 90% przypadków.** Standardowa, prosta i mocna. | Gdy aplikacja legacy ma "zaszytą" nazwę hosta. | Gdy wymagane jest bezpieczne połączenie z siecią prywatną. |

**Rekomendacja:**
Zdecydowanie zacznij od **Propozycji 1**. Jest to najbardziej uniwersalne, proste w utrzymaniu i powszechnie stosowane rozwiązanie. Jeśli napotkacie problemy z kodem, który nie chce używać konfigurowalnego hosta, wtedy **Propozycja 2** jest dobrym planem B. **Propozycję 3** zachowajcie dla scenariuszy z podwyższonymi wymaganiami bezpieczeństwa lub skomplikowaną topologią sieci.
