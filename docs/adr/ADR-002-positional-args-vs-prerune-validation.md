# ADR-002 — Répartition de la validation entre `Args` et `PreRunE`

| Champ       | Valeur                               |
|-------------|--------------------------------------|
| Statut      | Accepté                              |
| Date        | 2026-04-07                           |
| Décideurs   | Équipe nexus                         |
| Tags        | cli, cobra, validation, architecture |

---

## Contexte

Le projet `nexus` utilise [Cobra](https://github.com/spf13/cobra) pour structurer la CLI et `internal/validator` pour centraliser certaines validations custom.

Une duplication s'était installée autour des arguments positionnels :

- certaines commandes validaient la cardinalité via `Args`;
- des règles comme `RequireArgs()` et `ValidateToken()` validaient aussi les arguments dans `PreRunE`.

Cette superposition rendait la responsabilité de chaque couche moins claire :

- faut-il lire `Args` pour comprendre la forme de la commande ;
- ou `PreRunE` ;
- ou les deux.

À mesure que la CLI grandit, cette ambiguïté augmente le risque de divergence entre commandes, de logique redondante, et de messages incohérents.

---

## Décision

Le projet adopte la convention suivante :

- `Args` valide la **structure de l'invocation** ;
- `PreRunE` valide la **cohérence fonctionnelle et métier** ;
- `RunE` exécute uniquement le comportement métier.

Concrètement :

- la cardinalité des arguments positionnels doit être portée par `Args` ;
- les validations de type "argument obligatoire", "nombre d'arguments attendu", ou "argument vide" ne doivent plus être dupliquées dans `PreRunE` ;
- `PreRunE` reste le bon endroit pour les règles de flags, de compatibilité entre options, et les validations métier.

Exemples :

- `Args`: `sync` attend exactement `<source> <destination>` ;
- `Args`: `ls`, `status` et `worker` attendent exactement `<token>` ;
- `PreRunE`: `--output` doit être parmi les formats supportés ;
- `PreRunE`: `--limit` doit être strictement positif.

---

## Alternatives considérées

### Option B — Tout valider dans `PreRunE`

Conserver la cardinalité des arguments dans `internal/validator`.

**Rejeté** : cela détourne `Args` de sa responsabilité naturelle dans Cobra, rend chaque commande moins lisible, et oblige à aller chercher la forme de la commande ailleurs que dans sa définition.

### Option C — Tout valider dans `Args`

Déplacer aussi les validations de flags et de cohérence métier vers `Args`.

**Rejeté** : `Args` est adapté aux arguments positionnels, pas aux validations transverses sur les flags ou aux règles métier. Cette approche mélangerait deux niveaux d'abstraction.

### Option D — Mélanger `Args` et `PreRunE` selon les commandes

Laisser chaque commande choisir librement.

**Rejeté** : trop flexible, donc source d'incohérence. La CLI doit suivre une convention stable.

---

## Conséquences

### Positives

- Lecture plus immédiate des commandes : la forme d'invocation est visible directement dans `Use` + `Args`.
- Moins de duplication entre `cmd/*` et `internal/validator`.
- Messages d'erreur plus homogènes.
- Meilleure évolutivité quand de nouvelles sous-commandes seront ajoutées.

### Négatives / points de vigilance

- Les helpers `Args` doivent rester simples et centrés sur les arguments positionnels.
- Toute nouvelle règle métier ajoutée par réflexe dans `Args` devra être revue si elle concerne en réalité les flags ou la logique applicative.

---

## Application dans le code

Cette décision implique notamment :

- l'usage de `Args` sur les commandes définies dans `cmd/`;
- la suppression des validateurs `RequireArgs()` et `ValidateToken()` devenus redondants ;
- la conservation des validateurs de flags dans `internal/validator`.

---

## Références

- [Cobra — Positional and Custom Arguments](https://pkg.go.dev/github.com/spf13/cobra#PositionalArgs)
- `cmd/ux.go` — helper `exactArgs(...)`
- `cmd/ls.go`
- `cmd/status.go`
- `cmd/worker.go`
- `cmd/sync.go`
- `internal/validator/validator.go`
