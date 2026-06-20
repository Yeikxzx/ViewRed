# ViewRed — Contexto Completo del Proyecto

> Generado el 2026-06-20. Para restaurar contexto después de formateo:
> 1. Clonar el repo
> 2. Leer `docs/context/` en orden numérico
> 3. El agente leerá `00-README.md` primero, después el resto

## Índice

| Archivo | Contenido |
|---------|-----------|
| `00-README.md` | Este archivo — instrucciones de restauración |
| `01-project-summary.md` | Resumen del proyecto en un párrafo |
| `02-architecture.md` | Stack, decisiones de arquitectura, MVP scope |
| `03-sdd-context.md` | SDD preflight, strict TDD, init config |
| `04-git-state.md` | Estado actual del repo, ramas, commits |
| `05-completed-plans.md` | Planes 1-3: qué se implementó y mergeó |
| `06-plan4-current-state.md` | Branch feat/plan4-ml-and-alerts: cambios pendientes |
| `07-known-issues.md` | WARNINGs conocidos, bugs no resueltos |
| `08-ml-engine.md` | Motor ML: feature vector, WindowStats, training, datasets |
| `09-frontend.md` | Frontend: React Router, Zustand, SSE, componentes |
| `10-next-steps.md` | Próximos pasos recomendados |

## Para restaurar en una nueva sesión

```
1. git log --oneline -10          # Ver últimos commits
2. git status                      # Ver cambios sin commitear
3. git diff main..HEAD --stat     # Ver diff del branch activo
4. go test ./...                   # Verificar tests Go
5. cd web && npx vitest run        # Verificar tests frontend
6. cat docs/context/01-project-summary.md  # Leer resumen
```
