# ViewRed — Próximos Pasos Recomendados

## Prioridad 1: Commitear el trabajo pendiente del otro agente

Hay 28 archivos modificados + 14 untracked. Decidir:
- ¿Commitear todo como un solo batch?
- ¿Separar por plan (Plan 4 / Plan 5 / Plan 6)?
- ¿Revisar antes de commitear?

## Prioridad 2: Crear SDD artifacts para Plan 4+

No existen artifacts SDD (explore, propose, spec, design, tasks) para Plan 4+.
Usar `sdd-new` para cada plan antes de continuar implementando.

## Prioridad 3: Integración completa de ML + Rules + main.go

El main.go actual tiene el rules engine y ml engine creados, pero:
- `store.GetCustomRules` puede fallar si store es nil (corregido por este agente)
- El worker pool evalúa packets pero la integración con storage no está completa
- Las alerts del ML engine no están wireadas al writer/broker

## Prioridad 4: Feature engineering adicional

- Agregar más features: SrcPort ya agregado (commit d4be4fc)
- Considerar: TCP flags, flow duration, bytes por flow
- Reentrenar con datasets reales (CIC-IoT-2023 recomendado)

## Prioridad 5: Limpieza del repo

- Decidir si commitear `onnxruntime.dll` (11MB) o agregar a `.gitignore`
- Decidir si commitear `models/` o generar via script
- Eliminar `fix_tests.py` si no se necesita
- Decidir entre `package-lock.json` (npm) y `pnpm-lock.yaml`
- Agregar `bin/`, `*.exe`, `viewred` a `.gitignore`

## Prioridad 6: Decisiones de branch

- `feat/plan2-capture-and-storage` se puede eliminar (histórico)
- `hotfix/plan3-warnings` está vacía
- Decidir si mergear `feat/plan4-ml-and-alerts` a `main` o seguir en feature branch

## Sobre ViewRed en general

- Timeline estimado: 7-9 meses solo, 3-5 con equipo de 2-3
- Próximo milestone: tener Plan 4 (ML+alerts) funcional en main
- El proyecto está en una etapa temprana pero con base sólida (Planes 1-3)
