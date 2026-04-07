# ADR-001 — Désactivation globale de l'affichage automatique de l'usage Cobra

| Champ       | Valeur                        |
|-------------|-------------------------------|
| Statut      | Accepté                       |
| Date        | 2026-04-06                    |
| Décideurs   | Équipe nexus                  |
| Tags        | cli, ux, cobra, error-handling |

---

## Contexte

Le framework [Cobra](https://github.com/spf13/cobra) affiche automatiquement le bloc `Usage` d'une commande chaque fois qu'une erreur est retournée par `RunE` ou `PreRunE`. Ce comportement par défaut est activé via le flag `SilenceUsage = false` (valeur initiale).

Le projet `nexus` dispose d'un système de messages d'erreur custom centralisé dans `internal/validator` et `pkg/output`, conçu pour afficher des messages clairs, contextualisés et avec des hints actionnables. L'usage brut de Cobra, affiché en plus, crée :

- **une redondance d'information** : l'utilisateur voit à la fois le message custom et le bloc usage complet ;
- **une incohérence** : le message custom est affiché pour les erreurs de validation, mais le bloc usage Cobra s'affiche pour les erreurs natives (flag inconnu, sous-commande inconnue) ;
- **une régression UX** : sur des commandes avec de nombreux flags, le bloc usage peut occuper 20-30 lignes, noyant le message d'erreur réel.

---

## Décision

`SilenceUsage = true` est positionné **globalement** sur `rootCmd` dans `cmd/root.go`.

```go
// cmd/root.go — init()
rootCmd.SilenceErrors = true
rootCmd.SilenceUsage = true
```

Cela désactive l'affichage automatique de l'usage sur **toutes** les sous-commandes, quelle que soit la source de l'erreur (validation custom ou erreur Cobra native).

---

## Alternatives considérées

### Option B — `SilenceUsage` conditionnel dans `RunE`

Positionner `cmd.SilenceUsage = true` uniquement à l'intérieur des handlers métier, au cas par cas.

**Rejeté** : approche fragile, facilement oubliée sur les nouvelles commandes, et source d'incohérence à mesure que le projet grandit.

### Option C — Statu quo (comportement précédent)

`SilenceUsage = true` uniquement dans le `PreRunE` du validator, donc uniquement pour les erreurs de validation custom.

**Rejeté** : comportement incohérent. Les erreurs Cobra natives (ex: `--flag-inconnu`) continuaient à afficher l'usage, contrairement aux erreurs de validation.

---

## Conséquences

### Positives

- Comportement homogène : aucune erreur n'affiche jamais l'usage automatiquement.
- Messages d'erreur lisibles, sans bruit visuel.
- Alignement avec les CLIs modernes de référence (`gh`, `docker`, `kubectl`).
- Simplifie le code : le `cmd.SilenceUsage = true` redondant dans `internal/validator/validator.go` a été supprimé.

### Négatives / points de vigilance

- L'usage n'est **plus jamais** affiché automatiquement. En contrepartie, les messages d'erreur custom **doivent** fournir un hint suffisamment actionnable pour guider l'utilisateur.
- Convention à respecter pour toutes les nouvelles commandes : toute erreur retournée par `RunE` doit inclure un message clair indiquant comment obtenir de l'aide (ex: `"utilise 'nexus <cmd> --help' pour voir les options"`).

---

## Références

- [Cobra — SilenceUsage](https://pkg.go.dev/github.com/spf13/cobra#Command.SilenceUsage)
- `cmd/root.go` — application de la décision
- `internal/validator/validator.go` — gestion centralisée des erreurs de validation
- `pkg/output/output.go` — formatage des messages d'erreur
