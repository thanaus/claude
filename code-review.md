# Code Review — nexus v0.1.0

> Périmètre : `cmd/`, `internal/cli/`, `internal/nats/`, `internal/service/`, `internal/config/`
> Date : 2026-04-08

---

## Points forts

**Architecture CLI** — La séparation `Args` / `PreRunE` / `RunE` (ADR-002) est appliquée rigoureusement. `SilenceUsage` global (ADR-001), `MarkFlagRequired` natif (ADR-003), propagation du contexte NATS via `context.WithValue` : cohérent et justifié.

**`internal/cli/validator`** — Le pattern `Rule func(cmd, args) *ValidationError` + `Validator.Add(...).PreRunE()` est propre, extensible, et produit des erreurs agrégées en une passe. Bonne base pour la suite.

**NATS / JetStream** — `connect()` partagé, gestion du timeout via context, `defer nc.Close()`, retry sur collision de token (UUID v4 via `crypto/rand`) : correct.

---

## Problèmes à corriger

### 1. `cancel` nullable — `defer` ne protège pas correctement

**Fichiers :** `internal/nats/client.go`, `internal/nats/provisioner.go`
**Sévérité :** Moyenne

Si `ctx` possède déjà une deadline, `connect()` retourne `cancel = nil` et le `defer` est skippé.

```go
// Situation actuelle — fragile
nc, provisionCtx, cancel, err := connect(ctx, cfg)
if cancel != nil {
    defer cancel()
}
```

C'est correct fonctionnellement aujourd'hui, mais source d'oubli lors d'une future refacto. Le pattern idiomatique Go est de toujours retourner une `cancel` non-nil.

```go
// Direction recommandée — toujours retourner un cancel appelable
if _, hasDeadline := ctx.Deadline(); !hasDeadline {
    opCtx, cancel = context.WithTimeout(ctx, timeout)
} else {
    opCtx, cancel = ctx, func() {}  // no-op explicite
}
// L'appelant peut toujours faire defer cancel() sans garde-fou
```

---

### 2. `natsCfg, _` ignore silencieusement l'échec de contexte

**Fichier :** `cmd/sync.go`
**Sévérité :** Moyenne

`RunE` assume que `PreRunE` a injecté la config NATS dans le contexte. Si le wiring est cassé (validator manquant, mauvais ordre), le `ok = false` est ignoré et `natsCfg` est une valeur zéro utilisée dans le message d'erreur.

```go
// Situation actuelle — le _ est dangereux
natsCfg, _ := config.NATSConfigFromContext(cmd.Context())
```

```go
// Direction recommandée — paniquer sur un bug de wiring
natsCfg, ok := config.NATSConfigFromContext(cmd.Context())
if !ok {
    panic("NATSConfig absent du contexte : ValidateNATSConfig doit être enregistré dans PreRunE")
}
```

Un `panic` est approprié ici : c'est un bug de programmation, pas une erreur runtime utilisateur.

---

### 3. `streamConfigMatches` incomplet — drift silencieux

**Fichier :** `internal/nats/streams.go`
**Sévérité :** Moyenne

Plusieurs champs opérationnels ne sont pas comparés. Si `MaxAge`, `MaxMsgs` ou `Discard` changent dans `streamConfigs`, `EnsureStreams` ne déclenchera pas de `UpdateStream` : le stream restera avec les anciens paramètres sans avertissement.

```go
// Situation actuelle — champs manquants
func streamConfigMatches(existing, expected jetstream.StreamConfig) bool {
    return existing.Name == expected.Name &&
        slices.Equal(existing.Subjects, expected.Subjects) &&
        existing.Retention == expected.Retention &&
        existing.Storage == expected.Storage &&
        existing.Replicas == expected.Replicas
    // ⚠️ MaxAge, MaxMsgs, Discard, Description absents
}
```

Deux options :

- **Option A** — compléter la comparaison avec tous les champs opérationnels (`MaxAge`, `MaxMsgs`, `Discard`, `MaxBytes`).
- **Option B** — supprimer la comparaison et appeler systématiquement `CreateOrUpdateStream`, qui est idempotent par design dans JetStream. Plus simple, légèrement moins optimal.

---

### 4. `exactArgs` sans noms — absence de garde-fou

**Fichier :** `cmd/ux.go`
**Sévérité :** Faible

Un appel `exactArgs("hint")` sans noms d'arguments produit `expected = 0`, ce qui laisse passer n'importe quelle invocation silencieusement. Aucun signal au développeur.

```go
// Situation actuelle — aucun garde-fou
func exactArgs(hint string, names ...string) cobra.PositionalArgs {
    return func(cmd *cobra.Command, args []string) error {
        expected := len(names) // 0 si appelé sans noms
        // ...
    }
}
```

```go
// Direction recommandée — paniquer à l'initialisation
func exactArgs(hint string, names ...string) cobra.PositionalArgs {
    if len(names) == 0 {
        panic("exactArgs : au moins un nom d'argument est requis")
    }
    // ...
}
```

---

## Dette technique

### 5. Double connexion NATS si probe + provision combinés

**Fichiers :** `internal/nats/client.go`, `internal/nats/provisioner.go`
**Sévérité :** Dette (non bloquant aujourd'hui)

`Client.Probe()` et `Provisioner.Provision()` ouvrent chacun leur propre connexion NATS via `connect()`. Aujourd'hui `cmd/sync` n'utilise que `Provisioner`, donc une seule connexion est ouverte. Mais une future commande composant probe + provision créerait deux connexions séquentielles pour la même exécution.

Le TECHDEBT.md documente ce point. La direction recommandée y est correcte : introduire un `NATSSession` ou `NATSHandle` interne partageable, que `Provision` accepte en paramètre plutôt qu'en créer un lui-même.

---

## Anticipations pour la suite

### 6. `GetJob()` manquant pour `ls`, `worker`, `status`

**Fichier :** `internal/nats/kv.go`

Les trois commandes à implémenter devront récupérer un `SyncJob` depuis le KV bucket via le token fourni en argument. Il n'existe aujourd'hui que `SaveJob()`. Anticiper `GetJob(ctx, kv, token string) (SyncJob, error)` dès maintenant évite de le redécouvrir commande par commande.

```go
// À ajouter dans internal/nats/kv.go
func GetJob(ctx context.Context, kv jetstream.KeyValue, token string) (SyncJob, error) {
    entry, err := kv.Get(ctx, token)
    if err != nil {
        if errors.Is(err, jetstream.ErrKeyNotFound) {
            return SyncJob{}, fmt.Errorf("sync job %q not found", token)
        }
        return SyncJob{}, fmt.Errorf("cannot retrieve sync job %q: %w", token, err)
    }
    var job SyncJob
    if err := json.Unmarshal(entry.Value(), &job); err != nil {
        return SyncJob{}, fmt.Errorf("cannot decode sync job %q: %w", token, err)
    }
    return job, nil
}
```

### 7. Accès au flag `--verbose` persistant

**Fichiers :** `cmd/ls.go`, `cmd/worker.go`, `cmd/status.go`

`--verbose` est déclaré en `PersistentFlags` sur `root`, mais lu via `cmd.Flags().GetCount("verbose")` dans chaque `RunE`. Cobra résout les deux en pratique, mais l'accès explicite via `cmd.Root().PersistentFlags()` ou `cmd.InheritedFlags()` est plus lisible et évite toute ambiguïté à la lecture du code.

### 8. Compatibilité des dépendances avec Go 1.26

**Fichier :** `go.mod`

`cobra v1.10.2` et `nats.go v1.50.0` doivent être vérifiés contre le toolchain Go 1.26. Une v2 de Cobra a pu sortir dans l'intervalle — à contrôler avant d'implémenter les commandes suivantes.

### 9. Absence de tests

L'architecture actuelle (validators comme `Rule`, services injectés via interface) est naturellement testable. À prioriser avant d'implémenter les `RunE` métier des commandes `ls`, `worker` et `status`.

---

## Tableau récapitulatif

| # | Sévérité | Fichier(s) | Problème |
|---|----------|------------|----------|
| 1 | Moyenne | `internal/nats/*.go` | `cancel` nullable — retourner un no-op |
| 2 | Moyenne | `cmd/sync.go` | `natsCfg, _` ignore l'échec de contexte |
| 3 | Moyenne | `internal/nats/streams.go` | `streamConfigMatches` incomplet → drift silencieux |
| 4 | Faible | `cmd/ux.go` | `exactArgs()` sans noms : pas de garde-fou |
| 5 | Dette | `internal/nats/*.go` | Double connexion NATS si probe + provision combinés |
| 6 | Anticipation | `internal/nats/kv.go` | `GetJob()` manquant pour `ls` / `worker` / `status` |
| 7 | Anticipation | `cmd/*.go` | Accès au flag `--verbose` persistant à clarifier |
| 8 | Anticipation | `go.mod` | Compatibilité dépendances avec Go 1.26 à vérifier |
| 9 | Anticipation | — | Absence de tests unitaires |
