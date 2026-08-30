# Hydra Market — Engineering Journal

Registro dos principais problemas técnicos encontrados durante a evolução do Hydra Market.

O objetivo é documentar:

- o problema observado;
- como reproduzimos;
- evidências coletadas;
- causa raiz;
- alternativas consideradas;
- solução escolhida;
- trade-offs;
- validação da solução;
- aprendizados.

## Problems

| # | Problem | Concepts |
|---|---|---|
| 001 | [Lost update concorrente no estoque](problems/001-concurrent-inventory-lost-update.md) | Race condition, lost update, atomic conditional update |
| 002 | [Falta de atomicidade entre estoque e pedido](problems/002-order-atomicity.md) | ACID, transaction boundary, commit, rollback |

## Architecture Decisions

| ADR | Decision |
|---|---|
