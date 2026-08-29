# 001 — Lost update concorrente no estoque

## Contexto

A primeira versão do fluxo de compra do Hydra Market consulta o estoque atual,
calcula a nova quantidade na aplicação e grava o novo valor no PostgreSQL.

Fluxo atual:

GetStock
→ validar quantidade
→ calcular novo estoque
→ SetStock
→ criar pedido

A implementação atual é propositalmente simples e não utiliza lock,
controle de versão ou atualização atômica do estoque.

---

## Experimento

Script utilizado:

./scripts/concurrent-orders.sh

O script:

1. redefine o estoque inicial;
2. registra estoque e vendas antes do teste;
3. dispara múltiplas requisições concorrentes para POST /orders;
4. registra estoque e vendas após o teste.

Cada pedido tenta comprar uma unidade do mesmo produto.

---

## Evidências

### Execução 1

Cenário:

- Estoque inicial: 10
- Requisições: 50
- Concorrência: 50
- Quantidade por pedido: 1

Resultado:

50 × HTTP 201

Estado final:

Estoque final: 1
Novas vendas: 50

O sistema aceitou 50 vendas mesmo existindo apenas 10 unidades disponíveis.

---

### Execução 2

Cenário:

- Estoque inicial: 10
- Requisições: 50
- Concorrência: 50
- Quantidade por pedido: 1

Resultado:

49 × HTTP 201
1 × HTTP 409

Estado final:

Estoque final: 0
Novas vendas: 49

Novamente o sistema aceitou significativamente mais pedidos do que o estoque disponível.

O resultado variou entre execuções, demonstrando comportamento não determinístico sob concorrência.

---

## Evidência do lost update

Para observar melhor o mecanismo da falha, foi executado um cenário menor.

Cenário:

- Estoque inicial: 2
- Requisições: 5
- Concorrência: 5
- Quantidade por pedido: 1

Resultado:

5 × HTTP 201

Estado final:

Estoque final: 0
Novas vendas: 5

O comportamento correto seria permitir no máximo duas vendas.

Logs coletados durante a execução:

[RACE] READ  product=1 stock=2
[RACE] WRITE product=1 stock=1

[RACE] READ  product=1 stock=1
[RACE] WRITE product=1 stock=0

[RACE] READ  product=1 stock=1
[RACE] WRITE product=1 stock=0

[RACE] READ  product=1 stock=1
[RACE] WRITE product=1 stock=0

[RACE] READ  product=1 stock=1
[RACE] WRITE product=1 stock=0

Várias requisições leram o mesmo valor de estoque (1) e tomaram a decisão de venda com base nesse mesmo estado.

Cada uma calculou individualmente:

1 - 1 = 0

e posteriormente gravou 0.

Assim, múltiplos pedidos foram criados, mas as atualizações de estoque sobrescreveram umas às outras.

---

## Comportamento observado

O sistema permite overselling quando múltiplas requisições tentam comprar o mesmo produto simultaneamente.

O estoque não necessariamente fica negativo.

Em vez disso, atualizações concorrentes podem sobrescrever valores calculados por outras requisições.

Esse comportamento caracteriza um problema de concorrência conhecido como lost update.

---

## Hipótese

O problema ocorre porque a alteração de estoque é implementada como uma operação read-modify-write composta por múltiplas etapas independentes:

SELECT quantity
↓
cálculo na aplicação
↓
UPDATE quantity

Entre o SELECT e o UPDATE, outras requisições podem ler e modificar o mesmo registro.

---

## Causa raiz

O estoque é lido e atualizado em operações separadas, sem nenhum mecanismo que garanta exclusividade ou valide se o estado utilizado no cálculo continua atual.

Exemplo:

Request A lê estoque = 1
Request B lê estoque = 1

Request A calcula 0
Request B calcula 0

Request A grava 0
Request B grava 0

Duas vendas foram realizadas, mas o estoque foi reduzido apenas uma vez.

A validação:

currentStock >= requestedQuantity

acontece na aplicação utilizando um valor que pode se tornar obsoleto antes da gravação.

Portanto, o problema não é ausência de validação de estoque.

O problema é que a validação e a atualização não formam uma operação atômica.

---

## Alternativas consideradas

Ainda não avaliadas.

Possíveis alternativas a investigar:

- atualização atômica no PostgreSQL;
- SELECT ... FOR UPDATE dentro de uma transação;
- optimistic locking com controle de versão;
- mutex na aplicação.

---

## Decisão

Pendente.

A solução será escolhida após comparação das alternativas e seus respectivos trade-offs.

---

## Validação

Pendente.

Após implementar a solução, o mesmo experimento deverá ser executado novamente.

Para o cenário:

Estoque inicial: 10
Requisições: 50

o comportamento esperado será:

10 × sucesso
40 × rejeição por estoque insuficiente

e:

Estoque final: 0

---

## Lições aprendidas

Pendente.

Esta seção será concluída após a implementação e validação da solução.