# aicommit

Консольная утилита на Go для генерации commit message по текущим изменениям в git.

**Быстрый старт**
`go run .`

**Примеры**
- `go run . -staged`
- `go run . -format plain`
- `go run . -body stats -max-items 6`
- `go run . -lang ru`
- `go run . -type feat -scope api`
- `go run . -refs "#123" -closes "#456"`
- `go run . -emoji`
- `OPENAI_API_KEY=... go run . -llm -provider openai -model <model>`
- `OPENROUTER_API_KEY=... go run . -llm -provider openrouter -model <model>`

**Возможности**
- Автовыбор staged или unstaged изменений
- Поддержка Conventional Commits и gitmoji-кодов
- Автоопределение типа и scope
- Поиск breaking изменений по diff
- Генерация тела коммита: список файлов, статистика или краткое резюме
- Настройка длины subject и количества строк в теле
- Ссылки на задачи через `Refs:` и `Closes:`
- Копирование результата в буфер (`-copy`)
- `-explain` для вывода причин выбора в stderr
- Генерация с помощью LLM (OpenAI или OpenRouter)

**LLM**
- Включение: `-llm`
- Провайдер: `-provider openai|openrouter`
- Модель: `-model <model>` (по умолчанию `gpt-5-nano`)
- Ключи: `OPENAI_API_KEY` или `OPENROUTER_API_KEY` (или `COMMITGEN_LLM_KEY`)
- При ошибке LLM утилита падает обратно на эвристику; используйте `-llm-strict`, чтобы получить ошибку.

**Переменные окружения**
- `COMMITGEN_FORMAT`
- `COMMITGEN_LANG`
- `COMMITGEN_BODY`
- `COMMITGEN_MAX_ITEMS`
- `COMMITGEN_MAX_SUBJECT`
- `COMMITGEN_TYPE`
- `COMMITGEN_SCOPE`
- `COMMITGEN_REFS`
- `COMMITGEN_CLOSES`
- `COMMITGEN_LLM`
- `COMMITGEN_LLM_PROVIDER`
- `COMMITGEN_LLM_MODEL`
- `COMMITGEN_LLM_ENDPOINT`
- `COMMITGEN_LLM_KEY`
- `COMMITGEN_LLM_TEMPERATURE`
- `COMMITGEN_LLM_MAX_TOKENS`
- `COMMITGEN_LLM_MAX_DIFF`
- `COMMITGEN_LLM_STRICT`
- `COMMITGEN_LLM_SYSTEM`
- `COMMITGEN_LLM_USER`
- `COMMITGEN_OPENROUTER_REFERER`
- `COMMITGEN_OPENROUTER_TITLE`
- `OPENAI_API_KEY`
- `OPENROUTER_API_KEY`
