## Советы по высоконагруженному логированию

Если вы храните много логов в PostgreSQL / TimescaleDB, одиночные `INSERT` быстро становятся узким местом. Для сценариев с большим количеством записей лучше использовать пакетную вставку через `COPY FROM`.

### Почему `COPY FROM` быстрее

- Отправляет данные на сервер в компактном потоке (текст/бинарный).
- Уменьшает количество сетевых round-trip'ов.
- Позволяет PostgreSQL вставлять множество строк за одну операцию, что хорошо работает с hypertable и партициями TimescaleDB.

Это особенно удобно для таблиц вроде `user_audit_log`, куда постоянно дописываются новые записи.

### Пример: пакетная вставка в `user_audit_log`

Ниже функция, использующая `pgx.CopyFrom` для пакетной вставки строк в таблицу `user_audit_log`.

```go
// BatchUserLog вставляет пачку записей аудита пользователей
// в таблицу user_audit_log с помощью COPY FROM.
//
// Порядок колонок в каждой строке:
//   action, user_email, details, created_at
func BatchUserLog(data [][]any) {
	if len(data) == 0 {
		return
	}

	const timeout = 5 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	rows := pgx.CopyFromRows(data)

	count, err := masterDB.CopyFrom(
		ctx,
		pgx.Identifier{"user_audit_log"},
		[]string{"action", "user_email", "details", "created_at"},
		rows,
	)
	if err != nil {
		log.Println("BatchUserLog: failed to copy from rows:", err)
		return
	}

	if count != int64(len(data)) {
		log.Printf("BatchUserLog: copyFrom count mismatch: expected=%d got=%d\n", len(data), count)
	}
}
```

### Замечания по использованию

- `masterDB` должен быть активным соединением (`*pgx.Conn` или `*pgxpool.Pool`) с вашей базой PostgreSQL / TimescaleDB.
- Старайтесь группировать логи в разумные батчи (например, 500–5 000 строк) перед вызовом `BatchUserLog`.
- Следите, чтобы `created_at` совпадал с временной колонкой hypertable — так TimescaleDB сможет эффективно направлять строки в нужные партиции.

Такой подход значительно снижает нагрузку на запись и особенно хорошо работает вместе с временным партиционированием / hypertable.
