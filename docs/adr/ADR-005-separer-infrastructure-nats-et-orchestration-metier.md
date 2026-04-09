# ADR-005 — Séparer l'infrastructure NATS et l'orchestration métier

| Champ       | Valeur                                                |
|-------------|-------------------------------------------------------|
| Statut      | Accepté                                               |
| Date        | 2026-04-09                                            |
| Décideurs   | Équipe nexus                                          |
| Tags        | architecture, nats, jetstream, services, separation-of-concerns |

---

## Contexte

Le projet `nexus` s'appuie sur NATS et JetStream pour :

- ouvrir une connexion au serveur NATS ;
- vérifier la disponibilité de JetStream ;
- déclarer les streams et leurs subjects ;
- accéder au bucket KV `jobs` ;
- créer et lire des jobs ;
- orchestrer des workflows métiers comme `sync` et `ls`.

Au fil de l'implémentation, une partie de cette orchestration s'est retrouvée dans `internal/nats`, notamment via des types comme `Provisioner` et `JobsChecker`.

Cette approche mélangeait deux niveaux d'abstraction :

- **l'infrastructure bas niveau** : connexion, session JetStream, streams, KV ;
- **les cas d'usage métier** : créer un job, vérifier les prérequis de `ls`, préparer les ressources d'une commande.

Ce mélange rendait l'architecture moins lisible :

- le package `internal/nats` portait à la fois des primitives techniques et des scénarios complets ;
- la réutilisation des briques NATS dans plusieurs workflows était moins évidente ;
- les services métier dépendaient d'orchestrateurs NATS spécialisés au lieu de composer eux-mêmes les primitives nécessaires.

---

## Décision

Le package `internal/nats` devient une couche **strictement technique**.

Il porte uniquement :

- l'ouverture et la fermeture d'une session NATS / JetStream ;
- les helpers de configuration et d'accès aux streams ;
- les helpers de configuration et d'accès au KV `jobs`.

L'orchestration des workflows est déplacée dans `internal/service/...`.

Concrètement :

- `internal/nats/client.go` expose la primitive `OpenJetStream(...)` ;
- `internal/nats/streams.go` décrit et prépare les streams ;
- `internal/nats/kv.go` décrit le modèle `Job` et les accès au bucket `jobs` ;
- `internal/service/sync/service.go` orchestre la création d'un job ;
- `internal/service/ls/service.go` orchestre les prérequis du workflow `ls`.

Les orchestrateurs spécialisés dans `internal/nats` sont supprimés lorsqu'ils portent de la logique métier.

---

## Alternatives considérées

### Option B — Conserver l'orchestration dans `internal/nats`

Garder des types comme `Provisioner`, `JobsChecker` ou d'autres orchestrateurs spécialisés dans le package `internal/nats`.

**Rejeté** : cela entretient une frontière floue entre infrastructure et métier. Le package `internal/nats` devient un point d'entrée hétérogène, plus difficile à comprendre et à faire évoluer.

### Option C — Remonter toute la logique NATS dans `internal/service`

Faire manipuler directement `nats.Conn`, `jetstream.JetStream`, la configuration des streams et le KV depuis les services.

**Rejeté** : cela dupliquerait du code technique, disperserait la gestion de la connexion NATS et affaiblirait l'encapsulation des détails JetStream.

---

## Conséquences

### Positives

- La frontière d'architecture est plus nette : `internal/nats` fournit des primitives, `internal/service` compose des workflows.
- Les services métier deviennent plus explicites sur les étapes qu'ils exécutent.
- Les primitives NATS sont plus facilement réutilisables par `sync`, `ls`, `worker` et d'autres workflows futurs.
- Les tests peuvent viser séparément les helpers d'infrastructure et les scénarios métier.
- La réutilisation d'une session JetStream dans un même workflow devient plus simple via `OpenJetStream(...)`.

### Négatives / points de vigilance

- Les services métier contiennent désormais davantage de code d'orchestration, qu'il faudra garder lisible.
- Il faut éviter de recréer, côté service, des helpers techniques qui devraient rester dans `internal/nats`.
- Cette décision n'interdit pas d'ajouter de nouveaux helpers dans `internal/nats`, mais ils doivent rester au niveau infrastructure et non au niveau use case.

---

## Règle de conception

Quand un nouveau besoin apparaît, utiliser la règle suivante :

- si le code décrit **comment parler à NATS / JetStream / KV**, il appartient à `internal/nats` ;
- si le code décrit **dans quel ordre exécuter des étapes métier** pour une commande ou un workflow, il appartient à `internal/service/...`.

Exemples :

- `EnsureStreams()` : `internal/nats`
- `GetJob()` / `UpdateJob()` : `internal/nats`
- "charger un job, le passer en `running`, publier dans `DISCOVERY`, puis scanner" : `internal/service/ls`
- "préparer les streams, le KV, puis créer un job" : `internal/service/sync`

---

## Références

- `internal/nats/client.go` — ouverture d'une session JetStream
- `internal/nats/streams.go` — définition et préparation des streams
- `internal/nats/kv.go` — accès au bucket `jobs`
- `internal/service/sync/service.go` — orchestration du workflow `sync`
- `internal/service/ls/service.go` — orchestration des prérequis `ls`
- `TECHDEBT.md` — historique autour du cycle de vie des connexions NATS
