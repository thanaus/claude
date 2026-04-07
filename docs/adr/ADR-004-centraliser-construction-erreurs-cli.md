# ADR-004 — Centraliser progressivement la construction des erreurs CLI

| Champ       | Valeur                              |
|-------------|-------------------------------------|
| Statut      | Accepté                             |
| Date        | 2026-04-07                          |
| Décideurs   | Équipe nexus                        |
| Tags        | cli, ux, validator, output, cobra   |

---

## Contexte

Le projet `nexus` a récemment clarifié sa structure CLI autour de :

- `cmd/` pour la définition des commandes Cobra ;
- `internal/cli/output` pour le rendu des erreurs et messages utilisateur ;
- `internal/cli/validator` pour les règles de validation exécutées en `PreRunE`.

Cependant, un recouvrement fonctionnel subsiste entre `cmd/ux.go` et
`internal/cli/validator/validator.go`.

Aujourd'hui :

- `cmd/ux.go` gère `exactArgs(...)`, `flagErrorFunc(...)` et construit aussi des
  `ValidationError` ;
- `internal/cli/validator` définit les règles de validation réutilisables et
  construit également des `ValidationError`.

Cette situation fonctionne, mais elle brouille la séparation des responsabilités :

- où une erreur CLI doit-elle être créée ;
- dans `cmd/ux.go` ;
- ou dans `internal/cli/validator`.

---

## Décision

La direction cible du projet est la suivante :

- `internal/cli/output` reste responsable du **format de rendu** ;
- `internal/cli/validator` devient l'endroit privilégié pour la
  **construction des erreurs de validation CLI** ;
- `cmd/ux.go` reste limité aux helpers spécifiques à Cobra
  (`Args`, `flagErrorFunc`, adaptation du framework).

Autrement dit :

- `ux.go` peut continuer à porter des helpers liés à Cobra ;
- mais il ne doit plus, à terme, réimplémenter sa propre mini-couche de
  construction d'erreurs indépendante du validateur ;
- la création des `ValidationError` doit converger progressivement vers une
  logique centralisée.

---

## Alternatives considérées

### Option B — Conserver durablement deux points de construction d'erreurs

Laisser `cmd/ux.go` et `internal/cli/validator` créer chacun leurs erreurs.

**Rejeté** : cela entretient l'impression de redondance, rend l'architecture
moins lisible et augmente le risque de divergence dans les messages ou dans
leur formatage.

### Option C — Déplacer immédiatement toute la logique de `ux.go` dans `validator`

Tout faire converger en une seule étape.

**Rejeté** : `ux.go` porte encore des adaptations spécifiques à Cobra
(`PositionalArgs`, `flagErrorFunc`) qui méritent de rester proches de la
couche `cmd/`.

---

## Conséquences

### Positives

- architecture plus lisible pour les futures commandes ;
- moins de duplication conceptuelle autour des erreurs CLI ;
- meilleur point d'extension pour faire évoluer les messages, hints et usages ;
- séparation plus nette entre adaptation de Cobra et validation métier / CLI.

### Négatives / points de vigilance

- la convergence sera progressive, pas instantanée ;
- `cmd/ux.go` continuera temporairement à contenir une petite part de logique
  de construction d'erreur tant que le refactor complet n'aura pas été mené ;
- il faudra veiller à ne pas introduire de nouveaux helpers parallèles.

---

## Application dans le code

À court terme :

- `internal/cli/output` reste inchangé comme couche de rendu ;
- `internal/cli/validator` continue d'agréger les règles `PreRunE` ;
- `cmd/ux.go` conserve `exactArgs(...)` et `flagErrorFunc(...)`.

À moyen terme :

- les helpers de construction d'erreur actuellement présents dans `cmd/ux.go`
  devront converger vers une logique partagée avec `internal/cli/validator`.

---

## Références

- `cmd/ux.go`
- `internal/cli/validator/validator.go`
- `internal/cli/output/output.go`
- `docs/adr/ADR-002-positional-args-vs-prerune-validation.md`
