# ADR-003 — Utiliser les flags requis natifs de Cobra

| Champ       | Valeur                               |
|-------------|--------------------------------------|
| Statut      | Accepté                              |
| Date        | 2026-04-07                           |
| Décideurs   | Équipe nexus                         |
| Tags        | cli, cobra, flags, validation, ux    |

---

## Contexte

Le projet `nexus` disposait d'un helper `RequireFlag()` dans `internal/validator`
pour vérifier manuellement qu'un flag texte obligatoire n'était pas vide.

Cette logique dupliquait une capacité déjà fournie nativement par
[Cobra](https://github.com/spf13/cobra) via `MarkFlagRequired()`.

Le principal argument en faveur du helper custom était le contrôle du message
affiché à l'utilisateur. Or le projet dispose déjà d'un point central de
reformatage des erreurs de flags dans `cmd/ux.go` grâce à `SetFlagErrorFunc(...)`.

---

## Décision

Le projet adopte la convention suivante :

- les flags requis simples doivent être déclarés avec `MarkFlagRequired()` ;
- les erreurs natives de Cobra associées aux flags requis doivent être
  reformattées via `flagErrorFunc(...)` ;
- `PreRunE` reste réservé aux validations qui dépassent la notion de
  "flag requis".

Exemples de validations qui restent dans `PreRunE` :

- incompatibilité entre deux flags ;
- dépendance conditionnelle entre plusieurs options ;
- validation métier d'une valeur déjà parsée.

---

## Alternatives considérées

### Option B — Conserver `RequireFlag()` dans `internal/validator`

**Rejeté** : cela réimplémente une fonctionnalité Cobra déjà disponible, ajoute
du code spécifique au projet, et disperse la responsabilité entre déclaration
du flag, validation et formatage du message.

### Option C — Utiliser `MarkFlagRequired()` sans reformater les erreurs

**Rejeté** : le message brut de Cobra est correct techniquement, mais trop peu
guidant pour l'UX souhaitée dans `nexus`.

---

## Conséquences

### Positives

- moins de code custom à maintenir ;
- meilleure proximité entre la déclaration d'un flag et son caractère requis ;
- meilleure cohérence avec l'approche déjà retenue pour les arguments
  positionnels ;
- messages utilisateur toujours homogènes grâce à `flagErrorFunc(...)`.

### Négatives / points de vigilance

- éviter `MarkPersistentFlagRequired()` sur `rootCmd`, car cela peut nuire à
  certaines commandes d'aide ou de complétion ;
- ne pas utiliser `MarkFlagRequired()` pour des règles conditionnelles qui
  relèvent plutôt de `PreRunE`.

---

## Application dans le code

Cette décision implique notamment :

- la suppression du helper `RequireFlag()` dans `internal/validator` ;
- l'enrichissement de `flagErrorFunc(...)` pour reformater les erreurs
  `required flag(s) ... not set` ;
- l'usage futur de `MarkFlagRequired()` sur les sous-commandes qui auront
  de vrais flags requis simples.

---

## Références

- [Cobra — MarkFlagRequired](https://pkg.go.dev/github.com/spf13/cobra#Command.MarkFlagRequired)
- `cmd/ux.go`
- `internal/validator/validator.go`
- `docs/adr/ADR-002-positional-args-vs-prerune-validation.md`
