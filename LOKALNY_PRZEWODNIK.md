# Przewodnik po uruchamianiu pipeline'u lokalnie

Ten przewodnik opisuje, jak skonfigurować środowisko deweloperskie do pracy z projektem, pobierania kodu z Gerrit i uruchamiania pipeline'u lokalnie za pomocą Docker Compose.

## 1. Wymagania Wstępne

Zanim zaczniesz, upewnij się, że masz zainstalowane następujące narzędzia:

- **Git:** System kontroli wersji.
- **Klient SSH:** Do bezpiecznego połączenia z Gerrit.
- **Docker Desktop:** Do uruchamiania kontenerów i `docker compose`.
- **Dagger (Opcjonalnie):** Jeśli chcesz w pełni zautomatyzować i uruchamiać pipeline w taki sam sposób, jak na CI.

## 2. Konfiguracja Klucza SSH

Do komunikacji z Gerrit używacie SSH, więc każdy deweloper potrzebuje klucza SSH.

### a. Generowanie klucza SSH
Otwórz terminal (PowerShell lub Git Bash na Windows, terminal na Alma Linux) i wykonaj polecenie:
```bash
ssh-keygen -t rsa -b 4096 -C "twoj_email@example.com"
```
- Naciśnij Enter, aby zapisać klucz w domyślnej lokalizacji (`~/.ssh/id_rsa`).
- Zaleca się ustawienie silnego hasła (passphrase) dla dodatkowego bezpieczeństwa.

### b. Dodawanie klucza publicznego do Gerrit
- Skopiuj zawartość swojego klucza publicznego. Znajduje się on w pliku `~/.ssh/id_rsa.pub`.
- Zaloguj się na swoje konto w Gerrit, przejdź do `Settings > SSH Public Keys` i wklej skopiowany klucz.

## 3. Konfiguracja Gita i Gerrit

### a. Konfiguracja połączenia SSH (`~/.ssh/config`)
Aby ułatwić sobie pracę, skonfiguruj plik `~/.ssh/config`, dodając wpis dla waszego serwera Gerrit. Stwórz plik, jeśli nie istnieje.

```
Host gerrit.example.com
  HostName gerrit.example.com
  User twoja_nazwa_uzytkownika_gerrit
  IdentityFile ~/.ssh/id_rsa
  Port 29418
```
**Zastąp:**
- `gerrit.example.com` na adres waszego serwera Gerrit.
- `twoja_nazwa_uzytkownika_gerrit` na swoją nazwę użytkownika w Gerrit.
- `29418` na port SSH waszego Gerrita (jeśli jest inny).

### b. Konfiguracja użytkownika Git
```bash
git config --global user.name "Twoje Imię i Nazwisko"
git config --global user.email "twoj_email@example.com"
```

## 4. Pobieranie Kodu (Checkout)

### a. Klonowanie repozytorium
Użyj polecenia `git clone` z adresem SSH waszego repozytorium. Domyślnie pobrana zostanie gałąź `develop`.
```bash
git clone ssh://gerrit.example.com/NazwaWaszegoProjektu
cd NazwaWaszegoProjektu
```
**Zastąp:** `gerrit.example.com` i `NazwaWaszegoProjektu`.

### b. Pobieranie istniejącej zmiany (Code Review)
- **Zalecane:** Użyj narzędzia `git-review`.
  ```bash
  pip install git-review
  git review -d <numer_zmiany>
  ```
- **Alternatywnie:** Użyj `git fetch`. Znajdź komendę na stronie zmiany w Gerrit.
  ```bash
  git fetch <url> refs/changes/XX/YYYY/ZZ && git checkout FETCH_HEAD
  ```

## 5. Instrukcje dla Środowisk

### Windows 11 (z WSL2) - Zalecane
1. Zainstaluj **WSL2** i **Docker Desktop for Windows**.
2. W ustawieniach Docker Desktop włącz integrację z twoją dystrybucją WSL2.
3. **Wszystkie operacje (git, ssh, docker) wykonuj w terminalu WSL2.**

### Windows 11 (natywnie)
1. Zainstaluj **Git for Windows** i **Docker Desktop for Windows**.
2. Polecenia `git` i `ssh` wykonuj w **Git Bash**.
3. Polecenia `docker` i `docker compose` wykonuj w **PowerShell** lub **CMD**.

## 6. Uruchamianie Pipeline'u Lokalnie

Po pobraniu kodu, możesz uruchomić pipeline.

### a. Uruchomienie głównych usług
```bash
docker compose up -d --build
```

### b. Uruchomienie testów Playwright
- **Tryb Headless (domyślny):**
  ```bash
  docker compose --profile playwright up --build --abort-on-container-exit
  ```
- **Tryb Headed (do debugowania):**
  ```bash
  HEADED_MODE=true docker compose --profile playwright up --build --abort-on-container-exit
  ```

*Uwaga: W zależności od konfiguracji, polecenia `docker` mogą wymagać `sudo` w WSL2.*
