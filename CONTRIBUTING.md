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
