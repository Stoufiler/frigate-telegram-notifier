# Refacto Frigate Telegram Bot — Design

Date : 2026-07-14

## Contexte

Bot Go qui écoute le topic MQTT `frigate/reviews` de Frigate et envoie des
notifications Telegram : un snapshot au début d'un événement (`type == "new"`),
un aperçu GIF à la fin (`type == "end"`). Tout le code est aujourd'hui dans un
seul `bot/main.go` (~400 lignes) accumulant du code mort et un schéma JSON
incorrect. Objectif : refacto propre, concis, compatible Frigate 0.17.2 **et**
0.18.0 (à sortir).

Le **comportement fonctionnel reste identique** : mêmes déclencheurs, mêmes
filtres, mêmes messages i18n. C'est un nettoyage + robustesse + compat 0.18.

## Décisions cadrées avec l'utilisateur

- Auth Frigate : support **optionnel** (inactif si variables vides).
- Compat : **0.17.2 ET 0.18.0**.
- Structure : **découpage en petits fichiers** (même package `main`).
- Robustesse : timeouts HTTP + arrêt propre + retry aussi sur le preview.
- Filtre `severity` : **non retenu** (YAGNI).

## Structure cible (package `main`, `bot/`)

| Fichier | Rôle |
|---|---|
| `main.go` | Câblage : charge config, construit clients, s'abonne MQTT, gère l'arrêt propre |
| `config.go` | `Config` chargée depuis l'env + validation en un seul endroit |
| `frigate.go` | Client HTTP Frigate : auth optionnelle, `Snapshot(id)`, `Preview(reviewID)`, retry générique |
| `notifier.go` | Enveloppe Telegram : `SendPhoto`, `SendAnimation` |
| `i18n.go` | Bundle + localizer, helpers `tr()` / `translateLabel()` |
| `event.go` | Struct `Review` (allégée), `handleNew` / `handleEnd`, filtrage factorisé |

Chaque unité a une responsabilité unique et une interface claire.

## Schéma JSON — struct `Review` allégée

Seuls les champs réellement publiés sur `frigate/reviews` sont modélisés
(le reste du schéma *events* actuel est supprimé) :

```go
type reviewItem struct {
    ID        string   `json:"id"`
    Camera    string   `json:"camera"`
    StartTime float64  `json:"start_time"`
    EndTime   *float64 `json:"end_time"`
    Severity  string   `json:"severity"` // parsé, non filtré
    Data      struct {
        Objects    []string `json:"objects"`
        Detections []string `json:"detections"`
        Zones      []string `json:"zones"`
    } `json:"data"`
}

type Review struct {
    Type  string     `json:"type"` // "new" | "update" | "end"
    After reviewItem `json:"after"`
}
```

Le parsing Go ignore les champs inconnus → tolérant à l'ajout de champs en 0.18.

## Compatibilité 0.17 / 0.18

- **Snapshot** : appelé avec `?bbox=1&bounding_box=1`. Frigate ignore le
  paramètre inconnu, donc la bounding box est dessinée dans les deux versions
  (`bbox` en 0.17, `bounding_box` en 0.18).
- **Preview** : `GET /api/review/{id}/preview` (inchangé). À reconfirmer au build.
- Parsing JSON tolérant (cf. ci-dessus).

## Auth Frigate (optionnelle)

Nouvelles variables **optionnelles** : `FRIGATE_USERNAME`, `FRIGATE_PASSWORD`.

- Vides → requêtes anonymes (cas port `:5000`, setup actuel, zéro changement).
- Remplies → le client fait `POST /api/login` (JSON `{"user","password"}`),
  récupère le JWT, l'attache en `Authorization: Bearer <jwt>` sur chaque requête.
  Sur un **401**, il se re-login une fois puis rejoue la requête (gère
  l'expiration du token pour un process longue durée).

Le client Frigate encapsule cette logique ; les handlers n'en savent rien.

## Robustesse

- `http.Client` unique avec timeout (~30 s).
- Retry générique (n tentatives, délai fixe) appliqué au snapshot **et** au preview.
- Arrêt propre via `signal.NotifyContext` (SIGINT/SIGTERM) : désabonnement +
  déconnexion MQTT, remplace `select {}`.
- Erreurs de `os.Create` / `io.Copy` vérifiées (plus d'écriture silencieuse d'un
  fichier vide envoyé à Telegram).

## Nettoyage

- Suppression du message `test_template` (debug) dans le code et les locales.
- `strings.Title` (déprécié) → `golang.org/x/text/cases` + `language`.
- `parseChatID` inliné.
- Filtrage caméra + objets factorisé en une fonction partagée entre `new`/`end`.

## Toolchain

- `go.mod` → **Go 1.26**.
- `Dockerfile` → base `golang:1.26-alpine`, multi-stage conservé.
- Dépendances : `go get -u ./... && go mod tidy`.

## Config finale (variables d'environnement)

Obligatoires : `TELEGRAM_TOKEN`, `TELEGRAM_CHAT_ID`, `MQTT_BROKER`,
`MQTT_TOPIC`, `FRIGATE_URL`.
Optionnelles : `MQTT_USERNAME`, `MQTT_PASSWORD`, `CAMERA_LIST`,
`MQTT_OBJECT_FILTER`, `BOT_LANGUAGE` (défaut `fr`),
**`FRIGATE_USERNAME`**, **`FRIGATE_PASSWORD`** (nouvelles).

## Vérification

- `go build ./...`, `go vet ./...`, `gofmt -l` propre.
- Revue manuelle : comportement `new`/`end` inchangé, filtres identiques.
- (Build binaire testé ; test end-to-end réel dépend d'une instance Frigate.)

## Hors périmètre (YAGNI)

- Filtre par `severity`.
- Réécriture de la CI GitHub Actions (hors demande).
