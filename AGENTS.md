# Hydra Market

Hydra Market is a continuous learning laboratory for Backend, Database, and Distributed Systems. The repository is the source of truth for both the working system and the lessons learned while evolving it.

## Working principles

- Start with the simplest implementation that makes the behavior observable.
- Do not introduce technology before a concrete pain has been observed.
- Reproduce a problem before fixing it.
- Measure before optimizing.
- Compare alternatives, trade-offs, and operational consequences before choosing a solution.
- Record investigated technical cases in `docs/problems/`.
- Record architectural decisions in `docs/adr/`.
- Use branches named `problem/<number>-<description>` for problem-driven work.
- When the objective is learning, do not make large changes automatically. Work incrementally and leave important decisions visible for review.
- Study documentation may be written in Portuguese. Code, branch names, and commit messages must be written in English.

## Current commands

```bash
docker compose up -d

for migration in db/migrations/*.sql; do
  docker compose exec -T postgres psql -U hydra -d hydra_market < "$migration"
done

go run ./cmd/api
go test ./...
./scripts/001-concurrent-orders.sh
```
