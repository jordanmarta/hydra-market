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

./scripts/001-concurrent-orders.sh

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

### 1. Mutex na aplicação

Uma possibilidade seria proteger o trecho crítico com um `sync.Mutex` no Go:

GetStock
→ lock
→ validar estoque
→ atualizar estoque
→ unlock

Isso impediria que duas goroutines da MESMA instância executassem simultaneamente a alteração de estoque.

Problema:

Em produção provavelmente teremos múltiplas instâncias:

Hydra Instance A
Hydra Instance B
Hydra Instance C

Cada processo possui sua própria memória e, portanto, seu próprio mutex.

Exemplo:

Request 1 → Instance A → Mutex A
Request 2 → Instance B → Mutex B

Os dois mutexes podem ser adquiridos ao mesmo tempo.

Portanto, o mutex resolve concorrência dentro de um único processo, mas não coordena múltiplas instâncias da aplicação.

Por isso ele não é uma boa solução para proteger um estado compartilhado armazenado no PostgreSQL.

---

### 2. Optimistic Locking

Outra alternativa seria adicionar uma versão ao estoque:

product_id | quantity | version
1          | 10       | 5

A aplicação primeiro lê:

quantity = 10
version = 5

Depois tenta atualizar:

UPDATE inventory
SET
    quantity = 9,
    version = 6
WHERE product_id = 1
  AND version = 5;

Se outra requisição tiver alterado esse registro antes:

version = 6

o UPDATE não modifica nenhuma linha.

Isso significa:

"o estado que você leu já mudou."

Nesse caso a aplicação pode:

- buscar novamente o estado;
- recalcular;
- tentar novamente;
- ou retornar conflito.

Essa solução FUNCIONARIA.

Trade-off:

Ela adiciona controle de versão e normalmente exige lógica de retry/conflito na aplicação.

É especialmente interessante quando conflitos concorrentes são relativamente raros.

---

### 3. Pessimistic Locking — SELECT FOR UPDATE

Também poderíamos fazer:

BEGIN;

SELECT quantity
FROM inventory
WHERE product_id = 1
FOR UPDATE;

validar estoque

UPDATE inventory ...

COMMIT;

O `FOR UPDATE` bloqueia aquela linha para alterações concorrentes enquanto a transação estiver aberta.

Se outra transação tentar pegar a mesma linha com `FOR UPDATE`, ela terá que esperar.

Exemplo:

Transaction A
→ bloqueia inventory/product=1

Transaction B
→ tenta bloquear product=1
→ espera

Transaction A
→ atualiza
→ COMMIT
→ libera lock

Transaction B
→ continua
→ agora enxerga o novo estoque

Essa solução também FUNCIONARIA.

Trade-off:

Mantemos um lock durante a transação, aumentando contenção e exigindo cuidado com:

- duração da transação;
- deadlocks;
- throughput;
- ordem de aquisição de locks.

É especialmente útil quando precisamos ler um estado e executar várias operações relacionadas garantindo que ninguém o altere durante aquele fluxo.

---

### 4. UPDATE condicional atômico

Foi a solução escolhida.

Em vez de:

SELECT
→ validar na aplicação
→ calcular
→ UPDATE

fazemos tudo em uma única instrução:

UPDATE inventory
SET
    quantity = quantity - $2,
    updated_at = NOW()
WHERE product_id = $1
  AND quantity >= $2
RETURNING quantity;

A condição:

quantity >= $2

e a alteração:

quantity = quantity - $2

fazem parte da mesma operação executada pelo PostgreSQL.

Quando o estoque acaba, nenhuma linha atende à condição e nenhum UPDATE acontece.

Essa solução também funciona com múltiplas instâncias da aplicação porque a coordenação ocorre onde o estado compartilhado realmente existe: no banco.

---

## Por que escolhemos UPDATE atômico?

As quatro abordagens atacam problemas de concorrência, mas com características diferentes.

Mutex:
simples, porém limitado a uma única instância.

Optimistic Locking:
distribuído e válido, mas adiciona versionamento e tratamento de conflito/retry.

SELECT FOR UPDATE:
forte e válido, mas mantém lock durante uma transação e é mais adequado quando várias operações precisam ser protegidas juntas.

UPDATE condicional:
resolve exatamente nossa necessidade atual em uma única operação atômica e com pouca complexidade adicional.

Para o problema atual:

"decrementar estoque somente se ainda existir quantidade suficiente"

o UPDATE condicional é a solução mais simples entre as alternativas avaliadas.