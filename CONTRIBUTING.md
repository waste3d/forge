# Как внести вклад в проект

## Первые шаги

1. **Форкни** репозиторий.  
2. Собери проект локально:  
   ```sh
   git clone https://github.com/yourname/myapp
   cd myapp
   go build ./cmd/myapp
   ```

## Запуск тестов  
```sh
go test ./...
golangci-lint run  # проверка стиля
```

## Отправка изменений  

1. Создай новую ветку:  
   ```sh
   git checkout -b feature/my-cool-feature
   ```  
2. Сделай commit (`git commit -m "описание"`).  
3. Запуши в свой форк (`git push origin feature/...`).  
4. Открой **Pull Request** в основной репозиторий.  

## Правила  

- Код должен проходить `golangci-lint`.  
- Новый функционал — только с тестами.  
- Документируй публичные методы (используй `godoc`).  

## Вопросы?  

Пиши в Issues или в Telegram: @waste3d.  
```

---

### **Дополнительные советы**  
1. **Ссылайся на эти файлы в README.md**:  
   ```markdown
   ## Участие  
   Перед тем как внести вклад, прочти [CONTRIBUTING.md](CONTRIBUTING.md) и [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).  
   ```  

2. **Добавь шаблоны для Issues/Pull Requests**  
   В папке `.github/` создай:  
   - `ISSUE_TEMPLATE/bug_report.md`  
   - `PULL_REQUEST_TEMPLATE.md`  

3. **Для CLI-проектов** добавь раздел **"Development"** в `CONTRIBUTING.md`:  
   ```markdown
   ## Разработка CLI  

   Чтобы добавить новую команду:  
   1. Реализуй её в `cmd/root.go` (используй Cobra, если проект на нём).  
   2. Добавь автотесты.  
   3. Обнови `--help` и документацию.  
   ```  

Эти файлы сделают проект прозрачным и удобным для контрибьюторов! 🚀