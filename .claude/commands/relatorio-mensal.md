# Gerar Relatório Mensal de Atividades

Gere o relatório mensal de prestação de serviços buscando as atividades no Jira e preenchendo o template HTML.

## Fluxo

### 1. Preparar o binário

Verifique se o binário `jira-reporter` existe na raiz do projeto. Se não existir, compile com:

```bash
cd /home/alangomes/projetos/jira-reporter && go build -o jira-reporter .
```

### 2. Determinar o mês do relatório

- Se o usuário especificou um mês/ano (ex: "03/2026"), use esse valor com a flag `-d`.
- Caso contrário, o CLI automaticamente usa o mês anterior - não precisa passar a flag `-d`.

### 3. Perguntar preferências ao usuário

Antes de executar, pergunte:
- **Formato**: HTML ou DOCX? (padrão: HTML)
- **Incluir tarefas de QA?** (flag `-q` inclui cards onde o usuário é QA assignee)

Se o usuário não quiser ser perguntado (ex: "gera logo", "gera rápido"), use os padrões: HTML sem QA.

### 4. Executar o CLI

Monte o comando baseado nas respostas:

```bash
cd /home/alangomes/projetos/jira-reporter && ./jira-reporter -n "Relatório de Prestação de Serviços" -p "./reports/" [-f html|docx] [-d MM/YYYY] [-q]
```

Flags disponíveis:
- `-n "nome"` - Nome do arquivo (sem extensão)
- `-p "./reports/"` - Diretório de saída
- `-f html` ou `-f docx` - Formato (padrão: html)
- `-d "MM/YYYY"` - Mês/ano específico (padrão: mês anterior)
- `-q` - Incluir tarefas de QA

### 5. Enriquecer descrições das atividades via API do Jira

O CLI gera o relatório, mas as descrições na seção "RESUMO DAS ATIVIDADES" frequentemente ficam truncadas ou genéricas (ex: "Descrição detalhada"). Este passo corrige isso buscando as descrições completas diretamente da API do Jira.

1. Leia o arquivo HTML gerado e extraia todos os IDs de issues (ex: `PSD-2788`, `PPRDJ-456`)
2. Leia o `.env` para obter `EMAIL`, `API_KEY` e `URL`
3. Faça uma chamada à API do Jira para buscar as descrições completas:

```bash
source .env && curl -s -u "$EMAIL:$API_KEY" \
  -H "Content-Type: application/json" \
  "$URL/rest/api/3/search/jql" \
  -d '{
    "jql": "key in (ISSUE-1,ISSUE-2,...)",
    "fields": ["key","description"],
    "maxResults": 50
  }'
```

**Importante**: A API antiga `/rest/api/3/search` foi descontinuada. Use `/rest/api/3/search/jql`.

4. Para cada issue, extraia o texto da descrição do campo `description` (formato Atlassian Document Format - ADF):
   - Percorra os nodes recursivamente extraindo o `text` de nodes do tipo `text`
   - Pule cabeçalhos genéricos como "Descrição detalhada", "Necessidade", "Evidência"
   - Pare antes de seções como "Passo a Passo", "Situação encontrada", "Evidências"
   - Limite cada descrição a ~500 caracteres
   - Faça escape de HTML nos textos extraídos

5. Substitua a seção "RESUMO DAS ATIVIDADES" no HTML, trocando cada `<p><b>KEY:</b> texto antigo</p>` pela descrição completa extraída da API

6. Salve o arquivo HTML atualizado

### 6. Verificar resultado

Após a execução e enriquecimento:
1. Verifique se o comando retornou sem erro
2. Confirme que o arquivo foi criado em `reports/`
3. Leia o arquivo HTML gerado para mostrar ao usuário um resumo:
   - Quantidade de atividades encontradas
   - Período do relatório (mês/ano)
   - Lista resumida das tarefas (key + summary)
4. Informe o caminho completo do arquivo gerado

### 7. Se houver erro

Erros comuns:
- **"missing required configuration"**: O arquivo `.env` está incompleto. Leia o `.env` e informe qual campo está faltando.
- **Erro de autenticação/401**: A API_KEY pode estar expirada. Peça ao usuário para gerar um novo token em https://id.atlassian.com/manage-profile/security/api-tokens
- **Nenhuma atividade encontrada**: O JQL pode não ter retornado resultados para o período. Sugira verificar se há cards atribuídos ao usuário no Jira para aquele mês, ou tentar com a flag `-q` para incluir tarefas de QA.
- **LibreOffice não encontrado** (só para DOCX): O formato DOCX requer LibreOffice instalado. Sugira usar HTML ou instalar o LibreOffice.
